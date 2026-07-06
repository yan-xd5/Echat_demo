package main

import (
	"context"
	"encoding/json"

	thttp "trpc.group/trpc-go/trpc-go/http"

	"echat/sdk/entity"
	"echat/sdk/mysql"
	"echat/sdk/redis"
)

type extraChatImpl struct {
	privateRepo  *mysql.PrivateChatRepo
	groupRepo   *mysql.GroupRepo
	groupMsgRepo *mysql.GroupMessageRepo
	onlineRepo  *redis.OnlineRepo
	userRepo    *mysql.UserRepo
}

// GetConversations 获取会话列表（私聊 + 群聊）。
func (s *extraChatImpl) GetConversations(ctx context.Context) {
	w, _ := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	type convInfo struct {
		ChatID      string `json:"chat_id"`
		ChatType    string `json:"chat_type"` // private / group
		Name        string `json:"name"`
		Avatar      string `json:"avatar"`
		LastMsg     string `json:"last_msg"`
		LastTime    int64  `json:"last_time"`
		UnreadCount int    `json:"unread_count"`
		IsPinned    bool   `json:"is_pinned"`
	}
	var list []convInfo

	// 私聊会话
	chats, err := s.privateRepo.FindChatsByUser(ctx, uid)
		if err != nil {
			writeJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
			return
		}
	for _, c := range chats {
		friendUID := c.UID1
		if friendUID == uid {
			friendUID = c.UID2
		}
		friend, _ := s.userRepo.FindUserByUID(ctx, friendUID)

		latest, _ := s.privateRepo.FindLatestMessageByChat(ctx, c.PID)
		unread, _ := s.privateRepo.GetUnreadMessageCountByChat(ctx, c.PID, uid)

		name := friendUID
		avatar := ""
		if friend != nil {
			name = friend.Username
			avatar = entity.PtrVal(friend.Avatar)
		}
		lastTime := int64(0)
		lastMsg := ""
		if latest != nil {
			if latest.SendTime != nil {
				lastTime = *latest.SendTime
			}
			lastMsg = latest.Content
		}
		isPinned := false
		if uid == c.UID1 && c.IsPinnedByUID1 != nil {
			isPinned = *c.IsPinnedByUID1
		} else if uid == c.UID2 && c.IsPinnedByUID2 != nil {
			isPinned = *c.IsPinnedByUID2
		}

		list = append(list, convInfo{
			ChatID: c.PID, ChatType: "private", Name: name, Avatar: avatar,
			LastMsg: lastMsg, LastTime: lastTime, UnreadCount: unread, IsPinned: isPinned,
		})
	}

	// 群聊会话
	members, _ := s.groupRepo.FindGroupsByUser(ctx, uid)
	gids := make([]string, 0, len(members))
	for _, m := range members {
		gids = append(gids, m.GID)
	}
	groupMap, _ := s.groupRepo.FindGroupsByGIDs(ctx, gids)

	for _, m := range members {
		g, ok := groupMap[m.GID]
		if !ok {
			continue
		}
		latest, _ := s.groupMsgRepo.FindLatestMessageByGroup(ctx, m.GID)
		unread, _ := s.groupMsgRepo.GetUnreadMessageCountByGroup(ctx, m.GID, uid)

		lastTime := int64(0)
		lastMsg := ""
		if latest != nil {
			if latest.SendTime != nil {
				lastTime = *latest.SendTime
			}
			lastMsg = latest.Content
		}
		isPinned := false
		if m.IsPinned != nil {
			isPinned = *m.IsPinned
		}

		list = append(list, convInfo{
			ChatID: m.GID, ChatType: "group", Name: g.GroupName,
			Avatar: entity.PtrVal(g.GroupAvatar),
			LastMsg: lastMsg, LastTime: lastTime, UnreadCount: unread, IsPinned: isPinned,
		})
	}

	writeJSON(ctx, w, 200, map[string]interface{}{"code": 0, "conversations": list})
}

