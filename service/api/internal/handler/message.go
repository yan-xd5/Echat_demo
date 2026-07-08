package handler

import (
	"context"
	"fmt"

	"trpc.group/trpc-go/trpc-go/log"

	"echat/sdk/repository/mysql"
	pb "echat/service/api/stub"
	"echat/service/api/internal/shared"
)

// messageImpl 实现 MessageServiceService 接口。
type MessageImpl struct {
	MsgRepo    *mysql.MessageRepo
	chatRepo   *mysql.PrivateChatRepo
	GroupRepo  *mysql.GroupRepo
}

// checkParticipant 检查用户是否为会话参与者。
func (s *MessageImpl) checkParticipant(ctx context.Context, uid, chatID, chatType string) error {
	if chatType == "group" {
		m, err := s.GroupRepo.FindMember(ctx, chatID, uid)
		if err != nil || m == nil {
			return fmt.Errorf("不是群成员")
		}
	} else {
		c, err := s.chatRepo.FindChatByPID(ctx, chatID)
		if err != nil {
			return fmt.Errorf("查会话失败: %w", err)
		}
		if c == nil || (c.UID1 != uid && c.UID2 != uid) {
			return fmt.Errorf("不是会话参与者")
		}
	}
	return nil
}

// GetHistory 查询历史消息。
func (s *MessageImpl) GetHistory(ctx context.Context, req *pb.GetHistoryRequest) (*pb.GetHistoryResponse, error) {
	uid := shared.GetUID(ctx)
	if uid == "" {
		return nil, fmt.Errorf("未登录")
	}
	// 检查是否为会话参与者
	if err := s.checkParticipant(ctx, uid, req.ChatId, req.ChatType); err != nil {
		return nil, err
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}

	var msgs []mysql.HistoryMsg
	var err error
	if req.ChatType == "group" {
		msgs, err = s.MsgRepo.GetGroupHistory(ctx, req.ChatId, req.Before, limit)
	} else {
		msgs, err = s.MsgRepo.GetPrivateHistory(ctx, req.ChatId, req.Before, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("查历史消息失败: %w", err)
	}

	var list []*pb.HistoryMessage
	for _, m := range msgs {
		list = append(list, &pb.HistoryMessage{
			MsgId: m.MsgID, SenderUid: m.SenderUID, Content: m.Content,
			Type: m.Type, SeqId: m.SeqID, SendTime: m.SendTime,
		})
	}

	log.InfoContextf(ctx, "[API] 历史消息: chat=%s, type=%s, count=%d", req.ChatId, req.ChatType, len(list))
	return &pb.GetHistoryResponse{Messages: list}, nil
}

// MarkRead 标记已读。
func (s *MessageImpl) MarkRead(ctx context.Context, req *pb.MarkReadRequest) (*pb.MarkReadResponse, error) {
	uid := shared.GetUID(ctx)
	if uid == "" {
		return nil, fmt.Errorf("未登录")
	}
	if err := s.checkParticipant(ctx, uid, req.ChatId, req.ChatType); err != nil {
		return nil, err
	}

	var affected int64
	var err error
	if req.ChatType == "group" {
		err = s.MsgRepo.MarkGroupRead(ctx, req.MsgId, req.ChatId, uid)
		if err == nil {
			affected = 1
		}
	} else {
		affected, err = s.MsgRepo.MarkPrivateRead(ctx, req.ChatId, uid)
	}
	if err != nil {
		return nil, fmt.Errorf("标记已读失败: %w", err)
	}

	log.InfoContextf(ctx, "[API] 标记已读: chat=%s, affected=%d", req.ChatId, affected)
	return &pb.MarkReadResponse{Affected: affected}, nil
}
func NewMessageImpl(MsgRepo *mysql.MessageRepo, chatRepo *mysql.PrivateChatRepo, GroupRepo *mysql.GroupRepo) *MessageImpl {
	return &MessageImpl{MsgRepo: MsgRepo, chatRepo: chatRepo, GroupRepo: GroupRepo}
}
