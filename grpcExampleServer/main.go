package main

import (
	"context"
	_ "crypto/tls"
	"fmt"
	"github.com/go-jose/go-jose/v4/jwt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	//"github.com/golang-jwt/jwt/v5"
	pb "github.com/example/userservice/pb" // Replace with your actual proto package
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	port      = ":50051"
	jwtSecret = "your-secret-key-change-this-in-production"
)

// Server implements the gRPC services
type Server struct {
	pb.UnimplementedAuthServiceServer
	pb.UnimplementedUserServiceServer
	pb.UnimplementedFileServiceServer

	users     map[string]*pb.User
	files     map[string][]byte
	chatRooms map[string][]*pb.ChatMessage
	mu        sync.RWMutex
}

// NewServer creates a new server instance
func NewServer() *Server {
	return &Server{
		users:     make(map[string]*pb.User),
		files:     make(map[string][]byte),
		chatRooms: make(map[string][]*pb.ChatMessage),
	}
}

// JWT Claims structure
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Authentication interceptor
func (s *Server) authInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// Skip authentication for login endpoint
	if info.FullMethod == "/userservice.AuthService/Login" {
		return handler(ctx, req)
	}

	// Extract metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
	}

	// Get authorization header
	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "missing authorization header")
	}

	// Validate token
	token := strings.TrimPrefix(authHeaders[0], "Bearer ")
	claims, err := s.validateToken(token)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}

	// Add user info to context
	ctx = context.WithValue(ctx, "user_id", claims.UserID)
	ctx = context.WithValue(ctx, "username", claims.Username)
	ctx = context.WithValue(ctx, "role", claims.Role)

	return handler(ctx, req)
}

// Stream authentication interceptor
func (s *Server) streamAuthInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	// Skip authentication for certain endpoints if needed
	ctx := ss.Context()

	// Extract metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Errorf(codes.Unauthenticated, "missing metadata")
	}

	// Get authorization header
	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return status.Errorf(codes.Unauthenticated, "missing authorization header")
	}

	// Validate token
	token := strings.TrimPrefix(authHeaders[0], "Bearer ")
	claims, err := s.validateToken(token)
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}

	// Create new context with user info
	newCtx := context.WithValue(ctx, "user_id", claims.UserID)
	newCtx = context.WithValue(newCtx, "username", claims.Username)
	newCtx = context.WithValue(newCtx, "role", claims.Role)

	// Create wrapped stream with new context
	wrapped := &wrappedStream{ServerStream: ss, ctx: newCtx}
	return handler(srv, wrapped)
}

// Wrapped stream for context propagation
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}

// Token validation
func (s *Server) validateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// Generate JWT token
func (s *Server) generateToken(userID, username, role string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

// AuthService implementation
func (s *Server) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	// Extract client metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		clientIP := md.Get("x-forwarded-for")
		userAgent := md.Get("user-agent")
		log.Printf("Login attempt from IP: %v, User-Agent: %v", clientIP, userAgent)
	}

	// Simple hardcoded authentication (replace with real auth)
	if req.Username == "admin" && req.Password == "password" {
		user := &pb.User{
			Id:        "1",
			Username:  req.Username,
			Email:     "admin@example.com",
			FirstName: "Admin",
			LastName:  "User",
			Role:      pb.UserRole_USER_ROLE_ADMIN,
			CreatedAt: timestamppb.Now(),
			UpdatedAt: timestamppb.Now(),
			IsActive:  true,
			Metadata: map[string]string{
				"last_login": time.Now().Format(time.RFC3339),
			},
		}

		s.mu.Lock()
		s.users[user.Id] = user
		s.mu.Unlock()

		accessToken, err := s.generateToken(user.Id, user.Username, user.Role.String())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to generate token: %v", err)
		}

		refreshToken, err := s.generateToken(user.Id, user.Username, user.Role.String())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to generate refresh token: %v", err)
		}

		// Send response metadata
		header := metadata.Pairs("server-version", "1.0.0")
		grpc.SendHeader(ctx, header)

		return &pb.LoginResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    86400, // 24 hours
			User:         user,
		}, nil
	}

	return nil, status.Errorf(codes.Unauthenticated, "invalid credentials")
}

