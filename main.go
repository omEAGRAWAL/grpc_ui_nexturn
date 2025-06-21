package main

import (
	"github.com/gin-gonic/gin"
	"grpc_ui/internals/handler"
)

func main() {
	// Create a new Gin router instance
	router := gin.Default()

	// Handle CORS
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Serve static files for the React UI
	router.Static("/assets", "./GRPC_UI/dist/assets")

	// Serve index.html for any unrecognized routes (for SPA routing)
	router.NoRoute(func(c *gin.Context) {
		c.File("./GRPC_UI/dist/index.html")
	})

	// API routes
	router.POST("/api/upload/proto", handler.HandleProtoUpload)      // Upload .proto files
	router.GET("/api/listServices", handler.HandleListServices)      // List services/methods
	router.GET("/grpc/ws/stream", handler.HandleGRPCWebSocketStream) // gRPC via WebSocket (all modes)
	router.POST("/rtc/offer", handler.HandleRTCOffer)                // WebRTC offer handler
	router.POST("/rtc/answer", handler.HandleRTCAnswer)              // WebRTC answer handler

	// Start the server on port 8081
	if err := router.Run("0.0.0.0:8081"); err != nil {
		panic("Failed to start server: " + err.Error())
	}
}
