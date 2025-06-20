package handler

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"google.golang.org/grpc"
	"net/http"
)

func HandleGRPCServerStreamSSE(c *gin.Context) {
	target := c.Query("target")
	service := c.Query("service")
	method := c.Query("method")
	message := c.Query("message")

	// Find method descriptor
	var methodDesc *desc.MethodDescriptor
	found := false
	for _, file := range descriptorSet.File {
		for _, svc := range file.GetService() {
			fullService := fmt.Sprintf("%s.%s", file.GetPackage(), svc.GetName())
			if fullService == service || svc.GetName() == service {
				for _, m := range svc.GetMethod() {
					if m.GetName() == method {
						fileDesc, _ := desc.CreateFileDescriptor(file)
						sd := fileDesc.FindService(fullService)
						methodDesc = sd.FindMethodByName(method)
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
	if !found || methodDesc == nil || !methodDesc.IsServerStreaming() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid streaming method"})
		return
	}

	// Build request
	reqMsg := dynamic.NewMessage(methodDesc.GetInputType())
	reqMsg.SetFieldByName("message", message)

	// Connect to gRPC server
	conn, err := grpc.Dial(target, grpc.WithInsecure())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Connection failed"})
		return
	}
	defer conn.Close()

	stub := grpcdynamic.NewStub(conn)
	stream, err := stub.InvokeRpcServerStream(context.Background(), methodDesc, reqMsg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Stream failed", "details": err.Error()})
		return
	}

	// Set headers for SSE
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	// Flush headers
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Streaming unsupported"})
		return
	}

	// Stream messages as they arrive
	for {
		msg, err := stream.RecvMsg()
		if err != nil {
			break // done or EOF
		}
		dynMsg, ok := msg.(*dynamic.Message)
		if !ok {
			continue
		}
		jsonData, err := dynMsg.MarshalJSON()
		if err == nil {
			fmt.Fprintf(c.Writer, "data: %s\n\n", jsonData)
			flusher.Flush()
		}
	}
}
