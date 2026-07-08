// 长时间稳定性测试：两个 WebSocket 保持连接，每分钟交替发送私聊+群聊消息。
// 用法: go run ./cmd/long_test/
// 环境变量: ROUNDS=10 MINUTES=1 (默认 10 轮，每轮间隔 60s)
//
// 测试目标:
//   1. 长连接稳定性
//   2. 私聊 + 群聊全链路（含 Push，两个用户均在线）
//   3. 观测数据生成（chat_type 标签区分私聊/群聊）
//   4. SessionRouter 背景刷新（运行超 30s）
//   5. seq_id 递增 + 消息去重

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"

	"echat/sdk/usecase/auth"
	"echat/sdk/domain/entity"
	"echat/sdk/repository/mysql"
)

type wStats struct {
	sentPrivate atomic.Int64
	sentGroup   atomic.Int64
	acked       atomic.Int64
	pushed      atomic.Int64
	errors      atomic.Int64
	rounds      atomic.Int64
}
type aStats struct {
	ok   atomic.Int64
	fail atomic.Int64
}

func (s *wStats) totalSent() int64 { return s.sentPrivate.Load() + s.sentGroup.Load() }

const apiBase = "http://127.0.0.1:9001/api/v1"

func main() {
	rounds := envInt("ROUNDS", 10)
	interval := time.Duration(envInt("MINUTES", 1)) * time.Minute
	// 快速测试模式（MINUTES=0）用 1 秒间隔
	if interval == 0 {
		interval = time.Second
	}
	dsn := envOr("MYSQL_DSN", "root:@tcp(127.0.0.1:3306)/chat?charset=utf8mb4&parseTime=true")
	db, err := mysql.NewDB(dsn)
	if err != nil {
		log.Fatal("MySQL:", err)
	}
	defer db.Close()

	ctx := context.Background()
	userRepo := mysql.NewUserRepo(db)
	hash, _ := bcrypt.GenerateFromPassword([]byte("test123456"), bcrypt.DefaultCost)

	// ─── 创建用户 + 好友关系 + 私聊 ───
	uidA := "long_test_alice"
	uidB := "long_test_bob"
	fid := "long_test_fid"
	for _, u := range []struct{ uid, account, name string }{
		{uidA, uidA, "Alice"}, {uidB, uidB, "Bob"},
	} {
		userRepo.SaveUser(ctx, &entity.User{UID: u.uid, Account: u.account, Password: string(hash), Username: u.name})
	}
	db.ExecContext(ctx, `INSERT IGNORE INTO friends (fid, uid, to_uid) VALUES (?, ?, ?)`, fid, uidA, uidB)
	db.ExecContext(ctx, `INSERT IGNORE INTO private_chat (pid, uid1, uid2) VALUES (?, ?, ?)`, fid, uidA, uidB)
	fmt.Println("✅ 用户 + 好友 + 私聊就绪")

	// ─── 创建群组 ───
	gid := "long_test_group"
	db.ExecContext(ctx, `INSERT IGNORE INTO group_chat (gid, group_name, manager_uid) VALUES (?, ?, ?)`,
		gid, "LongTestGroup", uidA)
	db.ExecContext(ctx, `INSERT IGNORE INTO group_member (uid, gid, role) VALUES (?, ?, 'member')`, uidA, gid)
	db.ExecContext(ctx, `INSERT IGNORE INTO group_member (uid, gid, role) VALUES (?, ?, 'member')`, uidB, gid)
	fmt.Println("✅ 群组 + 成员就绪 (gid=" + gid + ")")

	// ─── JWT ───
	tokenA, _ := auth.SignToken(uidA, uidA, "web")
	tokenB, _ := auth.SignToken(uidB, uidB, "web")

	// ─── WebSocket ───
	connA := dialWS(tokenA, "Alice")
	defer connA.Close()
	connB := dialWS(tokenB, "Bob")
	defer connB.Close()
	fmt.Println("✅ WebSocket 双连接建立")

	wst := &wStats{}
	ast := &aStats{}
	var wg sync.WaitGroup

	// Alice 收 ACK
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			connA.SetReadDeadline(time.Now().Add(2 * time.Minute))
			_, data, err := connA.ReadMessage()
			if err != nil {
				return
			}
			var resp map[string]interface{}
			if json.Unmarshal(data, &resp) == nil && resp["type"] == "ack" {
				wst.acked.Add(1)
				fmt.Printf("✔️  [ACK] msg_id=%v\n", resp["msg_id"])
			}
		}
	}()

	// Bob 收 Push
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			connB.SetReadDeadline(time.Now().Add(2 * time.Minute))
			_, data, err := connB.ReadMessage()
			if err != nil {
				return
			}
			var resp map[string]interface{}
			if json.Unmarshal(data, &resp) == nil && resp["type"] == "push" {
				wst.pushed.Add(1)
				chatType := "private"
				if resp["group_id"] != nil && resp["group_id"] != "" {
					chatType = "group"
				}
				fmt.Printf("📬 [Push] %s content=%v\n", chatType, resp["content"])
			}
		}
	}()

	// ─── API 验证阶段 ───
	fmt.Println()
	testAllAPIs(tokenA, uidB, gid, ast)

	// ─── 主循环：WS 消息 + API 调用 ───
	fmt.Printf("\n⏳ 测试开始: %d 轮, 间隔 %v\n", rounds, interval)
	fmt.Println(strings.Repeat("─", 50))

	startTime := time.Now()
	for round := 1; round <= rounds; round++ {
		<-time.After(interval)
		wst.rounds.Add(1)

		// WS 消息：交替私聊/群聊
		var msg map[string]interface{}
		var msgType string
		if round%2 == 1 {
			msgType = "private"
			msg = map[string]interface{}{"seq": round, "type": "chat", "to": uidB,
				"content": map[string]string{"text": fmt.Sprintf("[私聊 R%d] %s", round, time.Now().Format("15:04:05"))}}
			wst.sentPrivate.Add(1)
		} else {
			msgType = "group"
			msg = map[string]interface{}{"seq": round, "type": "group_chat", "group_id": gid,
				"content": map[string]string{"text": fmt.Sprintf("[群聊 R%d] %s", round, time.Now().Format("15:04:05"))}}
			wst.sentGroup.Add(1)
		}
		msgBytes, _ := json.Marshal(msg)
		if err := connA.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
			wst.errors.Add(1)
			log.Printf("WS 发送失败 (round %d): %v", round, err)
			continue
		}

		// API 调用：每轮轮换一个端点
		testAPIQuick(round, tokenA, uidB, gid, ast)
		fmt.Printf("📤 [Round %2d] WS:%s + API 已发送\n", round, msgType)
	}

	// ─── 收尾 ───
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("⏳ 等待最后一轮确认...")
	time.Sleep(3 * time.Second)

	// ─── 报告 ───
	elapsed := time.Since(startTime).Round(time.Second)
	sent := wst.totalSent()
	acked := wst.acked.Load()
	pushed := wst.pushed.Load()
	errs := wst.errors.Load()

	fmt.Println()
	fmt.Println("==================== 长时间测试报告 ====================")
	fmt.Printf("运行时长:      %v\n", elapsed)
	fmt.Printf("测试轮次:      %d\n", wst.rounds.Load())
	fmt.Printf("--- WebSocket ---\n")
	fmt.Printf("私聊: %d  群聊: %d  总发送: %d\n", wst.sentPrivate.Load(), wst.sentGroup.Load(), sent)
	fmt.Printf("ACK: %d  Push: %d  错误: %d\n", acked, pushed, errs)
	if sent > 0 {
		fmt.Printf("ACK 成功率:    %.1f%%\n", float64(acked)/float64(sent)*100)
		fmt.Printf("Push 成功率:   %.1f%%\n", float64(pushed)/float64(sent)*100)
	}
	fmt.Printf("--- API 服务 ---\n")
	fmt.Printf("成功: %d  失败: %d\n", ast.ok.Load(), ast.fail.Load())
	fmt.Println("========================================================")

	allOK := acked == sent && pushed == sent && errs == 0 && ast.fail.Load() == 0
	if allOK {
		fmt.Println("\n🎉 长时间稳定性测试通过（WS私聊+群聊 + API服务）！")
	} else {
		fmt.Println("\n⚠️  存在失败，请检查日志")
	}
}

