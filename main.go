package main

import (
	"context"
	"log"
	"time"

	_ "github.com/zolagz/ytb2bili/docs" // Swagger docs
	"github.com/zolagz/ytb2bili/internal/bootstrap"
	"github.com/zolagz/ytb2bili/internal/handler"
)

// 构建信息（通过 -ldflags 注入）
var (
	Version   = "dev"      // 版本号
	BuildTime = "unknown"  // 构建时间
	CommitSHA = "unknown"  // Git commit SHA
)

// @title           YouTube to Bilibili API
// @version         1.0
// @description     API for downloading YouTube videos and uploading to Bilibili with AI-powered processing
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      127.0.0.1:8096
// @BasePath  /api

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	// 设置构建信息到 handler
	handler.SetBuildInfo(Version, BuildTime, CommitSHA)
	
	// 打印版本信息
	log.Printf("🚀 YTB2BILI 启动中... 版本: %s, 构建时间: %s\n", Version, BuildTime)
	
	// 创建应用（配置会自动加载）
	app := bootstrap.NewApp()

	// 启动应用（带超时）
	startCtx, startCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer startCancel()

	if err := app.Start(startCtx); err != nil {
		log.Fatalf("❌ 应用启动失败: %v\n", err)
	}

	log.Println("✅ 应用启动成功，按 Ctrl+C 停止...")

	// fx 内部订阅 SIGINT / SIGTERM，Done() 在收到信号后关闭
	<-app.Done()
	log.Println("\n🛑 收到终止信号，正在优雅关闭...")

	// 停止应用（带超时）
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer stopCancel()

	if err := app.Stop(stopCtx); err != nil {
		log.Printf("❌ 应用关闭出错: %v\n", err)
		log.Fatal("强制退出")
	}

	log.Println("✅ 应用已成功关闭")
}

//
//yt-dlp: /usr/local/bin/yt-dlp
//ffmpeg: /usr/bin/ffmpeg