func (s *Server) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	claims, err := s.validateToken(req.RefreshToken)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid refresh token")
	}

	newToken, err := s.generateToken(claims.UserID, claims.Username, claims.Role)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate new token")
	}

	return &pb.RefreshTokenResponse{
		AccessToken: newToken,
		ExpiresIn:   86400,
	}, nil
}

func (s *Server) Logout(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	// In a real implementation, you would invalidate the token
	log.Printf("User logged out: %v", ctx.Value("username"))
	return &emptypb.Empty{}, nil
}

// UserService implementation
func (s *Server) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	// Check authorization
	userRole := ctx.Value("role").(string)
	requestingUserID := ctx.Value("user_id").(string)

	if userRole != "USER_ROLE_ADMIN" && requestingUserID != req.Id {
		return nil, status.Errorf(codes.PermissionDenied, "insufficient permissions")
	}

	s.mu.RLock()
	user, exists := s.users[req.Id]
	s.mu.RUnlock()

	if !exists {
		return nil, status.Errorf(codes.NotFound, "user not found")
	}

	return user, nil
}

func (s *Server) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.User, error) {
	// Check if user is admin
	userRole := ctx.Value("role").(string)
	if userRole != "USER_ROLE_ADMIN" {
		return nil, status.Errorf(codes.PermissionDenied, "only admins can create users")
	}

	user := &pb.User{
		Id:        fmt.Sprintf("user_%d", time.Now().Unix()),
		Username:  req.Username,
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Role:      req.Role,
		CreatedAt: timestamppb.Now(),
		UpdatedAt: timestamppb.Now(),
		IsActive:  true,
		Metadata:  req.Metadata,
	}

	s.mu.Lock()
	s.users[user.Id] = user
	s.mu.Unlock()

	return user, nil
}

func (s *Server) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.User, error) {
	userRole := ctx.Value("role").(string)
	requestingUserID := ctx.Value("user_id").(string)

	if userRole != "USER_ROLE_ADMIN" && requestingUserID != req.Id {
		return nil, status.Errorf(codes.PermissionDenied, "insufficient permissions")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[req.Id]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "user not found")
	}

	// Update fields
	if req.Username != "" {
		user.Username = req.Username
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.FirstName != "" {
		user.FirstName = req.FirstName
	}
	if req.LastName != "" {
		user.LastName = req.LastName
	}
	if req.Role != pb.UserRole_USER_ROLE_UNSPECIFIED {
		user.Role = req.Role
	}
	user.IsActive = req.IsActive
	user.UpdatedAt = timestamppb.Now()

	if req.Metadata != nil {
		user.Metadata = req.Metadata
	}

	return user, nil
}

func (s *Server) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*emptypb.Empty, error) {
	userRole := ctx.Value("role").(string)
	if userRole != "USER_ROLE_ADMIN" {
		return nil, status.Errorf(codes.PermissionDenied, "only admins can delete users")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[req.Id]; !exists {
		return nil, status.Errorf(codes.NotFound, "user not found")
	}

	delete(s.users, req.Id)
	return &emptypb.Empty{}, nil
}

// Server streaming RPC
func (s *Server) ListUsers(req *pb.ListUsersRequest, stream pb.UserService_ListUsersServer) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, user := range s.users {
		// Apply filters
		if req.RoleFilter != pb.UserRole_USER_ROLE_UNSPECIFIED && user.Role != req.RoleFilter {
			continue
		}
		if req.ActiveOnly && !user.IsActive {
			continue
		}

		if err := stream.Send(user); err != nil {
			return err
		}

		count++
		if req.PageSize > 0 && int32(count) >= req.PageSize {
			break
		}
	}

	return nil
}

// Client streaming RPC
func (s *Server) CreateMultipleUsers(stream pb.UserService_CreateMultipleUsersServer) error {
	var users []*pb.User
	var errors []string

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			// End of stream
			break
		}
		if err != nil {
			return err
		}

		user := &pb.User{
			Id:        fmt.Sprintf("user_%d_%d", time.Now().Unix(), len(users)),
			Username:  req.Username,
			Email:     req.Email,
			FirstName: req.FirstName,
			LastName:  req.LastName,
			Role:      req.Role,
			CreatedAt: timestamppb.Now(),
			UpdatedAt: timestamppb.Now(),
			IsActive:  true,
			Metadata:  req.Metadata,
		}

		s.mu.Lock()
		s.users[user.Id] = user
		s.mu.Unlock()

		users = append(users, user)
	}

	return stream.SendAndClose(&pb.CreateMultipleUsersResponse{
		Users:        users,
		CreatedCount: int32(len(users)),
		Errors:       errors,
	})
}

