package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func init() {
	// 自动加载当前目录的 .env 文件
	_ = godotenv.Load()
}

// GetDSN 从环境变量读取并拼装 MySQL 连接串
func GetDSN() string {
	host := envOr("DB_HOST", "127.0.0.1")
	port := envOr("DB_PORT", "3306")
	user := envOr("DB_USER", "root")
	pass := envOr("DB_PASS", "")
	name := envOr("DB_NAME", "chat")

	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true",
		user, pass, host, port, name)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
