package handler

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

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
)

// Constants and configuration
const (
	DefaultProtocVersion = "24.4"
	CleanupDelay         = 10 * time.Minute
	DefaultDialTimeout   = 30 * time.Second
	DefaultPort443       = ":443"
	DefaultPort80        = ":80"
)

// Log messages as variables
var (
	MsgCleanedUp             = "Cleaned up: %s"
	MsgProtoUploaded         = "Proto uploaded and compiled successfully"
	MsgNoFileUploaded        = "No file uploaded"
	MsgCreateUploadDirFailed = "Could not create upload dir"
	MsgSaveFileFailed        = "Could not save file"
	MsgProtocInstallFailed   = "protoc not installed and failed to download: %s"
	MsgProtoCompileFailed    = "Failed to compile .proto with protoc"
	MsgReadDescriptorFailed  = "Could not read descriptor set"
	MsgParseDescriptorFailed = "Failed to parse descriptor set"
	MsgNoDescriptorLoaded    = "No descriptor loaded"
	MsgInitPayloadFailed     = "Failed to read init payload"
	MsgInvalidInitJSON       = "Invalid init JSON"
	MsgMethodNotFound        = "Method not found"
	MsgDialTargetFailed      = "Failed to dial target"
	MsgUnknownMode           = "Unknown or unsupported mode"
	MsgNoInputMessage        = "No input message"
	MsgInvalidInput          = "Invalid input"
	MsgStreamFailed          = "Stream failed"
	MsgInvalidJSON           = "Invalid JSON"
	MsgUnmarshalFailed       = "UnmarshalJSON failed"
	MsgSendMsgFailed         = "SendMsg failed"
	MsgCloseStreamFailed     = "Closing stream failed"
	MsgBidiStreamFailed      = "Bidi stream failed"
	MsgRPCCallFailed         = "RPC call failed"
	MsgUnexpectedResponse    = "Unexpected response type"
	MsgUnsupportedOS         = "unsupported OS: %s"
)

// Global variables with better organization
var (
	descriptorSetPath = "./compiled.protoset"
	tempProtoDir      = "./uploaded_protos"
	importPaths       []string
	descriptorSet     descriptorpb.FileDescriptorSet
	descriptorSetMu   sync.RWMutex // Protect concurrent access
)

// WebSocket upgrader configuration
var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Data structures
type InitMessage struct {
	Target   string            `json:"target"`
	Service  string            `json:"service"`
	Method   string            `json:"method"`
	Mode     string            `json:"mode"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Auth     *AuthConfig       `json:"auth,omitempty"`
}

type AuthConfig struct {
	Type     string `json:"type"`
	Token    string `json:"token,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type StreamMode string

const (
	ModeUnary     StreamMode = "unary"
	ModeServer    StreamMode = "server"
	ModeClient    StreamMode = "client"
	ModeBidi      StreamMode = "bidi"
	EndSignal                = "__END__"
	EndJSONSignal            = `{"end": true}`
)

// Error definitions
var (
	ErrNoFileUploaded     = errors.New("no file uploaded")
	ErrCreateUploadDir    = errors.New("could not create upload dir")
	ErrSaveFile           = errors.New("could not save file")
	ErrReadDescriptor     = errors.New("could not read descriptor set")
	ErrParseDescriptor    = errors.New("failed to parse descriptor set")
	ErrNoDescriptorLoaded = errors.New("no descriptor loaded")
	ErrMethodNotFound     = errors.New("method not found")
	ErrInvalidInitJSON    = errors.New("invalid init JSON")
	ErrDialTarget         = errors.New("failed to dial target")
)

// Proto upload handler
func HandleProtoUpload(c *gin.Context) {
	file, err := c.FormFile("proto")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": MsgNoFileUploaded})
		return
	}

	userDir, err := createUserDirectory()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": MsgCreateUploadDirFailed})
		return
	}

	savedPath, err := saveUploadedFile(c, file, userDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": MsgSaveFileFailed})
		return
	}

	if err := compileProtoFile(savedPath, userDir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := loadDescriptorSet(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	scheduleCleanup(userDir)
	c.JSON(http.StatusOK, gin.H{"message": MsgProtoUploaded})
}

// List services handler
func HandleListServices(c *gin.Context) {
	descriptorSetMu.RLock()
	defer descriptorSetMu.RUnlock()

	if len(descriptorSet.File) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": MsgNoDescriptorLoaded})
		return
	}

	result := listServicesAndMethods()
	c.JSON(http.StatusOK, result)
}

