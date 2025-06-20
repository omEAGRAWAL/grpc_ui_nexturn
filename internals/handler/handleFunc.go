package handler

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var (
	descriptorSetPath = "./compiled.protoset"
	tempProtoDir      = "./uploaded_protos"
)
var importPaths []string
var descriptorSet descriptorpb.FileDescriptorSet

func HandleProtoUpload(c *gin.Context) {
	file, err := c.FormFile("proto")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	// Create a unique subfolder for this upload
	userDir := filepath.Join(tempProtoDir, fmt.Sprintf("user-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(userDir, os.ModePerm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create upload dir"})
		return
	}

	// Save the uploaded proto file
	savedPath := filepath.Join(userDir, file.Filename)
	if err := c.SaveUploadedFile(file, savedPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not save file"})
		return
	}

	// Install or find protoc
	protocPath, err := installProtocIfMissing()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "protoc not installed and failed to download: " + err.Error()})
		return
	}

	// Compile .proto to descriptor
	args := []string{}
	for _, path := range importPaths {
		args = append(args, "--proto_path="+path)
	}
	args = append(args,
		"--proto_path="+userDir,
		"--descriptor_set_out="+descriptorSetPath,
		"--include_imports",
		savedPath,
	)

	cmd := exec.Command(protocPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to compile .proto with protoc"})
		return
	}

	// Load descriptor set
	data, err := os.ReadFile(descriptorSetPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read descriptor set"})
		return
	}
	if err := proto.Unmarshal(data, &descriptorSet); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse descriptor set"})
		return
	}

	// ✅ Schedule folder deletion after 10 minutes
	go func(path string) {
		time.AfterFunc(10*time.Minute, func() {
			os.RemoveAll(path)
			fmt.Println("Cleaned up:", path)
		})
	}(userDir)

	c.JSON(http.StatusOK, gin.H{"message": "Proto uploaded and compiled successfully"})
}

func HandleListServices(c *gin.Context) {
	if len(descriptorSet.File) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No descriptor loaded"})
		return
	}

	result := ListServicesAndMethods()
	c.JSON(http.StatusOK, result)
}

