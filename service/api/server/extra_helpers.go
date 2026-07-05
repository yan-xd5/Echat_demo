package main

import (
	"encoding/json"
	"net/http"
)

// writeJSON 直接写入 JSON 响应，绕过 tRPC 序列化。
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	if w == nil {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}