// WebSocket gRPC stream handler
func HandleGRPCWebSocketStream(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	init, err := readInitMessage(conn)
	if err != nil {
		conn.WriteJSON(gin.H{"error": err.Error()})
		return
	}

	ctx := buildContext(init)
	methodDesc, err := findMethodDescriptor(init)
	if err != nil {
		conn.WriteJSON(gin.H{"error": err.Error()})
		return
	}

	clientConn, err := dialTarget(init.Target)
	if err != nil {
		conn.WriteJSON(gin.H{"error": MsgDialTargetFailed, "details": err.Error()})
		return
	}
	defer clientConn.Close()

	stub := grpcdynamic.NewStub(clientConn)
	mode := determineStreamMode(init.Mode, methodDesc)

	if err := handleStreamMode(ctx, stub, conn, methodDesc, mode); err != nil {
		conn.WriteJSON(gin.H{"error": err.Error()})
	}
}

// Helper functions
func createUserDirectory() (string, error) {
	userDir := filepath.Join(tempProtoDir, fmt.Sprintf("user-%d", time.Now().UnixNano()))
	return userDir, os.MkdirAll(userDir, os.ModePerm)
}

func saveUploadedFile(c *gin.Context, file *multipart.FileHeader, userDir string) (string, error) {
	savedPath := filepath.Join(userDir, file.Filename)
	return savedPath, c.SaveUploadedFile(file, savedPath)
}

func compileProtoFile(savedPath, userDir string) error {
	protocPath, err := installProtocIfMissing()
	if err != nil {
		return fmt.Errorf(MsgProtocInstallFailed, err.Error())
	}

	args := buildProtocArgs(userDir, savedPath)
	cmd := exec.Command(protocPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return errors.New(MsgProtoCompileFailed)
	}
	return nil
}

func buildProtocArgs(userDir, savedPath string) []string {
	args := make([]string, 0, len(importPaths)+5)

	for _, path := range importPaths {
		args = append(args, "--proto_path="+path)
	}

	args = append(args,
		"--proto_path="+userDir,
		"--descriptor_set_out="+descriptorSetPath,
		"--include_imports",
		savedPath,
	)

	return args
}

func loadDescriptorSet() error {
	descriptorSetMu.Lock()
	defer descriptorSetMu.Unlock()

	data, err := os.ReadFile(descriptorSetPath)
	if err != nil {
		return errors.New(MsgReadDescriptorFailed)
	}

	if err := proto.Unmarshal(data, &descriptorSet); err != nil {
		return errors.New(MsgParseDescriptorFailed)
	}

	return nil
}

func scheduleCleanup(userDir string) {
	go func(path string) {
		time.AfterFunc(CleanupDelay, func() {
			if err := os.RemoveAll(path); err == nil {
				fmt.Printf(MsgCleanedUp+"\n", path)
			}
		})
	}(userDir)
}

func listServicesAndMethods() map[string][]string {
	services := make(map[string][]string)

	for _, file := range descriptorSet.File {
		for _, service := range file.GetService() {
			serviceName := service.GetName()
			methods := make([]string, 0, len(service.GetMethod()))

			for _, method := range service.GetMethod() {
				methods = append(methods, method.GetName())
			}

			services[serviceName] = methods
		}
	}

	return services
}

