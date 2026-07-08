// 压测脚本：多用户并发消息收发
// 用法: go run ./cmd/stress_test/
// 环境变量: PAIRS=10 MSGS=50 (默认 10 对用户，每对 50 条消息)

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"

	"echat/sdk/usecase/auth"
	"echat/sdk/domain/entity"
	"echat/sdk/repository/mysql"
)

type stats struct {
	sent     atomic.Int64
	acked    atomic.Int64
	failed   atomic.Int64
	totalLat atomic.Int64 // 微秒累计
	errs     atomic.Int64
}

func (s *stats) addLatency(d time.Duration) {
	s.totalLat.Add(d.Microseconds())
}

func (s *stats) report(elapsed time.Duration) {
	sent := s.sent.Load()
	acked := s.acked.Load()
	failed := s.failed.Load()
	errs := s.errs.Load()
	totalLat := s.totalLat.Load()

	fmt.Println("\n==================== 压测报告 ====================")
	fmt.Printf("耗时:          %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("发送消息数:    %d\n", sent)
	fmt.Printf("收到 ACK:      %d\n", acked)
	fmt.Printf("ACK 错误:     %d\n", failed)
	fmt.Printf("连接/发送失败: %d\n", errs)
	if acked > 0 {
		fmt.Printf("平均延迟:      %v\n", time.Duration(totalLat/acked)*time.Microsecond)
	}
	if sent > 0 {
		fmt.Printf("吞吐量:        %.0f msg/s\n", float64(sent)/elapsed.Seconds())
		fmt.Printf("成功率:        %.1f%%\n", float64(acked)/float64(sent)*100)
	}
	fmt.Println("==================================================")
}

func main() {
	pairs := envInt("PAIRS", 10)
	msgsPerPair := envInt("MSGS", 50)

	dsn := envOr("MYSQL_DSN", "root:@tcp(127.0.0.1:3306)/chat?charset=utf8mb4&parseTime=true")
	db, err := mysql.NewDB(dsn)
	if err != nil {
		log.Fatal("❌ MySQL:", err)
	}
	defer db.Close()

	ctx := context.Background()
	userRepo := mysql.NewUserRepo(db)
	hash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)

	fmt.Printf("⚙️  %d 对用户 × %d 条消息 = %d 总消息\n", pairs, msgsPerPair, pairs*msgsPerPair)
	fmt.Println("⏳ 创建用户和好友关系...")

	// ① 批量创建用户和好友关系
	type pair struct {
		aUID, bUID, fid string
		token           string
	}
	pairsList := make([]pair, pairs)

	for i := 0; i < pairs; i++ {
		uidA := fmt.Sprintf("stress_a_%d", i)
		uidB := fmt.Sprintf("stress_b_%d", i)
		fid := fmt.Sprintf("stress_f_%d", i)

		userRepo.SaveUser(ctx, &entity.User{
			UID: uidA, Account: uidA, Password: string(hash), Username: fmt.Sprintf("UserA_%d", i),
		})
		userRepo.SaveUser(ctx, &entity.User{
			UID: uidB, Account: uidB, Password: string(hash), Username: fmt.Sprintf("UserB_%d", i),
		})

		db.ExecContext(ctx, `INSERT IGNORE INTO friends (fid, uid, to_uid) VALUES (?, ?, ?)`, fid, uidA, uidB)
		db.ExecContext(ctx, `INSERT IGNORE INTO private_chat (pid, uid1, uid2) VALUES (?, ?, ?)`, fid, uidA, uidB)

		token, _ := auth.SignToken(uidA, uidA, "web")
		pairsList[i] = pair{uidA, uidB, fid, token}
	}
	fmt.Printf("✅ %d 对用户就绪\n", pairs)

	// ② 并发连接 + 收发消息
	fmt.Println("⏳ 开始压测...")
	st := &stats{}
	startTime := time.Now()

	var wg sync.WaitGroup
	sem := make(chan struct{}, 50) // 最多 50 并发连接

	for i, p := range pairsList {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, p pair) {
			defer wg.Done()
			defer func() { <-sem }()

			// WebSocket 连接
			wsURL := fmt.Sprintf("ws://127.0.0.1:9000/ws?ticket=%s", p.token)
			conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{})
			if err != nil {
				st.errs.Add(1)
				return
			}
			defer conn.Close()

			// 另开协程收消息（含 ACK 和可能的 push）
			done := make(chan struct{})
			go func() {
				defer close(done)
				for {
					conn.SetReadDeadline(time.Now().Add(30 * time.Second))
					_, data, err := conn.ReadMessage()
					if err != nil {
						return
					}
					var resp map[string]interface{}
					json.Unmarshal(data, &resp)
					if resp["type"] == "ack" {
						st.acked.Add(1)
					}
				}
			}()

			// 发送消息
			for seq := int64(1); seq <= int64(msgsPerPair); seq++ {
				msg := map[string]interface{}{
					"seq":  seq,
					"type": "chat",
					"to":   p.bUID,
					"content": map[string]string{
						"text": fmt.Sprintf("stress_%d_%d_%d", idx, seq, time.Now().UnixNano()),
					},
				}
				msgBytes, _ := json.Marshal(msg)

				t0 := time.Now()
				if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
					st.errs.Add(1)
					return
				}
				st.sent.Add(1)
				st.addLatency(time.Since(t0))

				// 轻微间隔，避免压爆
				if seq%10 == 0 {
					time.Sleep(5 * time.Millisecond)
				}
			}

			// 等所有 ACK 返回（最多 15s）
			select {
			case <-done:
			case <-time.After(15 * time.Second):
			}
		}(i, p)

		// 错开连接建立
		time.Sleep(10 * time.Millisecond)
	}

	wg.Wait()
	elapsed := time.Since(startTime)
	st.report(elapsed)
}

func envOr(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

func envInt(k string, fallback int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
