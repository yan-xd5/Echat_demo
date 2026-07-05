package main

import (
	"context"
	"encoding/json"

	thttp "trpc.group/trpc-go/trpc-go/http"

	"echat/sdk/entity"
	"echat/sdk/mysql"
	"echat/sdk/redis"
)

type extraFinalImpl struct {
	userRepo     *mysql.UserRepo
	friendRepo   *mysql.FriendRepo
	privateRepo  *mysql.PrivateChatRepo
	groupRepo    *mysql.GroupRepo
	groupMsgRepo *mysql.GroupMessageRepo
	fileRepo     *mysql.FileRepo
	onlineRepo   *redis.OnlineRepo
}

// SearchUserByRegion 按地区搜用户。
func (s *extraFinalImpl) SearchUserByRegion(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	if getUID(ctx) == "" {
		writeJSON(w, 401, msg("未登录")); return
	}
	var req struct{ Region string `json:"region"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Region == "" {
		writeJSON(w, 400, msg("缺少 region")); return
	}
	users, err := s.userRepo.FindUserByRegion(ctx, req.Region)
	if err != nil {
		writeJSON(w, 500, msg(err.Error())); return
	}
	type u struct {
		UID, Account, Username, Avatar string
	}
	list := make([]u, 0, len(users))
	for _, us := range users {
		list = append(list, u{us.UID, us.Account, us.Username, entity.PtrVal(us.Avatar)})
	}
	writeJSON(w, 200, map[string]interface{}{"code": 0, "users": list})
}

// DeleteAccount 注销账号。
func (s *extraFinalImpl) DeleteAccount(ctx context.Context) {
	w, _ := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(w, 401, msg("未登录")); return
	}
	if err := s.userRepo.DeleteUser(ctx, uid); err != nil {
		writeJSON(w, 500, msg(err.Error())); return
	}
	writeJSON(w, 200, msg("账号已注销"))
}

// CancelFriendRequest 取消好友申请。
func (s *extraFinalImpl) CancelFriendRequest(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(w, 401, msg("未登录")); return
	}
	var req struct{ ReqId string `json:"req_id"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.ReqId == "" {
		writeJSON(w, 400, msg("参数错误")); return
	}
	fr, err := s.friendRepo.FindFriendRequestByID(ctx, req.ReqId)
	if err != nil || fr == nil {
		writeJSON(w, 400, msg("申请不存在")); return
	}
	if fr.SenderUID != uid {
		writeJSON(w, 403, msg("无权操作")); return
	}
	if fr.Status != entity.ReqStatusPending {
		writeJSON(w, 400, msg("申请已处理")); return
	}
	if err := s.friendRepo.UpdateRequestStatus(ctx, req.ReqId, entity.ReqStatusRejected); err != nil {
		writeJSON(w, 500, msg(err.Error())); return
	}
	writeJSON(w, 200, msg("已取消"))
}

// BlacklistList 黑名单列表。
func (s *extraFinalImpl) BlacklistList(ctx context.Context) {
	w, _ := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(w, 401, msg("未登录")); return
	}
	friends, err := s.friendRepo.FindBlacklistedFriends(ctx, uid)
	if err != nil {
		writeJSON(w, 500, msg(err.Error())); return
	}
	type f struct {
		Fid, Uid string
	}
	list := make([]f, 0, len(friends))
	for _, fr := range friends {
		friendUID := fr.ToUID
		if fr.ToUID == uid {
			friendUID = fr.UID
		}
		list = append(list, f{fr.FID, friendUID})
	}
	writeJSON(w, 200, map[string]interface{}{"code": 0, "friends": list})
}

