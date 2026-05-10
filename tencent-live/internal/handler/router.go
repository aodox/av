package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"tencent-live/internal/model"
	"tencent-live/internal/service"
)

func NewRouter(streamService *service.StreamService, callbackKey string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(RequestIDMiddleware())
	r.Use(LoggerMiddleware())
	r.Use(CORSMiddleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "ok",
			"timestamp": time.Now().Unix(),
		})
	})

	streamHandler := NewStreamHandler(streamService)
	callbackHandler := NewCallbackHandler(streamService, callbackKey)

	v1 := r.Group("/v1")
	{
		// 腾讯云回调接口（不需要鉴权）
		// 统一入口：处理推流/断流/录制/截图/审核/异常等所有回调
		v1.POST("/callback/event", callbackHandler.HandleCallback)
	}

	api := r.Group("/api/v1")
	{
		stream := api.Group("/stream")
		{
			stream.POST("/create", streamHandler.Create)
			stream.POST("/close", streamHandler.Close)
			stream.GET("/push-url", streamHandler.GetPushURL)
			stream.GET("/play-url", streamHandler.GetPlayURL)
			stream.GET("/status", streamHandler.GetStatus)
			stream.GET("/list", streamHandler.List)
		}
	}

	return r
}

// RequestIDMiddleware 请求ID中间件（用于链路追踪）
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = model.GenerateRequestID()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func LoggerMiddleware() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health"},
	})
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
