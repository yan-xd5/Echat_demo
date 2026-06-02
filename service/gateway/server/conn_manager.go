package main

import (
	"sync"

	"github.com/gorilla/websocket"
)

// ConnManager 管理用户与 WebSocket 连接的映射
type ConnManager struct {
	mu   sync.RWMutex
	conns map[string]*websocket.Conn // user_id → WebSocket 连接
}

// NewConnManager 创建连接管理器
func NewConnManager() *ConnManager {
	return &ConnManager{
		conns: make(map[string]*websocket.Conn),
	}
}

// Add 注册用户连接
func (m *ConnManager) Add(userID string, conn *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// 如果已有旧连接，先关闭（多端登录踢下线）
	if old, ok := m.conns[userID]; ok {
		old.Close()
	}
	m.conns[userID] = conn
}

// Remove 移除用户连接
func (m *ConnManager) Remove(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.conns, userID)
}

// Get 获取用户的 WebSocket 连接（nil 表示不在线）
func (m *ConnManager) Get(userID string) *websocket.Conn {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.conns[userID]
}

// OnlineUsers 返回当前在线用户数
func (m *ConnManager) OnlineUsers() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.conns)
}