package mysql

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"echat/sdk/domain/entity"
)

// MessageStore 消息持久化实现。
type MessageStore struct {
	DB *sqlx.DB
}

// NewMessageStore 创建消息持久化存储。
func NewMessageStore(db *sqlx.DB) *MessageStore {
	return &MessageStore{DB: db}
}

// SavePrivateMessage 保存私聊消息。
func (s *MessageStore) SavePrivateMessage(ctx context.Context, msg *entity.PrivateMessage) error {
	now := time.Now()
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO private_message (msg_id, pid, seq_id, content, sender_uid, send_time, type)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE content = VALUES(content)`,
		msg.MsgID, msg.PID, msg.SeqID, msg.Content, msg.SenderUID,
		now, string(msg.Type),
	)
	if err != nil {
		return fmt.Errorf("save private_message: %w", err)
	}
	return nil
}

// SaveGroupMessage 保存群聊消息。
func (s *MessageStore) SaveGroupMessage(ctx context.Context, msg *entity.GroupMessage) error {
	now := time.Now()
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO group_message (msg_id, gid, seq_id, content, sender_uid, send_time, type, quote_msg_id, mentioned_uids, is_announcement)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE content = VALUES(content)`,
		msg.MsgID, msg.GID, msg.SeqID, msg.Content, msg.SenderUID,
		now, string(msg.Type), msg.QuoteMsgID, msg.MentionedUIDs, msg.IsAnnouncement,
	)
	if err != nil {
		return fmt.Errorf("save group_message: %w", err)
	}
	return nil
}
