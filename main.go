// Package main provides the entry point for Kiro API Proxy.
//
// Kiro API Proxy is a reverse proxy service that translates Kiro API requests
// into OpenAI and Anthropic (Claude) compatible formats. Key features include:
//   - Multi-account pool with round-robin load balancing
//   - Automatic OAuth token refresh
//   - Streaming response support for real-time AI interactions
//   - Admin panel for account and configuration management
//
// The service exposes the following endpoints:
//   - /v1/messages - Claude API compatible endpoint
//   - /v1/chat/completions - OpenAI API compatible endpoint
//   - /admin - Web-based administration panel
package main

import (
	"fmt"
	"io"
	"kiro-go/config"
	"kiro-go/pool"
	"kiro-go/proxy"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	initLogging()

	// 配置文件路径，支持环境变量覆盖
	configPath := "data/config.json"
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		configPath = envPath
	}

	// 确保数据目录存在
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// 加载配置
	if err := config.Init(configPath); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 环境变量覆盖密码
	if envPassword := os.Getenv("ADMIN_PASSWORD"); envPassword != "" {
		config.SetPassword(envPassword)
	}

	// 初始化账号池
	pool.GetPool()

	// 创建 HTTP 处理器（包含后台刷新任务）
	handler := proxy.NewHandler()

	// 启动服务器
	addr := fmt.Sprintf("%s:%d", config.GetHost(), config.GetPort())
	log.Printf("Kiro-Go starting on http://%s", addr)
	log.Printf("Admin panel: http://%s/admin", addr)
	log.Printf("Claude API: http://%s/v1/messages", addr)
	log.Printf("OpenAI API: http://%s/v1/chat/completions", addr)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func initLogging() {
	logPath := os.Getenv("LOG_PATH")
	if logPath == "" {
		logPath = "data/kiro-go.log"
	}

	dir := filepath.Dir(logPath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("Failed to create log directory: %v", err)
			return
		}
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		log.Printf("Failed to open log file %s: %v", logPath, err)
		return
	}

	log.SetOutput(io.MultiWriter(os.Stdout, file))
}
