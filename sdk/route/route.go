// Package route 提供会话路由、seq_id 生成等 Gateway/Controller 共用逻辑。
package route

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"sync"
	"time"

	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
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
// SessionRouter — Polaris 一致性哈希路由
// ============================================================

const vnodesPerInstance = 150 // 每实例虚拟节点数

// SessionRouter 基于 Polaris 实例 ID 的一致性哈希路由器。
type SessionRouter struct {
	consumer    api.ConsumerAPI
	mu          sync.RWMutex
	hashRing    map[uint32]string          // hash → instanceID
	hashKeys    []uint32                   // 排序的哈希值（二分查找）
	instanceMap map[string]model.Instance  // instanceID → Instance
}

// NewSessionRouter 创建会话路由器。
func NewSessionRouter(consumer api.ConsumerAPI) *SessionRouter {
	return &SessionRouter{consumer: consumer}
}

// Resolve 根据 sessionID 路由到 Controller 实例地址。
func (r *SessionRouter) Resolve(sessionID string) (string, error) {
	resp, err := r.consumer.GetAllInstances(&api.GetAllInstancesRequest{
		GetAllInstancesRequest: model.GetAllInstancesRequest{
			Namespace: "default",
			Service:   "echat.controller.ControllerService",
		},
	})
	if err != nil {
		return "", fmt.Errorf("resolve: %w", err)
	}

	r.mu.Lock()
	r.buildRing(resp.Instances)
	instID := r.lookup(sessionID)
	inst := r.instanceMap[instID]
	r.mu.Unlock()

	if inst == nil {
		return "", fmt.Errorf("no instance for session %s", sessionID)
	}
	return fmt.Sprintf("ip://%s:%d", inst.GetHost(), inst.GetPort()), nil
}

// buildRing 用 Polaris 实例 ID 构建一致性哈希环。
func (r *SessionRouter) buildRing(instances []model.Instance) {
	r.hashRing = make(map[uint32]string)
	r.hashKeys = make([]uint32, 0, len(instances)*vnodesPerInstance)
	r.instanceMap = make(map[string]model.Instance)

	for _, inst := range instances {
		if !inst.IsHealthy() {
			continue
		}
		id := inst.GetId()
		r.instanceMap[id] = inst

		for i := 0; i < vnodesPerInstance; i++ {
			h := Hash(fmt.Sprintf("%s#%d", id, i))
			r.hashRing[h] = id
			r.hashKeys = append(r.hashKeys, h)
		}
	}
	sort.Slice(r.hashKeys, func(i, j int) bool { return r.hashKeys[i] < r.hashKeys[j] })
}

// lookup 在哈希环上顺时针查找 sessionID 对应的实例 ID。
func (r *SessionRouter) lookup(sessionID string) string {
	if len(r.hashKeys) == 0 {
		return ""
	}
	h := Hash(sessionID)
	idx := sort.Search(len(r.hashKeys), func(i int) bool { return r.hashKeys[i] >= h })
	if idx == len(r.hashKeys) {
		idx = 0 // 环回
	}
	return r.hashRing[r.hashKeys[idx]]
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
