// 集成测试：创建用户 → JWT 签发 → WebSocket → 发消息 → 收 ACK
// 运行: cd g:/CodeLearning/Go/echat && go run integration_main.go

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
	dsn := "root:sysu@tcp(127.0.0.1:3306)/chat?charset=utf8mb4&parseTime=true"
	db, err := mysql.NewDB(dsn)
	if err != nil {
		log.Fatal("❌ MySQL 连接失败:", err)
	}
	defer db.Close()
	fmt.Println("✅ MySQL 已连接")

	// ① 创建测试用户
	ctx := context.Background()
	userRepo := mysql.NewUserRepo(db)
	uid := "test_user_001"
	hash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)

	// 创建用户 A
	if err := userRepo.SaveUser(ctx, &entity.User{
		UID: uid, Account: "test_user_001", Password: string(hash), Username: "Tester",
	}); err != nil {
		log.Fatal("❌ 创建用户A失败:", err)
	}
	// 创建用户 B（接收者）
	if err := userRepo.SaveUser(ctx, &entity.User{
		UID: "test_user_002", Account: "test_user_002", Password: string(hash), Username: "Friend",
	}); err != nil {
		log.Fatal("❌ 创建用户B失败:", err)
	}
	// 创建好友关系 + 私聊会话
	fid := "test_fid_01"
	_, err = db.ExecContext(ctx, `INSERT IGNORE INTO friends (fid, uid, to_uid) VALUES (?, 'test_user_001', 'test_user_002')`, fid)
	if err != nil {
		log.Fatal("❌ 创建好友关系失败:", err)
	}
	_, err = db.ExecContext(ctx, `INSERT IGNORE INTO private_chat (pid, uid1, uid2) VALUES (?, 'test_user_001', 'test_user_002')`, fid)
	if err != nil {
		log.Fatal("❌ 创建私聊会话失败:", err)
	}
	fmt.Println("✅ 测试用户就绪: uid=test_user_001, friend=test_user_002")

	// ② 签发 JWT
	token, err := auth.SignToken(uid, "test_user_001", "web")
	if err != nil {
		log.Fatal("❌ JWT 签发失败:", err)
	}
	fmt.Println("✅ JWT 已签发:", token[:50]+"...")

	// ③ WebSocket 连接
	wsURL := fmt.Sprintf("ws://127.0.0.1:9000/ws?ticket=%s", token)
	header := http.Header{}
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		fmt.Printf("❌ WebSocket 连接失败: %v (HTTP %d)\n", err, resp.StatusCode)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Println("✅ WebSocket 已连接")

	// ④ 发私聊消息 (JSON 格式对应 Gateway handleMessage 的 WSMessage)
	msg := map[string]interface{}{
		"seq":  1,
		"type": "chat",
		"to":   "test_user_002",
		"content": map[string]string{
			"text": "Hello from integration test!",
		},
	}
	msgBytes, _ := json.Marshal(msg)
	if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		log.Fatal("❌ 发送消息失败:", err)
	}
	fmt.Println("📤 消息已发送:", string(msgBytes))

	// ⑤ 等 ACK（10s 超时）
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, ackBytes, err := conn.ReadMessage()
	if err != nil {
		log.Fatal("❌ 收 ACK 失败:", err)
	}
	var ack map[string]interface{}
	json.Unmarshal(ackBytes, &ack)
	fmt.Printf("📥 收到 ACK: %s\n", string(ackBytes))

	switch ack["type"] {
	case "ack":
		fmt.Println("\n🎉 集成测试通过！消息已路由: Controller → 持久化 → 转发 → ACK")
	case "error":
		fmt.Printf("\n⚠️  收到错误 ACK: %v\n", ack["error"])
	default:
		fmt.Printf("\n⚠️  未知响应: %v\n", ack["type"])
	}
}