func readInitMessage(conn *websocket.Conn) (*InitMessage, error) {
	_, initPayload, err := conn.ReadMessage()
	if err != nil {
		return nil, errors.New(MsgInitPayloadFailed)
	}

	var init InitMessage
	if err := json.Unmarshal(initPayload, &init); err != nil {
		return nil, errors.New(MsgInvalidInitJSON)
	}

	return &init, nil
}

func buildContext(init *InitMessage) context.Context {
	md := metadata.New(nil)

	// Add user-supplied metadata
	for k, v := range init.Metadata {
		md.Append(k, v)
	}

	// Add authentication metadata
	if init.Auth != nil {
		addAuthMetadata(md, init.Auth)
	}

	return metadata.NewOutgoingContext(context.Background(), md)
}

func addAuthMetadata(md metadata.MD, auth *AuthConfig) {
	switch strings.ToLower(auth.Type) {
	case "bearer":
		md.Append("authorization", "Bearer "+auth.Token)
	case "basic":
		creds := base64.StdEncoding.EncodeToString(
			[]byte(auth.Username + ":" + auth.Password))
		md.Append("authorization", "Basic "+creds)
	}
}

func findMethodDescriptor(init *InitMessage) (*desc.MethodDescriptor, error) {
	descriptorSetMu.RLock()
	defer descriptorSetMu.RUnlock()

	for _, file := range descriptorSet.File {
		for _, svc := range file.GetService() {
			fullService := fmt.Sprintf("%s.%s", file.GetPackage(), svc.GetName())

			if fullService == init.Service || svc.GetName() == init.Service {
				if methodDesc := findMethodInService(file, fullService, init.Method); methodDesc != nil {
					return methodDesc, nil
				}
			}
		}
	}

	return nil, errors.New(MsgMethodNotFound)
}

func findMethodInService(file *descriptorpb.FileDescriptorProto, fullService, methodName string) *desc.MethodDescriptor {
	fileDesc, err := desc.CreateFileDescriptor(file)
	if err != nil {
		return nil
	}

	sd := fileDesc.FindService(fullService)
	if sd == nil {
		return nil
	}

	return sd.FindMethodByName(methodName)
}

func determineStreamMode(requestedMode string, methodDesc *desc.MethodDescriptor) StreamMode {
	if requestedMode != "" {
		return StreamMode(requestedMode)
	}
	return inferMode(methodDesc)
}

func inferMode(method *desc.MethodDescriptor) StreamMode {
	switch {
	case method.IsClientStreaming() && method.IsServerStreaming():
		return ModeBidi
	case method.IsClientStreaming():
		return ModeClient
	case method.IsServerStreaming():
		return ModeServer
	default:
		return ModeUnary
	}
}

func handleStreamMode(ctx context.Context, stub grpcdynamic.Stub, conn *websocket.Conn,
	methodDesc *desc.MethodDescriptor, mode StreamMode) error {

	switch mode {
	case ModeUnary:
		return handleUnary(ctx, stub, conn, methodDesc)
	case ModeServer:
		return handleServerStream(ctx, stub, conn, methodDesc)
	case ModeClient:
		return handleClientStream(ctx, stub, conn, methodDesc)
	case ModeBidi:
		return handleBidiStream(ctx, stub, conn, methodDesc)
	default:
		return errors.New(MsgUnknownMode)
	}
}

func dialTarget(rawTarget string) (*grpc.ClientConn, error) {
	target, opts := parseTargetAndCredentials(rawTarget)
	return grpc.Dial(target, opts)
}

func parseTargetAndCredentials(rawTarget string) (string, grpc.DialOption) {
	// Handle full URLs
	if strings.HasPrefix(rawTarget, "http://") || strings.HasPrefix(rawTarget, "https://") {
		return parseURLTarget(rawTarget)
	}

	// Handle host:port format
	return parseHostPortTarget(rawTarget)
}

