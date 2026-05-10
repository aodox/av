package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"tencent-live/internal/config"
	"tencent-live/internal/handler"
	"tencent-live/internal/logger"
	"tencent-live/internal/service"
	"tencent-live/internal/store/cache"
	"tencent-live/internal/store/db"
	"tencent-live/internal/tencent"
)

var configPath = flag.String("config", "./config/config.yaml", "config file path")

func main() {
	flag.Parse()

	// 利用所有CPU核心
	runtime.GOMAXPROCS(runtime.NumCPU())

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("load config error: %v\n", err)
		os.Exit(1)
	}

	if err := logger.Init(cfg.Log); err != nil {
		fmt.Printf("init logger error: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Infof("starting tencent-live server... (CPUs: %d)", runtime.NumCPU())

	// 初始化 MySQL
	if err := db.Init(cfg.MySQL); err != nil {
		logger.Fatalf("init mysql failed: %v", err)
	}

	// 初始化 Redis
	if err := cache.Init(cfg.Redis); err != nil {
		logger.Fatalf("init redis failed: %v", err)
	}
	defer cache.Close()

	// 初始化异步写入器（Redis优先，批量写MySQL）
	cache.InitAsyncWriter(
		cfg.AsyncWriter.BatchSize,
		cfg.AsyncWriter.IntervalMs,
		db.BatchCreateStreams,
		db.BatchUpdateStreams,
		db.BatchUpsertDailyLogs,
	)
	defer cache.StopAsyncWriter()
	logger.Infof("async writer started: batchSize=%d, interval=%dms",
		cfg.AsyncWriter.BatchSize, cfg.AsyncWriter.IntervalMs)

	// 初始化腾讯云客户端
	tencentClient := tencent.NewClient(cfg.Tencent)
	logger.Info("tencent client initialized")

	// 初始化服务
	streamService := service.NewStreamService(tencentClient, cfg.Tencent, cfg.Stream)
	logger.Infof("stream config: nameWithTimestamp=%v", cfg.Stream.NameWithTimestamp)

	// 启动流状态监控（作为回调的补充，处理异常情况）
	monitor := service.NewMonitor(tencentClient, streamService, cfg.Monitor)
	if cfg.Monitor.Enabled {
		monitor.Start()
		defer monitor.Stop()
		logger.Info("stream monitor started (as callback fallback)")
	} else {
		logger.Info("stream monitor disabled (using callback only)")
	}

	// 初始化路由
	router := handler.NewRouter(streamService, cfg.Tencent.CallbackKey)

	// HTTP服务器配置
	srv := &http.Server{
		Addr:           fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:        router,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	go func() {
		logger.Infof("[HTTP] server listening on :%d", cfg.Server.Port)
		logger.Infof("[HTTP] callback url: http://YOUR_IP:%d/v1/callback/event", cfg.Server.Port)
		logger.Infof("[HTTP] api base url: http://YOUR_IP:%d/api/v1/", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("[HTTP] listen error: %v", err)
		}
	}()

	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatalf("server shutdown error: %v", err)
	}

	logger.Info("server exited")
}