func dialWS(token, name string) *websocket.Conn {
	wsURL := fmt.Sprintf("ws://127.0.0.1:9000/ws?ticket=%s", token)
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{})
	if err != nil {
		code := 0
		if resp != nil {
			code = resp.StatusCode
		}
		log.Fatalf("[%s] WebSocket 连接失败: %v (HTTP %d)", name, err, code)
	}
	fmt.Printf("✅ [%s] WebSocket 已连接\n", name)
	return conn
}

// ─── API 测试辅助 ───

func apiGet(token, path string) (int, string) {
	req, _ := http.NewRequest("GET", apiBase+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func apiPost(token, path string, data string) (int, string) {
	req, _ := http.NewRequest("POST", apiBase+path, bytes.NewBufferString(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func checkAPI(name string, code int, body string, ast *aStats) {
	ok := code >= 200 && code < 300 && !strings.Contains(body, `"code":999`)
	if ok {
		ast.ok.Add(1)
		fmt.Printf("  ✅ %s (HTTP %d)\n", name, code)
	} else {
		ast.fail.Add(1)
		fmt.Printf("  ❌ %s (HTTP %d) body=%s\n", name, code, truncate(body, 80))
	}
}

func testAllAPIs(token, uidB, gid string, ast *aStats) {
	fmt.Println("🔍 API 服务验证...")
	// 公开端点（无需认证）
	code, body := apiGet("", "/auth/public-key")
	checkAPI("GetPublicKey", code, body, ast)
	// 搜索用户
	code, body = apiPost(token, "/user/search", fmt.Sprintf(`{"keyword":"%s"}`, uidB))
	checkAPI("SearchUser", code, body, ast)
	// 会话列表
	code, body = apiPost(token, "/chat/conversations", "{}")
	checkAPI("GetConversations", code, body, ast)
	// 在线状态
	code, body = apiPost(token, "/chat/online-status", fmt.Sprintf(`{"uids":["%s"]}`, uidB))
	checkAPI("OnlineStatus", code, body, ast)
	// 群搜索
	code, body = apiPost(token, "/group/search", `{"keyword":"LongTest"}`)
	checkAPI("SearchGroup", code, body, ast)
	// 群公告
	code, body = apiPost(token, "/group/announces", fmt.Sprintf(`{"gid":"%s"}`, gid))
	checkAPI("GroupAnnounces", code, body, ast)
	// 批量查用户
	code, body = apiPost(token, "/user/batch", fmt.Sprintf(`{"uids":["%s"]}`, uidB))
	checkAPI("BatchGetUsers", code, body, ast)
	// 未认证拒绝（公开端点除外）
	code, body = apiPost("", "/user/search", `{"keyword":"test"}`)
	ok := code >= 400
	if ok {
		ast.ok.Add(1)
		fmt.Printf("  ✅ AuthReject (HTTP %d)\n", code)
	} else {
		ast.fail.Add(1)
		fmt.Printf("  ❌ AuthReject should block (HTTP %d)\n", code)
	}
}

func testAPIQuick(round int, token, uidB, gid string, ast *aStats) {
	endpoints := []struct {
		name   string
		method string
		path   string
		data   string
	}{
		{"SearchUser", "POST", "/user/search", fmt.Sprintf(`{"keyword":"%s"}`, uidB)},
		{"GetConversations", "POST", "/chat/conversations", "{}"},
		{"OnlineStatus", "POST", "/chat/online-status", fmt.Sprintf(`{"uids":["%s"]}`, uidB)},
		{"SearchGroup", "POST", "/group/search", `{"keyword":"LongTest"}`},
	}
	ep := endpoints[round%len(endpoints)]
	var code int
	var body string
	if ep.method == "POST" {
		code, body = apiPost(token, ep.path, ep.data)
	} else {
		code, body = apiGet(token, ep.path)
	}
	checkAPI(ep.name, code, body, ast)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
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
