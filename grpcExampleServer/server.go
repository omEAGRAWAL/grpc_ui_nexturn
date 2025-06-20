// cmd/server/main.go  (or whatever your entry file is)

package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc/peer"
	"io"
	"log"
	"net"
	"os"
	"strings"

	pb "grpcExampleServer/examplepb" // generated package

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// ---------------------------
// 1) Service implementation
// ---------------------------

type server struct {
	pb.UnimplementedExampleServiceServer
}

func (s *server) UnaryCall(ctx context.Context, req *pb.Request) (*pb.Response, error) {
	log.Printf("UnaryCall: %s", req.Message)
	return &pb.Response{Message: "Unary Response: " + req.Message}, nil
}

func (s *server) ServerStreamingCall(req *pb.Request, stream pb.ExampleService_ServerStreamingCallServer) error {
	log.Printf("ServerStreamingCall: %s", req.Message)
	for i := 0; i < 5; i++ {
		resp := &pb.Response{Message: fmt.Sprintf("Stream %d for: %s", i+1, req.Message)}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) ClientStreamingCall(stream pb.ExampleService_ClientStreamingCallServer) error {
	log.Println("ClientStreamingCall started")
	var messages []string
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.Response{Message: "Received: " + fmt.Sprintf("%v", messages)})
		}
		if err != nil {
			return err
		}
		log.Printf("Client sent: %s", req.Message)
		messages = append(messages, req.Message)
	}
}

func (s *server) BidirectionalStreamingCall(stream pb.ExampleService_BidirectionalStreamingCallServer) error {
	log.Println("BidirectionalStreamingCall started")
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		log.Printf("Received: %s", req.Message)
		if err := stream.Send(&pb.Response{Message: "Echo: " + req.Message}); err != nil {
			return err
		}
	}
}

// -------------------------------------------
// 2) Simple token‑based auth/interceptor logic
// -------------------------------------------

// getExpectedToken returns the token to check against, e.g. set via env var.
func getExpectedToken() string {
	return strings.TrimSpace(os.Getenv("AUTH_TOKEN")) // e.g. "Bearer super‑secret"
}

// unary interceptor
func authUnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		log.Println("🔐 No metadata received in unary call.")
	} else {
		log.Println("🔐 Unary Call Metadata:")
		for k, v := range md {
			log.Printf("  %s: %v\n", k, v)
		}
	}

	// Optional: log peer address
	if p, ok := peer.FromContext(ctx); ok {
		log.Printf("👤 Client Address: %s", p.Addr)
	}

	return handler(ctx, req)
}

// stream interceptor
func authStreamInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {

	expected := getExpectedToken()
	if expected == "" {
		return handler(srv, ss)
	}

	if md, ok := metadata.FromIncomingContext(ss.Context()); ok {
		auth := ""
		if vals := md.Get("authorization"); len(vals) > 0 {
			auth = vals[0]
		}
		if !strings.EqualFold(auth, expected) {
			return status.Error(codes.PermissionDenied, "invalid or missing token")
		}
	} else {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	return handler(srv, ss)
}

// -------------------
// 3) Boot the server
// -------------------

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(authUnaryInterceptor),
		grpc.StreamInterceptor(authStreamInterceptor),
	)

	pb.RegisterExampleServiceServer(grpcServer, &server{})
	reflection.Register(grpcServer)

	log.Println("gRPC server listening at :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
