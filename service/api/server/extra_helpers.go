package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"echat/sdk/observability"
)

// corsOrigin CORS 允许的源（环境变量 CORS_ORIGIN，默认仅本地开发）。
var corsOrigin = func() string {
	if v := os.Getenv("CORS_ORIGIN"); v != "" {
		return v
	}
	return "http://localhost:3000"
}()

// writeJSON 直接写入 JSON 响应，绕过 tRPC 序列化。
// ctx 用于回写 HTTP 状态码到 metrics filter。
func writeJSON(ctx context.Context, w http.ResponseWriter, statusCode int, data interface{}) {
	if w == nil {
		return
	}
	// 回写状态码到 context，供 metrics filter 采集
	observability.SetHTTPStatus(ctx, statusCode)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", corsOrigin)
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}
