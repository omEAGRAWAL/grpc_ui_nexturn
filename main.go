package main

import (
	"github.com/gin-gonic/gin"
	"grpc_ui/internals/handler"
)

func main() {

	r := gin.Default()

	//handle cors
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})
	r.Static("/assets", "./dist/assets")
	r.NoRoute(func(c *gin.Context) {
		c.File("./dist/index.html")
	})
	r.POST("/api/upload/proto", handler.HandleProtoUpload)      // Upload .proto
	r.GET("/api/listServices", handler.HandleListServices)      // List services and methods
	r.GET("/grpc/ws/stream", handler.HandleGRPCWebSocketStream) // All streaming types via WebSocket
	r.POST("/rtc/offer", handler.HandleRTCOffer)
	r.POST("/rtc/answer", handler.HandleRTCAnswer)

	//r.StaticFile("/", "./dist")
	r.Run(":8081")
}
