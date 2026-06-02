package main

import (
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL 驱动
	"github.com/jmoiron/sqlx"
)

// NewDB 创建 MySQL 连接池
// dsn 格式: "user:password@tcp(127.0.0.1:3306)/echat?charset=utf8mb4&parseTime=true"
func NewDB(dsn string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
	}

	// 连接池配置
	db.SetMaxOpenConns(50)                  // 最大连接数
	db.SetMaxIdleConns(10)                  // 最大空闲连接数
	db.SetConnMaxLifetime(1 * time.Hour)    // 连接最大存活时间
	db.SetConnMaxIdleTime(10 * time.Minute) // 空闲连接最大空闲时间

	// 验证连接是否正常
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("MySQL Ping 失败: %w", err)
	}

	return db, nil
}