// PinConversation 置顶/取消置顶会话。
func (s *extraChatImpl) PinConversation(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		ChatId   string `json:"chat_id"`
		ChatType string `json:"chat_type"`
		IsPinned bool   `json:"is_pinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ChatId == "" {
		writeJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	switch req.ChatType {
	case "group":
		m, _ := s.groupRepo.FindMember(ctx, req.ChatId, uid)
		if m == nil {
			writeJSON(ctx, w, 403, map[string]interface{}{"code": 1, "message": "不是群成员"})
			return
		}
		m.IsPinned = &req.IsPinned
		s.groupRepo.SaveMember(ctx, m)
	default:
		if err := s.privateRepo.UpdatePinStatus(ctx, req.ChatId, uid, req.IsPinned); err != nil {
			writeJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
			return
		}
	}
	writeJSON(ctx, w, 200, map[string]interface{}{"code": 0, "message": "已更新"})
}

// GetOnlineStatus 批量查询用户在线状态。
func (s *extraChatImpl) GetOnlineStatus(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	if getUID(ctx) == "" {
		writeJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		Uids []string `json:"uids"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if len(req.Uids) == 0 {
		writeJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	online, err := s.onlineRepo.BatchCheckOnlineStatus(ctx, req.Uids)
	if err != nil {
		writeJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	onlineSet := make(map[string]bool, len(online))
	for _, uid := range online {
		onlineSet[uid] = true
	}
	result := make(map[string]bool, len(req.Uids))
	for _, uid := range req.Uids {
		result[uid] = onlineSet[uid]
	}
	writeJSON(ctx, w, 200, map[string]interface{}{"code": 0, "online": result})
}

// DisbandGroup 解散群（仅群主）。
func (s *extraChatImpl) DisbandGroup(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		Gid string `json:"gid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Gid == "" {
		writeJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	g, err := s.groupRepo.FindGroupByGID(ctx, req.Gid)
	if err != nil || g == nil {
		writeJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "群不存在"})
		return
	}
	if g.ManagerUID != uid {
		writeJSON(ctx, w, 403, map[string]interface{}{"code": 1, "message": "只有群主能解散群"})
		return
	}
	if err := s.groupRepo.DeleteGroup(ctx, req.Gid); err != nil {
		writeJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	writeJSON(ctx, w, 200, map[string]interface{}{"code": 0, "message": "群已解散"})
}

// SearchGroup 搜索群。
func (s *extraChatImpl) SearchGroup(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	if getUID(ctx) == "" {
		writeJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		Keyword string `json:"keyword"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Keyword == "" {
		writeJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "缺少 keyword"})
		return
	}
	groups, err := s.groupRepo.FindGroupByName(ctx, req.Keyword)
	if err != nil {
		writeJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	type groupInfo struct {
		Gid         string `json:"gid"`
		GroupName   string `json:"group_name"`
		GroupAvatar string `json:"group_avatar"`
		GroupIntro  string `json:"group_intro"`
		MemberCount int    `json:"member_count"`
	}
	list := make([]groupInfo, 0, len(groups))
	for _, g := range groups {
		cnt, _ := s.groupRepo.GetMemberCount(ctx, g.GID)
		list = append(list, groupInfo{
			Gid: g.GID, GroupName: g.GroupName,
			GroupAvatar: entity.PtrVal(g.GroupAvatar),
			GroupIntro:  entity.PtrVal(g.GroupIntro),
			MemberCount: cnt,
		})
	}
	writeJSON(ctx, w, 200, map[string]interface{}{"code": 0, "groups": list})
}

// GetGroupAnnounces 获取群公告。
func (s *extraChatImpl) GetGroupAnnounces(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		Gid string `json:"gid"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Gid == "" {
		writeJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	// 检查是否是成员
	if _, err := s.groupRepo.FindMember(ctx, req.Gid, uid); err != nil {
		writeJSON(ctx, w, 403, map[string]interface{}{"code": 1, "message": "不是群成员"})
		return
	}
	announces, err := s.groupMsgRepo.FindAnnouncesByGroup(ctx, req.Gid)
	if err != nil {
		writeJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	type announceInfo struct {
		MsgId     string `json:"msg_id"`
		Content   string `json:"content"`
		SenderUid string `json:"sender_uid"`
		SendTime  int64  `json:"send_time"`
	}
	list := make([]announceInfo, 0, len(announces))
	for _, a := range announces {
		st := int64(0)
		if a.SendTime != nil {
			st = *a.SendTime
		}
		list = append(list, announceInfo{
			MsgId: a.MsgID, Content: a.Content, SenderUid: a.SenderUID, SendTime: st,
		})
	}
	writeJSON(ctx, w, 200, map[string]interface{}{"code": 0, "announces": list})
}