// Bidirectional streaming RPC
func (s *Server) ChatWithSupport(stream pb.UserService_ChatWithSupportServer) error {
	userID := stream.Context().Value("user_id").(string)

	// Start goroutine to handle incoming messages
	go func() {
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Printf("Error receiving message: %v", err)
				return
			}

			// Store message
			s.mu.Lock()
			s.chatRooms[userID] = append(s.chatRooms[userID], msg)
			s.mu.Unlock()

			// Echo message back with system response
			response := &pb.ChatMessage{
				Id:        fmt.Sprintf("msg_%d", time.Now().Unix()),
				UserId:    "system",
				Message:   fmt.Sprintf("Support received: %s", msg.Message),
				Timestamp: timestamppb.Now(),
				Type:      pb.MessageType_MESSAGE_TYPE_SYSTEM,
			}

			if err := stream.Send(response); err != nil {
				log.Printf("Error sending message: %v", err)
				return
			}
		}
	}()

	// Keep connection alive
	select {
	case <-stream.Context().Done():
		return stream.Context().Err()
	}
}

// FileService implementation
func (s *Server) UploadFile(stream pb.FileService_UploadFileServer) error {
	var fileID string
	var filename string
	var fileData []byte

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if fileID == "" {
			fileID = chunk.FileId
			filename = chunk.Filename
		}

		fileData = append(fileData, chunk.Data...)
	}

	s.mu.Lock()
	s.files[fileID] = fileData
	s.mu.Unlock()

	return stream.SendAndClose(&pb.UploadResponse{
		FileId:   fileID,
		Filename: filename,
		FileSize: int64(len(fileData)),
		Message:  "File uploaded successfully",
	})
}

func (s *Server) DownloadFile(req *pb.DownloadRequest, stream pb.FileService_DownloadFileServer) error {
	s.mu.RLock()
	fileData, exists := s.files[req.FileId]
	s.mu.RUnlock()

	if !exists {
		return status.Errorf(codes.NotFound, "file not found")
	}

	chunkSize := 1024 // 1KB chunks
	totalChunks := int64((len(fileData) + chunkSize - 1) / chunkSize)

	for i := 0; i < len(fileData); i += chunkSize {
		end := i + chunkSize
		if end > len(fileData) {
			end = len(fileData)
		}

		chunk := &pb.FileChunk{
			FileId:      req.FileId,
			Data:        fileData[i:end],
			ChunkNumber: int64(i/chunkSize + 1),
			TotalChunks: totalChunks,
		}

		if err := stream.Send(chunk); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	// Create server instance
	server := NewServer()

	// Create listener
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Create TLS credentials (optional)
	creds, err := credentials.NewServerTLSFromFile("server.crt", "server.key")
	if err != nil {
		log.Printf("Failed to load TLS credentials, using insecure connection: %v", err)
		creds = nil
	}

	// Create gRPC server with interceptors
	var opts []grpc.ServerOption
	if creds != nil {
		opts = append(opts, grpc.Creds(creds))
	}

	opts = append(opts,
		grpc.UnaryInterceptor(server.authInterceptor),
		grpc.StreamInterceptor(server.streamAuthInterceptor),
	)

	s := grpc.NewServer(opts...)

	// Register services
	pb.RegisterAuthServiceServer(s, server)
	pb.RegisterUserServiceServer(s, server)
	pb.RegisterFileServiceServer(s, server)

	log.Printf("gRPC server listening on port %s", port)
	log.Printf("Server supports:")
	log.Printf("- JWT Authentication")
	log.Printf("- Metadata handling")
	log.Printf("- Unary RPCs")
	log.Printf("- Server streaming RPCs")
	log.Printf("- Client streaming RPCs")
	log.Printf("- Bidirectional streaming RPCs")
	log.Printf("- TLS (if certificates are available)")

	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