func ListServicesAndMethods() map[string][]string {
	services := make(map[string][]string)

	for _, file := range descriptorSet.File {
		for _, service := range file.GetService() {
			serviceName := service.GetName()
			var methods []string
			for _, method := range service.GetMethod() {
				methods = append(methods, method.GetName())
			}
			services[serviceName] = methods
		}
	}

	return services
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ------------------------------------------------------------------
// 1) Extend the InitMessage
// ------------------------------------------------------------------
type InitMessage struct {
	Target   string            `json:"target"`
	Service  string            `json:"service"`
	Method   string            `json:"method"`
	Mode     string            `json:"mode"`               // "unary", "server", …
	Metadata map[string]string `json:"metadata,omitempty"` // extra gRPC‐MD
	Auth     *struct {
		Type     string `json:"type"`               // "basic" | "bearer"
		Token    string `json:"token,omitempty"`    // bearer / API‑key
		Username string `json:"username,omitempty"` // basic
		Password string `json:"password,omitempty"` // basic
	} `json:"auth,omitempty"`
}

// ------------------------------------------------------------------
// 2) Build an outgoing context that carries both custom MD and auth
// ------------------------------------------------------------------
func buildContext(init *InitMessage) context.Context {
	md := metadata.New(nil)

	// user‑supplied key/value pairs
	for k, v := range init.Metadata {
		md.Append(k, v)
	}

	// simple auth helpers (extend as needed)
	if init.Auth != nil {
		switch strings.ToLower(init.Auth.Type) {
		case "bearer":
			md.Append("authorization", "Bearer "+init.Auth.Token)
		case "basic":
			creds := base64.StdEncoding.EncodeToString(
				[]byte(init.Auth.Username + ":" + init.Auth.Password))
			md.Append("authorization", "Basic "+creds)
		}
	}

	return metadata.NewOutgoingContext(context.Background(), md)
}
func HandleGRPCWebSocketStream(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	_, initPayload, err := conn.ReadMessage()
	if err != nil {
		conn.WriteJSON(gin.H{"error": "Failed to read init payload"})
		return
	}

	var init InitMessage
	if err := json.Unmarshal(initPayload, &init); err != nil {
		conn.WriteJSON(gin.H{"error": "Invalid init JSON"})
		return
	}
	ctx := buildContext(&init)

	// Lookup method
	var methodDesc *desc.MethodDescriptor
	found := false
	for _, file := range descriptorSet.File {
		for _, svc := range file.GetService() {
			fullService := fmt.Sprintf("%s.%s", file.GetPackage(), svc.GetName())
			if fullService == init.Service || svc.GetName() == init.Service {
				for _, m := range svc.GetMethod() {
					if m.GetName() == init.Method {
						fileDesc, _ := desc.CreateFileDescriptor(file)
						sd := fileDesc.FindService(fullService)
						methodDesc = sd.FindMethodByName(init.Method)
						found = true
						break
					}
				}
			}
			if found {
				break
			}
		}
	}
	if !found || methodDesc == nil {
		conn.WriteJSON(gin.H{"error": "Method not found"})
		return
	}

	// Connect to gRPC
	clientConn, err := dialTarget(init.Target)
	if err != nil {
		conn.WriteJSON(gin.H{"error": "Failed to dial target", "details": err.Error()})
		return
	}
	defer clientConn.Close()

	stub := grpcdynamic.NewStub(clientConn)

	// ✨ Remove the next line – it nulled out your metadata!
	//   ctx = context.Background()

	mode := init.Mode
	if mode == "" {
		mode = inferMode(methodDesc)
	}

	switch mode {
	case "unary":
		handleUnary(ctx, stub, conn, methodDesc)
	case "server":
		handleServerStream(ctx, stub, conn, methodDesc)
	case "client":
		handleClientStream(ctx, stub, conn, methodDesc)
	case "bidi":
		handleBidiStream(ctx, stub, conn, methodDesc)
	default:
		conn.WriteJSON(gin.H{"error": "Unknown or unsupported mode"})
	}

}

func inferMode(method *desc.MethodDescriptor) string {
	switch {
	case method.IsClientStreaming() && method.IsServerStreaming():
		return "bidi"
	case method.IsClientStreaming():
		return "client"
	case method.IsServerStreaming():
		return "server"
	default:
		return "unary"
	}
}

func dialTarget(rawTarget string) (*grpc.ClientConn, error) {
	var target string
	var opts grpc.DialOption

	// Try to parse as full URL (e.g. "https://xxx.ngrok-free.app:443")
	if strings.HasPrefix(rawTarget, "http://") || strings.HasPrefix(rawTarget, "https://") {
		u, err := url.Parse(rawTarget)
		if err != nil {
			return nil, err
		}
		host := u.Host
		if !strings.Contains(host, ":") {
			if u.Scheme == "https" {
				host += ":443"
			} else {
				host += ":80"
			}
		}
		target = host
		if u.Scheme == "https" {
			opts = grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, ""))
		} else {
			opts = grpc.WithTransportCredentials(insecure.NewCredentials())
		}
	} else {
		// Assume it's a host:port like "localhost:50051" or "0.tcp.ngrok.io:21934"
		target = rawTarget
		if strings.HasSuffix(target, ":443") {
			opts = grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, ""))
		} else {
			opts = grpc.WithTransportCredentials(insecure.NewCredentials())
		}
	}

	return grpc.Dial(target, opts)
}

func handleServerStream(ctx context.Context, stub grpcdynamic.Stub, conn *websocket.Conn, method *desc.MethodDescriptor) {
	_, msgRaw, err := conn.ReadMessage()
	if err != nil {
		conn.WriteJSON(gin.H{"error": "No input message"})
		return
	}
	reqMsg := dynamic.NewMessage(method.GetInputType())
	if err := reqMsg.UnmarshalJSON(msgRaw); err != nil {
		conn.WriteJSON(gin.H{"error": "Invalid input", "details": err.Error()})
		return
	}
	stream, err := stub.InvokeRpcServerStream(ctx, method, reqMsg)
	if err != nil {
		conn.WriteJSON(gin.H{"error": "Stream failed", "details": err.Error()})
		return
	}

	for {
		msg, err := stream.RecvMsg()
		if err != nil {
			break
		}
		dynMsg, ok := msg.(*dynamic.Message)
		if ok {
			data, _ := dynMsg.MarshalJSON()
			conn.WriteMessage(websocket.TextMessage, data)
		}
	}
}

func handleClientStream(ctx context.Context, stub grpcdynamic.Stub, conn *websocket.Conn, method *desc.MethodDescriptor) {
	stream, err := stub.InvokeRpcClientStream(ctx, method)
	if err != nil {
		conn.WriteJSON(gin.H{"error": "Stream failed", "details": err.Error()})
		return
	}

	for {
		_, msgRaw, err := conn.ReadMessage()
		if err != nil {
			break
		}

		// Parse raw JSON to check for {"end": true}
		var generic map[string]interface{}
		if err := json.Unmarshal(msgRaw, &generic); err != nil {
			conn.WriteJSON(gin.H{"error": "Invalid JSON", "details": err.Error()})
			continue
		}

		if val, ok := generic["end"].(bool); ok && val {
			break
		}

		// Unmarshal into dynamic gRPC request message
		reqMsg := dynamic.NewMessage(method.GetInputType())
		if err := reqMsg.UnmarshalJSON(msgRaw); err != nil {
			conn.WriteJSON(gin.H{"error": "UnmarshalJSON failed", "details": err.Error()})
			continue
		}

		if err := stream.SendMsg(reqMsg); err != nil {
			conn.WriteJSON(gin.H{"error": "SendMsg failed", "details": err.Error()})
			break
		}
	}

	resp, err := stream.CloseAndReceive()
	if err != nil {
		conn.WriteJSON(gin.H{"error": "Closing stream failed", "details": err.Error()})
		return
	}
	if dynResp, ok := resp.(*dynamic.Message); ok {
		data, _ := dynResp.MarshalJSON()
		conn.WriteMessage(websocket.TextMessage, data)
	}
}

