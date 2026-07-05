package mysql

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// MessageRepo 消息历史查询实现。
type MessageRepo struct {
	DB *sqlx.DB
}

// NewMessageRepo 创建消息仓库。
func NewMessageRepo(db *sqlx.DB) *MessageRepo { return &MessageRepo{DB: db} }

// HistoryMsg 历史消息通用结构。
type HistoryMsg struct {
	MsgID     string `db:"msg_id"     json:"msg_id"`
	SenderUID string `db:"sender_uid" json:"sender_uid"`
	Content   string `db:"content"    json:"content"`
	Type      string `db:"type"       json:"type"`
	SeqID     int64  `db:"seq_id"     json:"seq_id"`
	SendTime  int64  `db:"send_time"  json:"send_time"`
}

// GetPrivateHistory 查询私聊历史消息。
func (r *MessageRepo) GetPrivateHistory(ctx context.Context, pid string, before int64, limit int) ([]HistoryMsg, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT msg_id, sender_uid, content, type, seq_id, UNIX_TIMESTAMP(send_time)*1000 AS send_time
		FROM private_message WHERE pid = ?`
	args := []interface{}{pid}
	if before > 0 {
		query += " AND send_time < FROM_UNIXTIME(? / 1000)"
		args = append(args, before)
	}
	query += " ORDER BY send_time DESC LIMIT ?"
	args = append(args, limit)

	var msgs []HistoryMsg
	err := r.DB.SelectContext(ctx, &msgs, query, args...)
	return msgs, err
}

// GetGroupHistory 查询群聊历史消息。
func (r *MessageRepo) GetGroupHistory(ctx context.Context, gid string, before int64, limit int) ([]HistoryMsg, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT msg_id, sender_uid, content, type, seq_id, UNIX_TIMESTAMP(send_time)*1000 AS send_time
		FROM group_message WHERE gid = ?`
	args := []interface{}{gid}
	if before > 0 {
		query += " AND send_time < FROM_UNIXTIME(? / 1000)"
		args = append(args, before)
	}
	query += " ORDER BY send_time DESC LIMIT ?"
	args = append(args, limit)

	var msgs []HistoryMsg
	err := r.DB.SelectContext(ctx, &msgs, query, args...)
	return msgs, err
}

// MarkPrivateRead 标记私聊消息已读。
func (r *MessageRepo) MarkPrivateRead(ctx context.Context, pid, uid string) (int64, error) {
	result, err := r.DB.ExecContext(ctx,
		`UPDATE private_message SET is_read = true
		 WHERE pid = ? AND sender_uid != ? AND is_read = false`, pid, uid)
	if err != nil {
		return 0, fmt.Errorf("mark read: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}

// MarkGroupRead 标记群聊消息已读。
func (r *MessageRepo) MarkGroupRead(ctx context.Context, msgID, gid, uid string) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT IGNORE INTO group_message_read (msg_id, gid, uid) VALUES (?, ?, ?)`,
		msgID, gid, uid)
	return err
}
