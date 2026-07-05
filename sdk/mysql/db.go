// Package mysql 提供基于 sqlx 的 MySQL Repository 实现。
package mysql

import (
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// NewDB 创建 MySQL 连接池。
func NewDB(dsn string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
	}
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(1 * time.Hour)
	db.SetConnMaxIdleTime(10 * time.Minute)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("MySQL Ping 失败: %w", err)
	}
	return db, nil
}

// ============================================================
// 时间转换（MySQL TIMESTAMP ↔ Unix 毫秒）
// ============================================================

// MsToTime Unix 毫秒 → time.Time
func MsToTime(ms int64) time.Time { return time.UnixMilli(ms) }

// TimeToMs time.Time → Unix 毫秒
func TimeToMs(t time.Time) int64 { return t.UnixMilli() }

// PtrTimeToMs *time.Time → *int64（nil → nil）
func PtrTimeToMs(t *time.Time) *int64 {
	if t == nil {
		return nil
	}
	ms := t.UnixMilli()
	return &ms
}