// UnreadMessages 查未读消息列表。
func (s *extraFinalImpl) UnreadMessages(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(w, 401, msg("未登录")); return
	}
	var req struct {
		ChatId   string `json:"chat_id"`
		ChatType string `json:"chat_type"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.ChatId == "" {
		writeJSON(w, 400, msg("参数错误")); return
	}
	type m struct {
		MsgId, Content, SenderUid string
		SendTime                  int64
	}
	var list []m
	if req.ChatType == "group" {
		msgs, err := s.groupMsgRepo.FindUnreadMessagesByUser(ctx, req.ChatId, uid)
		if err != nil {
			writeJSON(w, 500, msg(err.Error())); return
		}
		for _, msg := range msgs {
			st := int64(0)
			if msg.SendTime != nil {
				st = *msg.SendTime
			}
			list = append(list, m{msg.MsgID, msg.Content, msg.SenderUID, st})
		}
	} else {
		msgs, err := s.privateRepo.FindUnreadMessagesByChat(ctx, req.ChatId, uid)
		if err != nil {
			writeJSON(w, 500, msg(err.Error())); return
		}
		for _, msg := range msgs {
			st := int64(0)
			if msg.SendTime != nil {
				st = *msg.SendTime
			}
			list = append(list, m{msg.MsgID, msg.Content, msg.SenderUID, st})
		}
	}
	writeJSON(w, 200, map[string]interface{}{"code": 0, "messages": list})
}

// MessageReadUsers 查消息已读用户列表。
func (s *extraFinalImpl) MessageReadUsers(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	if getUID(ctx) == "" {
		writeJSON(w, 401, msg("未登录")); return
	}
	var req struct{ MsgId string `json:"msg_id"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.MsgId == "" {
		writeJSON(w, 400, msg("参数错误")); return
	}
	uids, err := s.groupMsgRepo.FindReadUsersByMessage(ctx, req.MsgId)
	if err != nil {
		writeJSON(w, 500, msg(err.Error())); return
	}
	writeJSON(w, 200, map[string]interface{}{"code": 0, "uids": uids})
}

