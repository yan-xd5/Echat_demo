package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// SessionInfo Redis 中存储的会话信息。
type SessionInfo struct {
	GatewayID  string `json:"gateway_id"`
	Platform   string `json:"platform"`
	ConnTime   int64  `json:"conn_time"`
	LastActive int64  `json:"last_active"`
}

// SessionRedis 管理 Redis Hash 中的会话路由信息。
type SessionRedis struct {
	client *redis.Client
}

// NewSessionRedis 创建 Redis 会话管理器。
func NewSessionRedis(client *redis.Client) *SessionRedis {
	return &SessionRedis{client: client}
}

func sessionKey(uid string) string { return fmt.Sprintf("user_sessions:%s", uid) }

// AddSession 连接建立时写 Redis Hash。
func (r *SessionRedis) AddSession(ctx context.Context, uid, deviceID string, info *SessionInfo) error {
	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal SessionInfo: %w", err)
	}
	if err := r.client.HSet(ctx, sessionKey(uid), deviceID, data).Err(); err != nil {
		return fmt.Errorf("HSET session: %w", err)
	}
	r.client.Expire(ctx, sessionKey(uid), 300*time.Second)
	return nil
}

// RemoveSession 连接断开时从 Redis Hash 中删除。
func (r *SessionRedis) RemoveSession(ctx context.Context, uid, deviceID string) error {
	return r.client.HDel(ctx, sessionKey(uid), deviceID).Err()
}

// GetUserDevices 查询用户所有在线设备及所在 Gateway。
func (r *SessionRedis) GetUserDevices(ctx context.Context, uid string) (map[string]*SessionInfo, error) {
	result, err := r.client.HGetAll(ctx, sessionKey(uid)).Result()
	if err != nil {
		return nil, err
	}
	devices := make(map[string]*SessionInfo, len(result))
	for deviceID, raw := range result {
		var info SessionInfo
		if json.Unmarshal([]byte(raw), &info) == nil {
			devices[deviceID] = &info
		}
	}
	return devices, nil
}

// RefreshTTL 心跳续期。
func (r *SessionRedis) RefreshTTL(ctx context.Context, uid string) error {
	return r.client.Expire(ctx, sessionKey(uid), 300*time.Second).Err()
}
