package restful

import (
	"context"
	"encoding/json"

	thttp "trpc.group/trpc-go/trpc-go/http"

	"echat/sdk/repository/mysql"
	"echat/service/api/internal/shared"
)

// ExtraMessageImpl 消息相关的额外 RESTful API。
type ExtraMessageImpl struct {
	MessageRepo  *mysql.MessageRepo
	PrivateRepo  *mysql.PrivateChatRepo
	GroupMsgRepo *mysql.GroupMessageRepo
}

// NewExtraMessageImpl 创建 ExtraMessageImpl。
func NewExtraMessageImpl(msgRepo *mysql.MessageRepo, privateRepo *mysql.PrivateChatRepo, groupMsgRepo *mysql.GroupMessageRepo) *ExtraMessageImpl {
	return &ExtraMessageImpl{MessageRepo: msgRepo, PrivateRepo: privateRepo, GroupMsgRepo: groupMsgRepo}
}

// RevokeMessage 撤回消息。
func (s *ExtraMessageImpl) RevokeMessage(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := shared.GetUID(ctx)
	if uid == "" {
		shared.WriteJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		MsgId    string `json:"msg_id"`
		ChatType string `json:"chat_type"` // private / group
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MsgId == "" {
		shared.WriteJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	switch req.ChatType {
	case "group":
		msg, err := s.GroupMsgRepo.FindMessageByID(ctx, req.MsgId)
		if err != nil || msg == nil {
			shared.WriteJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "消息不存在"})
			return
		}
		if msg.SenderUID != uid {
			shared.WriteJSON(ctx, w, 403, map[string]interface{}{"code": 1, "message": "只能撤回自己的消息"})
			return
		}
		if err := s.GroupMsgRepo.MarkMessageAsRevoked(ctx, req.MsgId); err != nil {
			shared.WriteJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
			return
		}
	default:
		msg, err := s.PrivateRepo.FindMessageByID(ctx, req.MsgId)
		if err != nil || msg == nil {
			shared.WriteJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "消息不存在"})
			return
		}
		if msg.SenderUID != uid {
			shared.WriteJSON(ctx, w, 403, map[string]interface{}{"code": 1, "message": "只能撤回自己的消息"})
			return
		}
		if err := s.PrivateRepo.MarkMessageAsRevoked(ctx, req.MsgId); err != nil {
			shared.WriteJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
			return
		}
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "message": "已撤回"})
}

// GetUnreadCount 获取未读消息数。
func (s *ExtraMessageImpl) GetUnreadCount(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := shared.GetUID(ctx)
	if uid == "" {
		shared.WriteJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		ChatId   string `json:"chat_id"`
		ChatType string `json:"chat_type"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	chatID := req.ChatId
	chatType := req.ChatType

	var count int
	var err error
	switch chatType {
	case "group":
		count, err = s.GroupMsgRepo.GetUnreadMessageCountByGroup(ctx, chatID, uid)
	default:
		count, err = s.PrivateRepo.GetUnreadMessageCountByChat(ctx, chatID, uid)
	}
	if err != nil {
		shared.WriteJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "unread_count": count})
}
