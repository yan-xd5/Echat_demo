// Package redis 提供基于 Redis 的数据访问实现（缓存层）。
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"echat/sdk/domain/entity"

	"trpc.group/trpc-go/trpc-go/log"
)

// ======================== 常量 ========================

const (
	// DefaultCacheTTL 默认缓存过期时间（5 分钟）。
	DefaultCacheTTL = 5 * time.Minute
)

// ======================== CacheRepo ========================

// CacheRepo MySQL 查询结果的 Redis 缓存仓库。
// 所有操作采用 best-effort 语义：失败打 warn 日志，不回传 error。
type CacheRepo struct {
	Client *goredis.Client
	TTL    time.Duration
}

// NewCacheRepo 创建缓存仓库。
func NewCacheRepo(client *goredis.Client) *CacheRepo {
	return &CacheRepo{Client: client, TTL: DefaultCacheTTL}
}

// ======================== Key 生成 ========================

func userKey(uid string) string      { return fmt.Sprintf("cache:user:%s", uid) }
func friendsKey(uid string) string   { return fmt.Sprintf("cache:friends:%s", uid) }
func groupKey(gid string) string     { return fmt.Sprintf("cache:group:%s", gid) }
func groupMembersKey(gid string) string { return fmt.Sprintf("cache:group_members:%s", gid) }
func userGroupsKey(uid string) string { return fmt.Sprintf("cache:user_groups:%s", uid) }

// ======================== 用户缓存 ========================

// GetUser 从缓存获取用户信息。
func (c *CacheRepo) GetUser(ctx context.Context, uid string) (*entity.User, bool) {
	data, err := c.Client.Get(ctx, userKey(uid)).Bytes()
	if err != nil {
		return nil, false
	}
	var u entity.User
	if err := json.Unmarshal(data, &u); err != nil {
		log.WarnContextf(ctx, "[cache] 反序列化 user:%s 失败: %v", uid, err)
		return nil, false
	}
	return &u, true
}

// SetUser 缓存用户信息。
func (c *CacheRepo) SetUser(ctx context.Context, uid string, u *entity.User) {
	if u == nil {
		return
	}
	data, err := json.Marshal(u)
	if err != nil {
		log.WarnContextf(ctx, "[cache] 序列化 user:%s 失败: %v", uid, err)
		return
	}
	if err := c.Client.Set(ctx, userKey(uid), data, c.TTL).Err(); err != nil {
		log.WarnContextf(ctx, "[cache] 写 user:%s 失败: %v", uid, err)
	}
}

// DeleteUser 删除用户缓存。
func (c *CacheRepo) DeleteUser(ctx context.Context, uid string) {
	if err := c.Client.Del(ctx, userKey(uid)).Err(); err != nil {
		log.WarnContextf(ctx, "[cache] 删 user:%s 失败: %v", uid, err)
	}
}

// BatchGetUsers 批量获取用户缓存。
// miss 的 UID 通过 loader 查 MySQL，写回 Redis 后合并返回。
func (c *CacheRepo) BatchGetUsers(ctx context.Context, uids []string, loader func(ctx context.Context, missed []string) (map[string]*entity.User, error)) (map[string]*entity.User, error) {
	if len(uids) == 0 {
		return nil, nil
	}

	// 查 Redis
	pipe := c.Client.Pipeline()
	cmds := make([]*goredis.StringCmd, len(uids))
	for i, uid := range uids {
		cmds[i] = pipe.Get(ctx, userKey(uid))
	}
	_, _ = pipe.Exec(ctx) // best-effort

	hits := make(map[string]*entity.User, len(uids))
	var missed []string
	for i, cmd := range cmds {
		data, err := cmd.Bytes()
		if err != nil {
			missed = append(missed, uids[i])
			continue
		}
		var u entity.User
		if err := json.Unmarshal(data, &u); err != nil {
			missed = append(missed, uids[i])
			continue
		}
		hits[uids[i]] = &u
	}

	// 查 MySQL（miss 的部分）
	if len(missed) > 0 {
		dbUsers, err := loader(ctx, missed)
		if err != nil {
			return hits, err // 返回已有的缓存 + 错误
		}
		// 写回 Redis
		for _, uid := range missed {
			if u, ok := dbUsers[uid]; ok {
				hits[uid] = u
				c.SetUser(ctx, uid, u)
			}
		}
	}

	return hits, nil
}

// ======================== 好友列表缓存 ========================

// GetFriends 从缓存获取好友列表。
func (c *CacheRepo) GetFriends(ctx context.Context, uid string) ([]*entity.Friends, bool) {
	data, err := c.Client.Get(ctx, friendsKey(uid)).Bytes()
	if err != nil {
		return nil, false
	}
	var list []*entity.Friends
	if err := json.Unmarshal(data, &list); err != nil {
		log.WarnContextf(ctx, "[cache] 反序列化 friends:%s 失败: %v", uid, err)
		return nil, false
	}
	return list, true
}

// SetFriends 缓存好友列表。
func (c *CacheRepo) SetFriends(ctx context.Context, uid string, list []*entity.Friends) {
	if list == nil {
		return
	}
	data, err := json.Marshal(list)
	if err != nil {
		log.WarnContextf(ctx, "[cache] 序列化 friends:%s 失败: %v", uid, err)
		return
	}
	if err := c.Client.Set(ctx, friendsKey(uid), data, c.TTL).Err(); err != nil {
		log.WarnContextf(ctx, "[cache] 写 friends:%s 失败: %v", uid, err)
	}
}

