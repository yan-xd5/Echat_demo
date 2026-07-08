// Package route 提供会话路由、seq_id 生成等 Gateway/Controller 共用逻辑。
package route

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// ============================================================
// extractSessionID
// ============================================================

// ExtractSessionID 从 RawBody 预提取 sessionID，仅解析 type/to/group_id。
// 解析失败兜底返回 fromUID，不阻塞流程。
func ExtractSessionID(rawBody []byte, fromUID string) string {
	var raw struct {
		Type    string `json:"type"`
		To      string `json:"to"`
		GroupID string `json:"group_id"`
	}
	json.Unmarshal(rawBody, &raw) // 失败时 raw 为零值，default 兜底

	switch raw.Type {
	case "chat", "typing":
		a, b := fromUID, raw.To
		if a > b {
			a, b = b, a
		}
		return a + "_" + b
	case "read_receipt", "delivery_ack":
		return fromUID
	case "group_chat":
		return raw.GroupID
	default:
		return fromUID
	}
}

// ============================================================
// FNV-32a 哈希
// ============================================================

// Hash 计算 sessionID 的 FNV-32a 哈希值，用于一致性哈希环。
func Hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// ============================================================
// SessionRouter — 一致性哈希路由
// ============================================================

const vnodesPerInstance = 150 // 每实例虚拟节点数

// SessionRouter 基于服务实例的一致性哈希路由器。
// 背景协程每 30s 刷新哈希环，Resolve 仅读锁查找，避免每次消息都调 discovery。
type SessionRouter struct {
	discovery ServiceDiscovery
	mu        sync.RWMutex
	hashRing  map[uint32]string // hash → instanceID
	hashKeys  []uint32          // 排序的哈希值（二分查找）
	addrMap   map[string]string // instanceID → address (ip:port)
	ready     atomic.Bool       // 首次刷新完成
	stopCh    chan struct{}
	stopped   atomic.Bool
}

// NewSessionRouter 创建会话路由器并启动背景刷新协程。
func NewSessionRouter(discovery ServiceDiscovery) *SessionRouter {
	r := &SessionRouter{
		discovery: discovery,
		stopCh:    make(chan struct{}),
	}
	go r.run()
	return r
}

// run 背景协程：首次立即刷新，之后每 30s 刷新。
func (r *SessionRouter) run() {
	r.refresh()
	r.ready.Store(true)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.refresh()
		case <-r.stopCh:
			return
		}
	}
}

// refresh 从 discovery 拉取实例并重建哈希环。
func (r *SessionRouter) refresh() {
	instances, err := r.discovery.GetInstances("echat.controller.ControllerService")
	if err != nil || len(instances) == 0 {
		return // 保留旧环
	}
	r.mu.Lock()
	r.buildRing(instances)
	r.mu.Unlock()
}

// Close 停止背景刷新协程。
func (r *SessionRouter) Close() {
	if r.stopped.CompareAndSwap(false, true) {
		close(r.stopCh)
	}
}

// Resolve 根据 sessionID 从缓存哈希环查找 Controller 实例地址。
func (r *SessionRouter) Resolve(sessionID string) (string, error) {
	if !r.ready.Load() {
		// 首次刷新未完成，同步等待一次
		r.refresh()
		r.ready.Store(true)
	}
	r.mu.RLock()
	addr := r.lookupAddr(sessionID)
	r.mu.RUnlock()

	if addr == "" {
		return "", fmt.Errorf("no instance for session %s", sessionID)
	}
	return "ip://" + addr, nil
}

// lookupAddr 组合 lookup + addrMap 查询（需持有读锁）。
func (r *SessionRouter) lookupAddr(sessionID string) string {
	if len(r.hashKeys) == 0 {
		return ""
	}
	h := Hash(sessionID)
	idx := sort.Search(len(r.hashKeys), func(i int) bool { return r.hashKeys[i] >= h })
	if idx == len(r.hashKeys) {
		idx = 0
	}
	instID := r.hashRing[r.hashKeys[idx]]
	return r.addrMap[instID]
}

// buildRing 用服务实例构建一致性哈希环。
func (r *SessionRouter) buildRing(instances []ServiceInstance) {
	r.hashRing = make(map[uint32]string)
	r.hashKeys = make([]uint32, 0, len(instances)*vnodesPerInstance)
	r.addrMap = make(map[string]string)

	for _, inst := range instances {
		id := inst.ID
		if id == "" {
			id = inst.Address // 兜底用 address 作 ID
		}
		r.addrMap[id] = inst.Address

			for i := 0; i < vnodesPerInstance; i++ {
				var h uint32
				for attempt := 0; ; attempt++ {
					h = Hash(fmt.Sprintf("%s#%d#%d", id, i, attempt))
					if _, exists := r.hashRing[h]; !exists {
						break
					}
				}
				r.hashRing[h] = id
				r.hashKeys = append(r.hashKeys, h)
			}
	}
	sort.Slice(r.hashKeys, func(i, j int) bool { return r.hashKeys[i] < r.hashKeys[j] })
}


// ============================================================
// GenSeqID — Redis 分布式锁 + INCR
// ============================================================

// GenSeqID 通过 Redis SETNX 锁 + INCR 获取会话内严格递增的 seq_id。
// 锁超时 100ms，获取失败重试 1 次。
func GenSeqID(ctx context.Context, rdb *redis.Client, sessionID string) (int64, error) {
	lockKey := "lock:seq:" + sessionID
	seqKey := "seq:" + sessionID
	lockVal := fmt.Sprintf("%d", time.Now().UnixNano())

	for attempt := 0; attempt < 2; attempt++ {
		ok, err := rdb.SetNX(ctx, lockKey, lockVal, 100*time.Millisecond).Result()
		if err != nil {
			return 0, fmt.Errorf("seq lock error: %w", err)
		}
		if ok {
			defer rdb.Del(ctx, lockKey)
			seq, err := rdb.Incr(ctx, seqKey).Result()
			if err != nil {
				return 0, fmt.Errorf("seq incr error: %w", err)
			}
			return seq, nil
		}
		if attempt == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	return 0, fmt.Errorf("seq lock timeout for session %s", sessionID)
}