// MessageChatCount 查会话消息总数。
func (s *extraFinalImpl) MessageChatCount(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	if getUID(ctx) == "" {
		writeJSON(w, 401, msg("未登录")); return
	}
	var req struct {
		ChatId   string `json:"chat_id"`
		ChatType string `json:"chat_type"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.ChatId == "" {
		writeJSON(w, 400, msg("参数错误")); return
	}
	var count int64
	var err error
	if req.ChatType == "group" {
		count, err = s.groupMsgRepo.GetMessageCountByGroup(ctx, req.ChatId)
	} else {
		count, err = s.privateRepo.GetMessageCountByChat(ctx, req.ChatId)
	}
	if err != nil {
		writeJSON(w, 500, msg(err.Error())); return
	}
	writeJSON(w, 200, map[string]interface{}{"code": 0, "count": count})
}

// OwnedGroups 我创建的群。
func (s *extraFinalImpl) OwnedGroups(ctx context.Context) {
	w, _ := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(w, 401, msg("未登录")); return
	}
	groups, err := s.groupRepo.FindGroupsByOwner(ctx, uid)
	if err != nil {
		writeJSON(w, 500, msg(err.Error())); return
	}
	type g struct {
		Gid, GroupName, GroupAvatar string
		MemberCount                 int
	}
	list := make([]g, 0, len(groups))
	for _, gr := range groups {
		cnt, _ := s.groupRepo.GetMemberCount(ctx, gr.GID)
		list = append(list, g{gr.GID, gr.GroupName, entity.PtrVal(gr.GroupAvatar), cnt})
	}
	writeJSON(w, 200, map[string]interface{}{"code": 0, "groups": list})
}

// MuteList 群禁言列表。
func (s *extraFinalImpl) MuteList(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(w, 401, msg("未登录")); return
	}
	var req struct{ Gid string `json:"gid"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Gid == "" {
		writeJSON(w, 400, msg("参数错误")); return
	}
	// 检查是否是成员
	if _, err := s.groupRepo.FindMember(ctx, req.Gid, uid); err != nil {
		writeJSON(w, 403, msg("不是群成员")); return
	}
	records, err := s.groupRepo.FindMuteRecordsByGroup(ctx, req.Gid)
	if err != nil {
		writeJSON(w, 500, msg(err.Error())); return
	}
	type mr struct {
		BanId, Uid    string
		Duration      int
		StartTime     int64
	}
	list := make([]mr, 0, len(records))
	for _, rec := range records {
		st := int64(0)
		if rec.StartTime != nil {
			st = *rec.StartTime
		}
		list = append(list, mr{rec.BanID, rec.UID, rec.MuteDuration, st})
	}
	writeJSON(w, 200, map[string]interface{}{"code": 0, "mutes": list})
}

// GroupOnlineMembers 群在线成员。
func (s *extraFinalImpl) GroupOnlineMembers(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	if getUID(ctx) == "" {
		writeJSON(w, 401, msg("未登录")); return
	}
	var req struct{ Gid string `json:"gid"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Gid == "" {
		writeJSON(w, 400, msg("参数错误")); return
	}
	uids, err := s.onlineRepo.GetGroupOnlineMembers(ctx, req.Gid)
	if err != nil {
		writeJSON(w, 500, msg(err.Error())); return
	}
	writeJSON(w, 200, map[string]interface{}{"code": 0, "online_uids": uids})
}

// MyGroupRequests 我的入群申请。
func (s *extraFinalImpl) MyGroupRequests(ctx context.Context) {
	w, _ := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(w, 401, msg("未登录")); return
	}
	reqs, err := s.groupRepo.FindRequestsByUser(ctx, uid)
	if err != nil {
		writeJSON(w, 500, msg(err.Error())); return
	}
	type myReq struct {
		ReqId, Gid, Status, ApplyText string
		CreateTime                   int64
	}
	list := make([]myReq, 0, len(reqs))
	for _, gr := range reqs {
		ct := int64(0)
		if gr.CreateTime != nil {
			ct = *gr.CreateTime
		}
		at := ""
		if gr.ApplyText != nil {
			at = *gr.ApplyText
		}
		list = append(list, myReq{gr.ReqID, gr.GID, string(gr.Status), at, ct})
	}
	writeJSON(w, 200, map[string]interface{}{"code": 0, "requests": list})
}

// AllGroupRequests 群全部入群申请（含已处理）。
func (s *extraFinalImpl) AllGroupRequests(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(w, 401, msg("未登录")); return
	}
	var req struct{ Gid string `json:"gid"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Gid == "" {
		writeJSON(w, 400, msg("参数错误")); return
	}
	op, _ := s.groupRepo.FindMember(ctx, req.Gid, uid)
	if op == nil || (op.Role != entity.RoleOwner && op.Role != entity.RoleAdmin) {
		writeJSON(w, 403, msg("无权查看")); return
	}
	reqs, err := s.groupRepo.FindAllRequestsByGroup(ctx, req.Gid)
	if err != nil {
		writeJSON(w, 500, msg(err.Error())); return
	}
	type reqEntry struct {
		ReqId, Gid, ApplicantUid, ApproverUid, Status, ApplyText string
		CreateTime, HandleTime                                    int64
	}
	list := make([]reqEntry, 0, len(reqs))
	for _, gr := range reqs {
		ct, ht := int64(0), int64(0)
		if gr.CreateTime != nil {
			ct = *gr.CreateTime
		}
		if gr.HandleTime != nil {
			ht = *gr.HandleTime
		}
		at, au := "", ""
		if gr.ApplyText != nil {
			at = *gr.ApplyText
		}
		if gr.ApproverUID != nil {
			au = *gr.ApproverUID
		}
		list = append(list, reqEntry{gr.ReqID, gr.GID, gr.ApplicantUID, au, string(gr.Status), at, ct, ht})
	}
	writeJSON(w, 200, map[string]interface{}{"code": 0, "requests": list})
}

// DeleteFileAssociation 删除文件关联。
func (s *extraFinalImpl) DeleteFileAssociation(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(w, 401, msg("未登录")); return
	}
	var req struct{ AssociationId string `json:"association_id"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.AssociationId == "" {
		writeJSON(w, 400, msg("参数错误")); return
	}
	if err := s.fileRepo.DeleteFileAssociation(ctx, req.AssociationId); err != nil {
		writeJSON(w, 500, msg(err.Error())); return
	}
	writeJSON(w, 200, msg("已删除"))
}

func msg(s string) map[string]interface{} {
	return map[string]interface{}{"code": 0, "message": s}
}