// DeleteFriends 删除好友列表缓存。
func (c *CacheRepo) DeleteFriends(ctx context.Context, uid string) {
	if err := c.Client.Del(ctx, friendsKey(uid)).Err(); err != nil {
		log.WarnContextf(ctx, "[cache] 删 friends:%s 失败: %v", uid, err)
	}
}

// ======================== 群信息缓存 ========================

// GetGroup 从缓存获取群信息。
func (c *CacheRepo) GetGroup(ctx context.Context, gid string) (*entity.GroupChat, bool) {
	data, err := c.Client.Get(ctx, groupKey(gid)).Bytes()
	if err != nil {
		return nil, false
	}
	var g entity.GroupChat
	if err := json.Unmarshal(data, &g); err != nil {
		log.WarnContextf(ctx, "[cache] 反序列化 group:%s 失败: %v", gid, err)
		return nil, false
	}
	return &g, true
}

// SetGroup 缓存群信息。
func (c *CacheRepo) SetGroup(ctx context.Context, gid string, g *entity.GroupChat) {
	if g == nil {
		return
	}
	data, err := json.Marshal(g)
	if err != nil {
		log.WarnContextf(ctx, "[cache] 序列化 group:%s 失败: %v", gid, err)
		return
	}
	if err := c.Client.Set(ctx, groupKey(gid), data, c.TTL).Err(); err != nil {
		log.WarnContextf(ctx, "[cache] 写 group:%s 失败: %v", gid, err)
	}
}

// DeleteGroup 删除群信息缓存。
func (c *CacheRepo) DeleteGroup(ctx context.Context, gid string) {
	if err := c.Client.Del(ctx, groupKey(gid)).Err(); err != nil {
		log.WarnContextf(ctx, "[cache] 删 group:%s 失败: %v", gid, err)
	}
}

// ======================== 群成员缓存 ========================

// GetGroupMembers 从缓存获取群成员列表。
func (c *CacheRepo) GetGroupMembers(ctx context.Context, gid string) ([]*entity.GroupMember, bool) {
	data, err := c.Client.Get(ctx, groupMembersKey(gid)).Bytes()
	if err != nil {
		return nil, false
	}
	var list []*entity.GroupMember
	if err := json.Unmarshal(data, &list); err != nil {
		log.WarnContextf(ctx, "[cache] 反序列化 group_members:%s 失败: %v", gid, err)
		return nil, false
	}
	return list, true
}

// SetGroupMembers 缓存群成员列表。
func (c *CacheRepo) SetGroupMembers(ctx context.Context, gid string, list []*entity.GroupMember) {
	if list == nil {
		return
	}
	data, err := json.Marshal(list)
	if err != nil {
		log.WarnContextf(ctx, "[cache] 序列化 group_members:%s 失败: %v", gid, err)
		return
	}
	if err := c.Client.Set(ctx, groupMembersKey(gid), data, c.TTL).Err(); err != nil {
		log.WarnContextf(ctx, "[cache] 写 group_members:%s 失败: %v", gid, err)
	}
}

// DeleteGroupMembers 删除群成员缓存。
func (c *CacheRepo) DeleteGroupMembers(ctx context.Context, gid string) {
	if err := c.Client.Del(ctx, groupMembersKey(gid)).Err(); err != nil {
		log.WarnContextf(ctx, "[cache] 删 group_members:%s 失败: %v", gid, err)
	}
}

// ======================== 用户群列表缓存 ========================

// GetUserGroups 从缓存获取用户加入的群列表。
func (c *CacheRepo) GetUserGroups(ctx context.Context, uid string) ([]*entity.GroupMember, bool) {
	data, err := c.Client.Get(ctx, userGroupsKey(uid)).Bytes()
	if err != nil {
		return nil, false
	}
	var list []*entity.GroupMember
	if err := json.Unmarshal(data, &list); err != nil {
		log.WarnContextf(ctx, "[cache] 反序列化 user_groups:%s 失败: %v", uid, err)
		return nil, false
	}
	return list, true
}

// SetUserGroups 缓存用户群列表。
func (c *CacheRepo) SetUserGroups(ctx context.Context, uid string, list []*entity.GroupMember) {
	if list == nil {
		return
	}
	data, err := json.Marshal(list)
	if err != nil {
		log.WarnContextf(ctx, "[cache] 序列化 user_groups:%s 失败: %v", uid, err)
		return
	}
	if err := c.Client.Set(ctx, userGroupsKey(uid), data, c.TTL).Err(); err != nil {
		log.WarnContextf(ctx, "[cache] 写 user_groups:%s 失败: %v", uid, err)
	}
}

// DeleteUserGroups 删除用户群列表缓存。
func (c *CacheRepo) DeleteUserGroups(ctx context.Context, uid string) {
	if err := c.Client.Del(ctx, userGroupsKey(uid)).Err(); err != nil {
		log.WarnContextf(ctx, "[cache] 删 user_groups:%s 失败: %v", uid, err)
	}
}
