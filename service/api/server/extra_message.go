package main

import (
	"context"
	"encoding/json"

	thttp "trpc.group/trpc-go/trpc-go/http"

	"echat/sdk/mysql"
)

type extraMessageImpl struct {
	msgRepo      *mysql.MessageRepo
	privateRepo  *mysql.PrivateChatRepo
	groupMsgRepo *mysql.GroupMessageRepo
}

func (s *extraMessageImpl) RevokeMessage(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		MsgId    string `json:"msg_id"`
		ChatType string `json:"chat_type"` // private / group
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MsgId == "" {
		writeJSON(w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	switch req.ChatType {
	case "group":
		msg, err := s.groupMsgRepo.FindMessageByID(ctx, req.MsgId)
		if err != nil || msg == nil {
			writeJSON(w, 400, map[string]interface{}{"code": 1, "message": "消息不存在"})
			return
		}
		if msg.SenderUID != uid {
			writeJSON(w, 403, map[string]interface{}{"code": 1, "message": "只能撤回自己的消息"})
			return
		}
		if err := s.groupMsgRepo.MarkMessageAsRevoked(ctx, req.MsgId); err != nil {
			writeJSON(w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
			return
		}
	default:
		msg, err := s.privateRepo.FindMessageByID(ctx, req.MsgId)
		if err != nil || msg == nil {
			writeJSON(w, 400, map[string]interface{}{"code": 1, "message": "消息不存在"})
			return
		}
		if msg.SenderUID != uid {
			writeJSON(w, 403, map[string]interface{}{"code": 1, "message": "只能撤回自己的消息"})
			return
		}
		if err := s.privateRepo.MarkMessageAsRevoked(ctx, req.MsgId); err != nil {
			writeJSON(w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
			return
		}
	}
	writeJSON(w, 200, map[string]interface{}{"code": 0, "message": "已撤回"})
}

func (s *extraMessageImpl) GetUnreadCount(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
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
		count, err = s.groupMsgRepo.GetUnreadMessageCountByGroup(ctx, chatID, uid)
	default:
		count, err = s.privateRepo.GetUnreadMessageCountByChat(ctx, chatID, uid)
	}
	if err != nil {
		writeJSON(w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"code": 0, "unread_count": count})
}
