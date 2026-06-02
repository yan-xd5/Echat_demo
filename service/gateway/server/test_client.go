//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	// 连接 Gateway WebSocket
	conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:9000/ws", nil)
	if err != nil {
		fmt.Println("❌ 连接失败:", err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Println("✅ 已连接到 Gateway")

	// ① 发送认证
	send(conn, map[string]string{"type": "auth", "token": "eyJhbGciOiJIUzI1NiJ9.xxx"})
	recv(conn)

	// ② 发送单聊消息（user_001 → user_002）
	send(conn, map[string]string{
		"type": "chat", "to": "user_002", "content": "你好李四！我是张三",
	})
	recv(conn)

	// ③ 发送心跳
	send(conn, map[string]string{"type": "ping"})
	recv(conn)

	time.Sleep(500 * time.Millisecond)
	fmt.Println("✅ 测试完成")
}

func send(conn *websocket.Conn, msg map[string]string) {
	data, _ := json.Marshal(msg)
	fmt.Printf("📤 发送: %s\n", data)
	conn.WriteMessage(websocket.TextMessage, data)
}

func recv(conn *websocket.Conn) {
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		fmt.Printf("❌ 接收失败: %v\n", err)
		return
	}
	fmt.Printf("📥 收到: %s\n", data)
}
