package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	config "github.com/meetmonth/dance-elite/internal/configs"
	"github.com/meetmonth/dance-elite/internal/pkg/logger"
)

// main 只做三件事：确定配置、初始化日志、进入服务生命周期。
// 配置读取见 internal/configs，服务装配见 run.go。
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	level, err := logger.ParseLevel(cfg.Log.Level)
	if err != nil {
		log.Fatalf("解析日志级别失败: %v", err)
	}
	if err := logger.Init(logger.Options{Level: level, Format: cfg.Log.Format}); err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}
	// 不再有 DEBUG 开关：日志级别为 debug 时使用 Gin debug 模式，否则 release。
	if cfg.Log.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 监听系统信号，触发优雅关闭。
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg); err != nil {
		logger.Log.Fatalf("服务运行失败: %v", err)
	}
}
