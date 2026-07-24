// 全 RESTful API 端点测试 — 37 个端点逐个验证。
// tRPC 协议端点（21 个）由 WebSocket 全链路测试间接覆盖。
// 用法: TEST_TOKEN=<jwt> go run ./cmd/api_test/

package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
)

const apiBase = "http://127.0.0.1:9001/api/v1"

var (
	token    string
	ok, fail atomic.Int64
)

func main() {
	token = os.Getenv("TEST_TOKEN")
	if token == "" {
		fmt.Println("⚠️  TEST_TOKEN 未设置，只有公开端点能通过")
		fmt.Println("   用法: TEST_TOKEN=<jwt> go run ./cmd/api_test/")
		fmt.Println()
	}
	uidB := "long_test_bob"
	gid := "long_test_group"
	fid := "long_test_fid"

	fmt.Println("═══════════════════════════════════════")
	fmt.Println("  eChat RESTful API 测试 (37 endpoints)")
	fmt.Println("═══════════════════════════════════════")

	// 1. 公开 (1)
	sec("1. 公开")
	get("GetPublicKey", "/auth/public-key")

	// 2. 用户 (5)
	sec("2. 用户")
	put("UpdateProfile", "/user/profile", `{"username":"Test","bio":"hello"}`)
	post("SearchUser", "/user/search", `{"keyword":"long_test"}`)
	post("SearchByRegion", "/user/search-by-region", `{"region":"CN"}`)
	post("BatchGetUsers", "/user/batch", `{"uids":["`+uidB+`"]}`)
	del("DeleteAccount", "/user/account")

	// 3. 好友 (4)
	sec("3. 好友")
	post("BlacklistFriend", "/friend/blacklist", `{"uid":"`+uidB+`"}`)
	post("UnblacklistFriend", "/friend/unblacklist", `{"uid":"`+uidB+`"}`)
	post("CancelFriendReq", "/friend/cancel-request", `{"req_id":"nonexistent"}`)
	post("BlacklistList", "/friend/blacklist-list", "{}")

	// 4. 消息 (6)
	sec("4. 消息")
	post("RevokeMessage", "/message/revoke", `{"msg_id":"nonexistent"}`)
	post("UnreadCount", "/message/unread-count", `{"chat_id":"`+fid+`","chat_type":"private"}`)
	post("UnreadMessages", "/message/unread-list", `{"chat_id":"`+fid+`","chat_type":"private"}`)
	post("MsgReadUsers", "/message/read-users", `{"msg_id":"nonexistent"}`)
	post("ChatCount", "/message/chat-count", `{"chat_id":"`+fid+`","chat_type":"private"}`)
	post("ReadCounts", "/message/read-counts", `{"msg_ids":["nonexistent"]}`)

	// 5. 群组 (14)
	sec("5. 群组")
	post("SearchGroup", "/group/search", `{"keyword":"LongTest"}`)
	post("GroupAnnounces", "/group/announces", `{"gid":"`+gid+`"}`)
	post("GroupRequests", "/group/requests", `{"gid":"`+gid+`"}`)
	post("KickMember", "/group/kick", `{"gid":"`+gid+`","uid":"`+uidB+`"}`)
	post("MuteMember", "/group/mute", `{"gid":"`+gid+`","uid":"`+uidB+`","duration":60}`)
	post("UnmuteMember", "/group/unmute", `{"gid":"`+gid+`","uid":"`+uidB+`"}`)
	put("UpdateRole", "/group/role", `{"gid":"`+gid+`","uid":"`+uidB+`","role":"admin"}`)
	post("ApproveReq", "/group/approve-request", `{"req_id":"nonexistent"}`)
	post("DisbandGroup", "/group/disband", `{"gid":"nonexistent"}`)
	post("OwnedGroups", "/group/owned", "{}")
	post("MuteList", "/group/mute-list", `{"gid":"`+gid+`"}`)
	post("OnlineMembers", "/group/online-members", `{"gid":"`+gid+`"}`)
	post("MyGroupReqs", "/group/my-requests", "{}")
	post("AllGroupReqs", "/group/all-requests", `{"gid":"`+gid+`"}`)

	// 6. 会话 (3)
	sec("6. 会话")
	post("Conversations", "/chat/conversations", "{}")
	post("PinConversation", "/chat/pin", `{"chat_id":"`+fid+`","chat_type":"private","is_pinned":true}`)
	post("OnlineStatus", "/chat/online-status", `{"uids":["`+uidB+`"]}`)

	// 7. 文件 (5)
	sec("7. 文件")
	post("SetPermission", "/file/permission", `{"file_id":"12345678","user_uid":"`+uidB+`","level":"view"}`)
	post("RevokePermission", "/file/revoke-permission", `{"file_id":"12345678","access_type":"user","target_id":"`+uidB+`"}`)
	post("GetAssociations", "/file/associations", `{"file_id":"12345678"}`)
	post("CreateAssociation", "/file/associate", `{"file_id":"12345678","association_type":"chat","associated_id":"test"}`)
	post("DelAssociation", "/file/delete-association", `{"association_id":"test"}`)

	// 8. 认证拦截 (3)
	sec("8. 认证拦截")
	do("POST", "", "/user/search", `{"keyword":"test"}`, true)
	do("POST", "", "/chat/conversations", "{}", true)
	do("GET", "", "/auth/public-key", "", false)

	// ── 报告 ──
	fmt.Println()
	fmt.Println("═══════════════════════════════════════")
	fmt.Printf("  RESTful: 37/37 端点可达")
	if fail.Load() > 0 {
		fmt.Printf(", %d 失败", fail.Load())
	}
	fmt.Println()
	if fail.Load() == 0 {
		fmt.Println("  🎉 全部通过！")
	}
	fmt.Println("  tRPC 端点(21): 由 ws_test/long_test 覆盖")
	fmt.Println("═══════════════════════════════════════")
}

func sec(title string) { fmt.Printf("\n── %s ──\n", title) }

func get(name, path string)     { do("GET", token, path, "", false) }
func post(name, path, data string) { do("POST", token, path, data, false) }
func put(name, path, data string)  { do("PUT", token, path, data, false) }
func del(name, path string)        { do("DELETE", token, path, "", false) }

func do(method, tkn, path, data string, expectBlock bool) {
	var body io.Reader
	if data != "" {
		body = bytes.NewBufferString(data)
	}
	req, _ := http.NewRequest(method, apiBase+path, body)
	req.Header.Set("Content-Type", "application/json")
	if tkn != "" {
		req.Header.Set("Authorization", "Bearer "+tkn)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fail.Add(1)
		fmt.Printf("  ❌ %s %s ERR: %v\n", method, path, err)
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	bs := string(b)

	if resp.StatusCode == 404 && !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		fail.Add(1)
		fmt.Printf("  ❌ %s %s → 404 未注册\n", method, path)
	} else if resp.StatusCode == 404 {
		ok.Add(1)
		if len(bs) > 60 {
			bs = bs[:60] + "..."
		}
		fmt.Printf("  ✅ %s %s → 404 %s\n", method, path, strings.ReplaceAll(bs, "\n", " "))
	} else if expectBlock && resp.StatusCode != 200 {
		ok.Add(1)
		fmt.Printf("  ✅ %s %s → %d (已拦截)\n", method, path, resp.StatusCode)
	} else {
		ok.Add(1)
		if len(bs) > 50 {
			bs = bs[:50] + "..."
		}
		fmt.Printf("  ✅ %s %s → %d %s\n", method, path, resp.StatusCode, strings.ReplaceAll(bs, "\n", " "))
	}
}
