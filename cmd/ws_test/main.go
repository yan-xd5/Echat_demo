// WebSocket 连接测试脚本
// 用法: go run ./cmd/ws_test/
// 自动: 注册两个用户 → 建立好友关系 → 签发 JWT → WebSocket 连接 → 发送消息 → 收 ACK

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"

	"echat/sdk/auth"
	"echat/sdk/entity"
	"echat/sdk/mysql"
)

func main() {
	dsn := envOr("MYSQL_DSN", "root:@tcp(127.0.0.1:3306)/chat?charset=utf8mb4&parseTime=true")
	db, err := mysql.NewDB(dsn)
	if err != nil {
		log.Fatal("❌ MySQL 连接失败:", err)
	}
	defer db.Close()

	ctx := context.Background()
	userRepo := mysql.NewUserRepo(db)
	hash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)

	// ========================
	// ① 创建测试用户
	// ========================
	uidA := "ws_test_alice"
	uidB := "ws_test_bob"

	for _, u := range []struct{ uid, account, name string }{
		{uidA, "ws_test_alice", "Alice"},
		{uidB, "ws_test_bob", "Bob"},
	} {
		userRepo.SaveUser(ctx, &entity.User{
			UID: u.uid, Account: u.account, Password: string(hash), Username: u.name,
		})
	}
	fmt.Println("✅ 测试用户就绪: alice, bob")

	// ========================
	// ② 建立好友关系 + 私聊
	// ========================
	fid := "ws_fid_alice_bob"
	db.ExecContext(ctx, `INSERT IGNORE INTO friends (fid, uid, to_uid) VALUES (?, ?, ?)`,
		fid, uidA, uidB)
	db.ExecContext(ctx, `INSERT IGNORE INTO private_chat (pid, uid1, uid2) VALUES (?, ?, ?)`,
		fid, uidA, uidB)
	fmt.Println("✅ 好友关系 + 私聊已建立")

	// ========================
	// ③ 签发 JWT
	// ========================
	token, err := auth.SignToken(uidA, "ws_test_alice", "web")
	if err != nil {
		log.Fatal("❌ JWT 签发失败:", err)
	}
	fmt.Println("✅ JWT 已签发")

	// ========================
	// ④ WebSocket 连接
	// ========================
	wsURL := fmt.Sprintf("ws://127.0.0.1:9000/ws?ticket=%s", token)
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{})
	if err != nil {
		fmt.Printf("❌ WebSocket 连接失败: %v (HTTP %d)\n", err, resp.StatusCode)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Println("✅ WebSocket 已连接")

	// ========================
	// ⑤ 发送消息
	// ========================
	msg := map[string]interface{}{
		"seq":     1,
		"type":    "chat",
		"to":      uidB,
		"content": map[string]string{"text": fmt.Sprintf("Hello! 测试消息 %s", time.Now().Format("15:04:05"))},
	}
	msgBytes, _ := json.Marshal(msg)
	if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		log.Fatal("❌ 发送失败:", err)
	}
	fmt.Println("📤 已发送:", string(msgBytes))

	// ========================
	// ⑥ 收 ACK
	// ========================
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, ackBytes, err := conn.ReadMessage()
	if err != nil {
		log.Fatal("❌ 收 ACK 超时:", err)
	}
	fmt.Println("📥 收到响应:", string(ackBytes))

	var ack map[string]interface{}
	json.Unmarshal(ackBytes, &ack)
	switch ack["type"] {
	case "ack":
		fmt.Println("\n🎉 测试通过！全链路: WS → Gateway → Controller → MySQL → ACK")
	case "error":
		fmt.Printf("\n⚠️  ACK 错误: %v\n", ack["error"])
	default:
		fmt.Printf("\n⚠️  未知响应: %v\n", ack["type"])
	}
}

func envOr(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