func handleBidiStream(ctx context.Context, stub grpcdynamic.Stub, conn *websocket.Conn, method *desc.MethodDescriptor) {
	stream, err := stub.InvokeRpcBidiStream(ctx, method)
	if err != nil {
		conn.WriteJSON(gin.H{"error": "Bidi stream failed", "details": err.Error()})
		return
	}

	done := make(chan struct{})

	// Reader (from client to gRPC)
	go func() {
		for {
			_, msgRaw, err := conn.ReadMessage()
			if err != nil || string(msgRaw) == "__END__" {
				stream.CloseSend()
				break
			}
			reqMsg := dynamic.NewMessage(method.GetInputType())
			if err := reqMsg.UnmarshalJSON(msgRaw); err == nil {
				_ = stream.SendMsg(reqMsg)
			}
		}
	}()

	// Writer (from gRPC to client)
	go func() {
		for {
			msg, err := stream.RecvMsg()
			if err != nil {
				break
			}
			dynMsg, ok := msg.(*dynamic.Message)
			if ok {
				data, _ := dynMsg.MarshalJSON()
				conn.WriteMessage(websocket.TextMessage, data)
			}
		}
		close(done)
	}()

	<-done
}
func handleUnary(ctx context.Context, stub grpcdynamic.Stub,
	conn *websocket.Conn, method *desc.MethodDescriptor) {

	// Read one payload frame from the WebSocket → request JSON
	_, msgRaw, err := conn.ReadMessage()
	if err != nil {
		conn.WriteJSON(gin.H{"error": "No input message"})
		return
	}

	// Build dynamic request from JSON
	reqMsg := dynamic.NewMessage(method.GetInputType())
	if err := reqMsg.UnmarshalJSON(msgRaw); err != nil {
		conn.WriteJSON(gin.H{"error": "Invalid input", "details": err.Error()})
		return
	}

	// Invoke the RPC
	resp, err := stub.InvokeRpc(ctx, method, reqMsg)
	if err != nil {
		conn.WriteJSON(gin.H{"error": "RPC call failed", "details": err.Error()})
		return
	}

	// Marshal and send the single response
	if dynResp, ok := resp.(*dynamic.Message); ok {
		data, _ := dynResp.MarshalJSON()
		conn.WriteMessage(websocket.TextMessage, data)
	} else {
		conn.WriteJSON(gin.H{"error": "Unexpected response type"})
	}
}

func installProtocIfMissing() (string, error) {
	protocPath, err := exec.LookPath("protoc")
	if err == nil {
		return protocPath, nil // Already installed
	}

	// Detect OS and arch
	var osName string
	//var arch string
	switch runtime.GOOS {
	case "windows":
		osName = "win64"
	case "darwin":
		osName = "osx-x86_64"
	case "linux":
		osName = "linux-x86_64"
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	//arch := "x86_64" // Only supporting 64-bit for simplicity

	// Construct download URL
	version := "24.4" // Change as needed
	baseURL := "https://github.com/protocolbuffers/protobuf/releases/download"
	filename := fmt.Sprintf("protoc-%s-%s.zip", version, osName)
	url := fmt.Sprintf("%s/v%s/%s", baseURL, version, filename)

	// Download
	tmpDir := filepath.Join(os.TempDir(), ".protoc")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}
	zipPath := filepath.Join(tmpDir, filename)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	out, err := os.Create(zipPath)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", err
	}

	// Unzip
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	installDir := filepath.Join(tmpDir, "protoc")
	os.RemoveAll(installDir) // Clean before unzip
	for _, f := range r.File {
		fpath := filepath.Join(installDir, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(fpath), 0755)

		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()

		dest, err := os.Create(fpath)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(dest, rc); err != nil {
			return "", err
		}
		dest.Close()
	}

	binDir := filepath.Join(installDir, "bin")
	protocExecutable := filepath.Join(binDir, "protoc")
	if runtime.GOOS == "windows" {
		protocExecutable += ".exe"
	}

	// Put protocExecutable in PATH by returning it for use in exec.Command
	return protocExecutable, nil
}
