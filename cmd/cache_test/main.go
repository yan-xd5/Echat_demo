// 缓存功能快速验证
// 用法: go run ./cmd/cache_test/

package main

import (
	"context"
	"fmt"

	"echat/sdk/domain/entity"
	sdkredis "echat/sdk/repository/redis"

	redis "github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()
	cli := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer cli.Close()

	// Ping
	if err := cli.Ping(ctx).Err(); err != nil {
		fmt.Printf("❌ Redis 连接失败: %v\n", err)
		return
	}
	fmt.Println("✅ Redis 连接成功")

	cache := sdkredis.NewCacheRepo(cli)

	// 测试 1: SetUser + GetUser
	fmt.Println("\n── 1. 用户缓存 ──")
	u := &entity.User{UID: "cache_test_1", Username: "CacheTest", Account: "ct", Bio: entity.Ptr("hello")}
	cache.SetUser(ctx, "cache_test_1", u)
	got, ok := cache.GetUser(ctx, "cache_test_1")
	if !ok || got.Username != "CacheTest" {
		fmt.Printf("❌ GetUser 失败: ok=%v, got=%+v\n", ok, got)
		return
	}
	fmt.Printf("✅ SetUser → GetUser: %s (bio=%s)\n", got.Username, *got.Bio)

	// 测试 2: DeleteUser
	cache.DeleteUser(ctx, "cache_test_1")
	_, ok = cache.GetUser(ctx, "cache_test_1")
	if ok {
		fmt.Println("❌ DeleteUser 未生效")
		return
	}
	fmt.Println("✅ DeleteUser → GetUser miss")

	// 测试 3: SetFriends + GetFriends
	fmt.Println("\n── 2. 好友缓存 ──")
	friends := []*entity.Friends{
		{FID: "f1", UID: "u1", ToUID: "u2"},
		{FID: "f2", UID: "u1", ToUID: "u3"},
	}
	cache.SetFriends(ctx, "u1", friends)
	gotF, ok := cache.GetFriends(ctx, "u1")
	if !ok || len(gotF) != 2 {
		fmt.Printf("❌ GetFriends 失败: ok=%v, len=%d\n", ok, len(gotF))
		return
	}
	fmt.Printf("✅ SetFriends → GetFriends: %d 个好友\n", len(gotF))

	// 测试 4: DeleteFriends
	cache.DeleteFriends(ctx, "u1")
	_, ok = cache.GetFriends(ctx, "u1")
	if ok {
		fmt.Println("❌ DeleteFriends 未生效")
		return
	}
	fmt.Println("✅ DeleteFriends → GetFriends miss")

	// 测试 5: 群组缓存
	fmt.Println("\n── 3. 群组缓存 ──")
	g := &entity.GroupChat{GID: "g1", GroupName: "TestGroup", ManagerUID: "u1"}
	cache.SetGroup(ctx, "g1", g)
	gotG, ok := cache.GetGroup(ctx, "g1")
	if !ok || gotG.GroupName != "TestGroup" {
		fmt.Printf("❌ GetGroup 失败: ok=%v\n", ok)
		return
	}
	fmt.Println("✅ SetGroup → GetGroup:", gotG.GroupName)

	// 测试 6: 群成员缓存
	members := []*entity.GroupMember{
		{UID: "u1", GID: "g1", Role: entity.RoleOwner},
		{UID: "u2", GID: "g1", Role: entity.RoleMember},
	}
	cache.SetGroupMembers(ctx, "g1", members)
	gotM, ok := cache.GetGroupMembers(ctx, "g1")
	if !ok || len(gotM) != 2 {
		fmt.Printf("❌ GetGroupMembers 失败: ok=%v, len=%d\n", ok, len(gotM))
		return
	}
	fmt.Printf("✅ SetGroupMembers → GetGroupMembers: %d 个成员\n", len(gotM))

	// 测试 7: BatchGetUsers
	fmt.Println("\n── 4. BatchGetUsers ──")
	cache.SetUser(ctx, "u1", &entity.User{UID: "u1", Username: "User1"})
	cache.SetUser(ctx, "u2", &entity.User{UID: "u2", Username: "User2"})
	// "u3" is not cached, will call loader
	users, err := cache.BatchGetUsers(ctx, []string{"u1", "u2", "u3"}, func(ctx context.Context, missed []string) (map[string]*entity.User, error) {
		fmt.Printf("  loader called with missed: %v\n", missed)
		return map[string]*entity.User{"u3": {UID: "u3", Username: "User3"}}, nil
	})
	if err != nil || len(users) != 3 {
		fmt.Printf("❌ BatchGetUsers 失败: err=%v, len=%d\n", err, len(users))
		return
	}
	fmt.Printf("✅ BatchGetUsers: %d 个用户 (2 cache + 1 DB)\n", len(users))

	// 清理
	cli.Del(ctx, "cache:user:u1", "cache:user:u2", "cache:user:u3")

	fmt.Println("\n🎉 全部测试通过！")
}
