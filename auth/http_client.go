// Package auth 提供认证相关功能的 HTTP 客户端
package auth

import (
	"kiro-go/httpclient"
	"time"
)

// 全局 HTTP 客户端，复用连接池
// 用于所有 auth 模块的 HTTP 请求
var httpClient = httpclient.New(30*time.Second, 50, 10)
