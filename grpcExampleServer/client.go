package main

import (
	"context"
	"io"
	"log"
	"time"

	pb "grpcExampleServer/examplepb" // Adjust the import path to your generated protobuf package

	"google.golang.org/grpc"
)

func main() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewExampleServiceClient(conn)

	// Unary Call
	doUnaryCall(client)

	// Server Streaming Call
	doServerStreamingCall(client)

	// Client Streaming Call
	doClientStreamingCall(client)

	// Bidirectional Streaming Call
	doBidirectionalStreamingCall(client)
}

func doUnaryCall(client pb.ExampleServiceClient) {
	log.Println("UnaryCall...")
	resp, err := client.UnaryCall(context.Background(), &pb.Request{Message: "Hello"})
	if err != nil {
		log.Fatalf("UnaryCall failed: %v", err)
	}
	log.Printf("Response: %s", resp.Message)
}

func doServerStreamingCall(client pb.ExampleServiceClient) {
	log.Println("ServerStreamingCall...")
	stream, err := client.ServerStreamingCall(context.Background(), &pb.Request{Message: "Hello Stream"})
	if err != nil {
		log.Fatalf("ServerStreamingCall failed: %v", err)
	}
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("ServerStreamingCall error: %v", err)
		}
		log.Printf("Received: %s", resp.Message)
	}
}

func doClientStreamingCall(client pb.ExampleServiceClient) {
	log.Println("ClientStreamingCall...")
	stream, err := client.ClientStreamingCall(context.Background())
	if err != nil {
		log.Fatalf("ClientStreamingCall failed: %v", err)
	}
	messages := []string{"A", "B", "C"}
	for _, msg := range messages {
		if err := stream.Send(&pb.Request{Message: msg}); err != nil {
			log.Fatalf("Send failed: %v", err)
		}
		log.Printf("Sent: %s", msg)
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("CloseAndRecv failed: %v", err)
	}
	log.Printf("Response: %s", resp.Message)
}

func doBidirectionalStreamingCall(client pb.ExampleServiceClient) {
	log.Println("BidirectionalStreamingCall...")
	stream, err := client.BidirectionalStreamingCall(context.Background())
	if err != nil {
		log.Fatalf("BidirectionalStreamingCall failed: %v", err)
	}
	done := make(chan struct{})

	// Receive
	go func() {
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("Recv error: %v", err)
			}
			log.Printf("Received: %s", resp.Message)
		}
		close(done)
	}()

	// Send
	messages := []string{"X", "Y", "Z"}
	for _, msg := range messages {
		if err := stream.Send(&pb.Request{Message: msg}); err != nil {
			log.Fatalf("Send error: %v", err)
		}
		log.Printf("Sent: %s", msg)
		time.Sleep(time.Second)
	}

	stream.CloseSend()
	<-done
}
