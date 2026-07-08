// Package redis 提供基于 Redis 的数据访问实现（在线状态等）。
package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"echat/sdk/domain/entity"
)

// OnlineRepo 用户在线状态 Redis 实现。
type OnlineRepo struct {
	Client *goredis.Client
}

func NewOnlineRepo(client *goredis.Client) *OnlineRepo {
	return &OnlineRepo{Client: client}
}

// ======================== 在线状态 ========================

// UserOnline 设置用户上线（Sadd 全局 + 群 Set，Expire 300s）。
func (r *OnlineRepo) UserOnline(ctx context.Context, info *entity.UserOnline, groupIDs []string) error {
	pipe := r.Client.Pipeline()
	pipe.SAdd(ctx, "global:online:users", info.Account)
	pipe.Expire(ctx, "global:online:users", 300*time.Second)
	for _, gid := range groupIDs {
		key := fmt.Sprintf("group:online:%s", gid)
		pipe.SAdd(ctx, key, info.Account)
		pipe.Expire(ctx, key, 300*time.Second)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// UserOffline 设置用户下线（Srem 全局 + 群 Set）。
func (r *OnlineRepo) UserOffline(ctx context.Context, account string, groupIDs []string) error {
	pipe := r.Client.Pipeline()
	pipe.SRem(ctx, "global:online:users", account)
	for _, gid := range groupIDs {
		pipe.SRem(ctx, fmt.Sprintf("group:online:%s", gid), account)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// UpdateHeartbeat 心跳刷新（Expire 全局 + 群 Set，不重新 Sadd）。
func (r *OnlineRepo) UpdateHeartbeat(ctx context.Context, groupIDs []string) error {
	pipe := r.Client.Pipeline()
	pipe.Expire(ctx, "global:online:users", 300*time.Second)
	for _, gid := range groupIDs {
		pipe.Expire(ctx, fmt.Sprintf("group:online:%s", gid), 300*time.Second)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// BatchCheckOnlineStatus 批量检查在线状态（Pipeline SIsMember）。
func (r *OnlineRepo) BatchCheckOnlineStatus(ctx context.Context, accounts []string) ([]string, error) {
	if len(accounts) == 0 {
		return nil, nil
	}
	pipe := r.Client.Pipeline()
	cmds := make([]*goredis.BoolCmd, len(accounts))
	for i, acc := range accounts {
		cmds[i] = pipe.SIsMember(ctx, "global:online:users", acc)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}
	var online []string
	for i, cmd := range cmds {
		if cmd.Val() {
			online = append(online, accounts[i])
		}
	}
	return online, nil
}

// GetGroupOnlineMembers 获取群在线成员列表（SMembers）。
func (r *OnlineRepo) GetGroupOnlineMembers(ctx context.Context, gid string) ([]string, error) {
	return r.Client.SMembers(ctx, fmt.Sprintf("group:online:%s", gid)).Result()
}