func parseURLTarget(rawTarget string) (string, grpc.DialOption) {
	u, err := url.Parse(rawTarget)
	if err != nil {
		return rawTarget, grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	host := u.Host
	if !strings.Contains(host, ":") {
		if u.Scheme == "https" {
			host += DefaultPort443
		} else {
			host += DefaultPort80
		}
	}

	if u.Scheme == "https" {
		return host, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, ""))
	}

	return host, grpc.WithTransportCredentials(insecure.NewCredentials())
}

func parseHostPortTarget(rawTarget string) (string, grpc.DialOption) {
	if strings.HasSuffix(rawTarget, DefaultPort443) {
		return rawTarget, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, ""))
	}

	return rawTarget, grpc.WithTransportCredentials(insecure.NewCredentials())
}

// Stream handlers
func handleUnary(ctx context.Context, stub grpcdynamic.Stub,
	conn *websocket.Conn, method *desc.MethodDescriptor) error {

	_, msgRaw, err := conn.ReadMessage()
	if err != nil {
		return errors.New(MsgNoInputMessage)
	}

	reqMsg := dynamic.NewMessage(method.GetInputType())
	if err := reqMsg.UnmarshalJSON(msgRaw); err != nil {
		return fmt.Errorf("%s: %v", MsgInvalidInput, err)
	}

	resp, err := stub.InvokeRpc(ctx, method, reqMsg)
	if err != nil {
		return fmt.Errorf("%s: %v", MsgRPCCallFailed, err)
	}

	dynResp, ok := resp.(*dynamic.Message)
	if !ok {
		return errors.New(MsgUnexpectedResponse)
	}

	data, _ := dynResp.MarshalJSON()
	return conn.WriteMessage(websocket.TextMessage, data)
}

func handleServerStream(ctx context.Context, stub grpcdynamic.Stub,
	conn *websocket.Conn, method *desc.MethodDescriptor) error {

	_, msgRaw, err := conn.ReadMessage()
	if err != nil {
		return errors.New(MsgNoInputMessage)
	}

	reqMsg := dynamic.NewMessage(method.GetInputType())
	if err := reqMsg.UnmarshalJSON(msgRaw); err != nil {
		return fmt.Errorf("%s: %v", MsgInvalidInput, err)
	}

	stream, err := stub.InvokeRpcServerStream(ctx, method, reqMsg)
	if err != nil {
		return fmt.Errorf("%s: %v", MsgStreamFailed, err)
	}

	for {
		msg, err := stream.RecvMsg()
		if err != nil {
			break
		}

		if dynMsg, ok := msg.(*dynamic.Message); ok {
			if data, err := dynMsg.MarshalJSON(); err == nil {
				conn.WriteMessage(websocket.TextMessage, data)
			}
		}
	}

	return nil
}

func handleClientStream(ctx context.Context, stub grpcdynamic.Stub,
	conn *websocket.Conn, method *desc.MethodDescriptor) error {

	stream, err := stub.InvokeRpcClientStream(ctx, method)
	if err != nil {
		return fmt.Errorf("%s: %v", MsgStreamFailed, err)
	}

	for {
		_, msgRaw, err := conn.ReadMessage()
		if err != nil {
			break
		}

		if shouldEndClientStream(msgRaw) {
			break
		}

		reqMsg := dynamic.NewMessage(method.GetInputType())
		if err := reqMsg.UnmarshalJSON(msgRaw); err != nil {
			continue // Skip invalid messages
		}

		if err := stream.SendMsg(reqMsg); err != nil {
			return fmt.Errorf("%s: %v", MsgSendMsgFailed, err)
		}
	}

	resp, err := stream.CloseAndReceive()
	if err != nil {
		return fmt.Errorf("%s: %v", MsgCloseStreamFailed, err)
	}

	if dynResp, ok := resp.(*dynamic.Message); ok {
		data, _ := dynResp.MarshalJSON()
		conn.WriteMessage(websocket.TextMessage, data)
	}

	return nil
}

