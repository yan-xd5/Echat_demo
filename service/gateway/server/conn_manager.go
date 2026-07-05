package main

import (
	"hash/fnv"
	"sync"
	"sync/atomic"

	"trpc.group/trpc-go/trpc-go/log"
)

// Bucket 一个 uid 分片，内部 map[uid]map[deviceID]*UserSession
type Bucket struct {
	mu       sync.RWMutex
	sessions map[string]map[string]*UserSession
}

// ConnManager 管理 uid → Session 映射，64 Bucket 分片。
type ConnManager struct {
	buckets  [64]Bucket
	count    atomic.Int64
	maxConn  int64
	limitMu sync.Mutex // 保护 maxConn 检查的全局互斥锁
}

// NewConnManager 创建连接管理器。
func NewConnManager(maxConn int64) *ConnManager {
	m := &ConnManager{maxConn: maxConn}
	for i := range m.buckets {
		m.buckets[i].sessions = make(map[string]map[string]*UserSession)
	}
	return m
}

func (m *ConnManager) getBucket(uid string) *Bucket {
	h := fnv.New32a()
	h.Write([]byte(uid))
	return &m.buckets[h.Sum32()%64]
}

// Register 注册新连接，含连接数上限检查。
func (m *ConnManager) Register(uid, deviceID, platform string, session *UserSession) error {
	b := m.getBucket(uid)
	b.mu.Lock()
	defer b.mu.Unlock()

	// 全局互斥保证跨 bucket 的 count 检查原子性
	m.limitMu.Lock()
	if m.count.Load() >= m.maxConn {
		m.limitMu.Unlock()
		return ErrMaxConnReached
	}
	m.count.Add(1)
	m.limitMu.Unlock()

	if b.sessions[uid] == nil {
		b.sessions[uid] = make(map[string]*UserSession)
	}
	b.sessions[uid][deviceID] = session
	log.Infof("[Gateway] 连接注册: uid=%s, device=%s, platform=%s, 在线数=%d", uid, deviceID, platform, m.count.Load())
	return nil
}

// Unregister 移除连接。
func (m *ConnManager) Unregister(uid, deviceID string) {
	b := m.getBucket(uid)
	b.mu.Lock()
	defer b.mu.Unlock()

	sessions := b.sessions[uid]
	if sessions == nil {
		return
	}
	if _, ok := sessions[deviceID]; !ok {
		return // 已注销，防止重复减 count
	}
	delete(sessions, deviceID)
	if len(sessions) == 0 {
		delete(b.sessions, uid)
	}
	m.count.Add(-1)
	log.Infof("[Gateway] 连接注销: uid=%s, device=%s, 在线数=%d", uid, deviceID, m.count.Load())
}

// LookupWriteChs 返回 uid 所有在线设备的 WriteCh 切片。
func (m *ConnManager) LookupWriteChs(uid string) []chan<- []byte {
	b := m.getBucket(uid)
	b.mu.RLock()
	defer b.mu.RUnlock()

	sessions := b.sessions[uid]
	if sessions == nil {
		return nil
	}
	chs := make([]chan<- []byte, 0, len(sessions))
	for _, s := range sessions {
		chs = append(chs, s.WriteCh)
	}
	return chs
}

// LookupSessionByPlatform 查找同 uid+platform 的旧 Session（顶号用）。
func (m *ConnManager) LookupSessionByPlatform(uid, platform string) *UserSession {
	b := m.getBucket(uid)
	b.mu.RLock()
	defer b.mu.RUnlock()

	// 遍历所有设备，排除新注册的 session（由调用方传入的指针比较）
	for _, s := range b.sessions[uid] {
		if s.Platform == platform {
			return s
		}
	}
	return nil
}

// LookupOldSession 查找 uid 同 platform 且不等于 exclude 的旧 Session。
func (m *ConnManager) LookupOldSession(uid, platform string, exclude *UserSession) *UserSession {
	b := m.getBucket(uid)
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, s := range b.sessions[uid] {
		if s.Platform == platform && s != exclude {
			return s
		}
	}
	return nil
}

// Len 返回当前连接数。
func (m *ConnManager) Len() int64 { return m.count.Load() }

// Shutdown 优雅关闭：逐 Bucket cancel 全部 Session。
func (m *ConnManager) Shutdown() {
	for i := range m.buckets {
		b := &m.buckets[i]
		b.mu.Lock()
		for uid, sessions := range b.sessions {
			for _, s := range sessions {
				s.Cancel()
			}
			delete(b.sessions, uid)
		}
		b.mu.Unlock()
	}
}

// ============================================================
// 顶号编排（调用方）
// ============================================================

// KickOldAndRegister 顶号：同 uid+platform 旧连接 Cancel，注册新连接。
// writer defer 负责 Unregister + RemoveSession，此处仅 Cancel。
func KickOldAndRegister(mgr *ConnManager, uid, deviceID, platform string, session *UserSession) error {
	if err := mgr.Register(uid, deviceID, platform, session); err != nil {
		return err
	}
	if old := mgr.LookupOldSession(uid, platform, session); old != nil {
		old.Cancel()
	}
	return nil
}

// ErrMaxConnReached 连接数超限
var ErrMaxConnReached = &ConnError{"连接数已达上限"}

// ConnError 连接错误
type ConnError struct{ Msg string }

func (e *ConnError) Error() string { return e.Msg }