func shouldEndClientStream(msgRaw []byte) bool {
	var generic map[string]interface{}
	if err := json.Unmarshal(msgRaw, &generic); err != nil {
		return false
	}

	if val, ok := generic["end"].(bool); ok && val {
		return true
	}

	return false
}

func handleBidiStream(ctx context.Context, stub grpcdynamic.Stub,
	conn *websocket.Conn, method *desc.MethodDescriptor) error {

	stream, err := stub.InvokeRpcBidiStream(ctx, method)
	if err != nil {
		return fmt.Errorf("%s: %v", MsgBidiStreamFailed, err)
	}

	done := make(chan struct{})
	errChan := make(chan error, 2)

	// Reader goroutine
	go func() {
		defer stream.CloseSend()

		for {
			_, msgRaw, err := conn.ReadMessage()
			if err != nil || string(msgRaw) == EndSignal {
				break
			}

			reqMsg := dynamic.NewMessage(method.GetInputType())
			if err := reqMsg.UnmarshalJSON(msgRaw); err == nil {
				if err := stream.SendMsg(reqMsg); err != nil {
					errChan <- err
					break
				}
			}
		}
	}()

	// Writer goroutine
	go func() {
		defer close(done)

		for {
			msg, err := stream.RecvMsg()
			if err != nil {
				break
			}

			if dynMsg, ok := msg.(*dynamic.Message); ok {
				if data, err := dynMsg.MarshalJSON(); err == nil {
					if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
						errChan <- err
						break
					}
				}
			}
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-done:
		return nil
	}
}

// Protocol buffer compiler installation
func installProtocIfMissing() (string, error) {
	if protocPath, err := exec.LookPath("protoc"); err == nil {
		return protocPath, nil
	}

	osName, err := getOSName()
	if err != nil {
		return "", err
	}

	return downloadAndInstallProtoc(osName)
}

func getOSName() (string, error) {
	switch runtime.GOOS {
	case "windows":
		return "win64", nil
	case "darwin":
		return "osx-x86_64", nil
	case "linux":
		return "linux-x86_64", nil
	default:
		return "", fmt.Errorf(MsgUnsupportedOS, runtime.GOOS)
	}
}

func downloadAndInstallProtoc(osName string) (string, error) {
	url := buildProtocDownloadURL(osName)

	tmpDir := filepath.Join(os.TempDir(), ".protoc")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}

	zipPath, err := downloadProtoc(url, tmpDir, osName)
	if err != nil {
		return "", err
	}

	return extractProtoc(zipPath, tmpDir)
}

func buildProtocDownloadURL(osName string) string {
	filename := fmt.Sprintf("protoc-%s-%s.zip", DefaultProtocVersion, osName)
	return fmt.Sprintf("https://github.com/protocolbuffers/protobuf/releases/download/v%s/%s",
		DefaultProtocVersion, filename)
}

func downloadProtoc(url, tmpDir, osName string) (string, error) {
	filename := fmt.Sprintf("protoc-%s-%s.zip", DefaultProtocVersion, osName)
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

	return zipPath, nil
}

func extractProtoc(zipPath, tmpDir string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	installDir := filepath.Join(tmpDir, "protoc")
	os.RemoveAll(installDir) // Clean before extraction

	for _, f := range r.File {
		if err := extractFile(f, installDir); err != nil {
			return "", err
		}
	}

	protocExecutable := filepath.Join(installDir, "bin", "protoc")
	if runtime.GOOS == "windows" {
		protocExecutable += ".exe"
	}

	return protocExecutable, nil
}

func extractFile(f *zip.File, installDir string) error {
	fpath := filepath.Join(installDir, f.Name)

	if f.FileInfo().IsDir() {
		return os.MkdirAll(fpath, 0755)
	}

	if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
		return err
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	dest, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer dest.Close()

	_, err = io.Copy(dest, rc)
	return err
}
