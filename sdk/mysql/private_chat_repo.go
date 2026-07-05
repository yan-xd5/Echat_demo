package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"

	"echat/sdk/entity"
)

// PrivateChatRepo 私聊数据访问实现。
type PrivateChatRepo struct {
	DB *sqlx.DB
}

func NewPrivateChatRepo(db *sqlx.DB) *PrivateChatRepo { return &PrivateChatRepo{DB: db} }

// ======================== 会话管理 ========================

func (r *PrivateChatRepo) SaveChat(ctx context.Context, chat *entity.PrivateChat) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO private_chat (pid, uid1, uid2, is_pinned_by_uid1, is_pinned_by_uid2, do_not_disturb_uid1, do_not_disturb_uid2)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		 is_pinned_by_uid1=VALUES(is_pinned_by_uid1), is_pinned_by_uid2=VALUES(is_pinned_by_uid2),
		 do_not_disturb_uid1=VALUES(do_not_disturb_uid1), do_not_disturb_uid2=VALUES(do_not_disturb_uid2)`,
		chat.PID, chat.UID1, chat.UID2, chat.IsPinnedByUID1, chat.IsPinnedByUID2, chat.DoNotDisturbUID1, chat.DoNotDisturbUID2)
	if err != nil {
		return fmt.Errorf("save private_chat: %w", err)
	}
	return tx.Commit()
}

func (r *PrivateChatRepo) FindChatByPID(ctx context.Context, pid string) (*entity.PrivateChat, error) {
	var c entity.PrivateChat
	err := r.DB.GetContext(ctx, &c,
		`SELECT pid, uid1, uid2, UNIX_TIMESTAMP(create_time)*1000 AS create_time,
		 is_pinned_by_uid1, is_pinned_by_uid2, do_not_disturb_uid1, do_not_disturb_uid2
		 FROM private_chat WHERE pid = ?`, pid)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &c, err
}

func (r *PrivateChatRepo) FindChatByUsers(ctx context.Context, uid1, uid2 string) (*entity.PrivateChat, error) {
	a, b := uid1, uid2
	if a > b {
		a, b = b, a
	}
	var c entity.PrivateChat
	err := r.DB.GetContext(ctx, &c,
		`SELECT pid, uid1, uid2, UNIX_TIMESTAMP(create_time)*1000 AS create_time,
		 is_pinned_by_uid1, is_pinned_by_uid2, do_not_disturb_uid1, do_not_disturb_uid2
		 FROM private_chat WHERE uid1 = ? AND uid2 = ?`, a, b)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &c, err
}

func (r *PrivateChatRepo) FindChatsByUser(ctx context.Context, uid string) ([]*entity.PrivateChat, error) {
	var chats []entity.PrivateChat
	err := r.DB.SelectContext(ctx, &chats,
		`SELECT pid, uid1, uid2, UNIX_TIMESTAMP(create_time)*1000 AS create_time,
		 is_pinned_by_uid1, is_pinned_by_uid2, do_not_disturb_uid1, do_not_disturb_uid2
		 FROM private_chat WHERE uid1 = ? OR uid2 = ? ORDER BY create_time DESC`, uid, uid)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.PrivateChat, len(chats))
	for i := range chats {
		ptr[i] = &chats[i]
	}
	return ptr, nil
}

func (r *PrivateChatRepo) UpdatePinStatus(ctx context.Context, pid, uid string, isPinned bool) error {
	var info struct{ UID1, UID2 string }
	err := r.DB.GetContext(ctx, &info, `SELECT uid1, uid2 FROM private_chat WHERE pid = ?`, pid)
	if err != nil {
		return err
	}
	field := "is_pinned_by_uid1"
	if uid == info.UID2 {
		field = "is_pinned_by_uid2"
	}
	pin := 0
	if isPinned {
		pin = 1
	}
	_, err = r.DB.ExecContext(ctx, `UPDATE private_chat SET `+field+` = ? WHERE pid = ?`, pin, pid)
	return err
}

// ======================== 消息管理 ========================

func (r *PrivateChatRepo) SaveMessage(ctx context.Context, msg *entity.PrivateMessage) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO private_message (msg_id, pid, seq_id, content, sender_uid, type)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE content=VALUES(content), type=VALUES(type)`,
		msg.MsgID, msg.PID, msg.SeqID, msg.Content, msg.SenderUID, string(msg.Type))
	if err != nil {
		return fmt.Errorf("save private_message: %w", err)
	}
	return tx.Commit()
}

func (r *PrivateChatRepo) FindMessageByID(ctx context.Context, msgID string) (*entity.PrivateMessage, error) {
	var m entity.PrivateMessage
	err := r.DB.GetContext(ctx, &m,
		`SELECT msg_id, pid, content, sender_uid, seq_id,
		 UNIX_TIMESTAMP(send_time)*1000 AS send_time, is_revoked, is_read, type
		 FROM private_message WHERE msg_id = ?`, msgID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &m, err
}

func (r *PrivateChatRepo) FindMessagesByChat(ctx context.Context, pid string) ([]*entity.PrivateMessage, error) {
	var msgs []entity.PrivateMessage
	err := r.DB.SelectContext(ctx, &msgs,
		`SELECT msg_id, pid, content, sender_uid, seq_id,
		 UNIX_TIMESTAMP(send_time)*1000 AS send_time, is_revoked, is_read, type
		 FROM private_message WHERE pid = ? ORDER BY send_time DESC`, pid)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.PrivateMessage, len(msgs))
	for i := range msgs {
		ptr[i] = &msgs[i]
	}
	return ptr, nil
}

func (r *PrivateChatRepo) MarkMessageAsRead(ctx context.Context, msgID string) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx, `UPDATE private_message SET is_read = 1 WHERE msg_id = ?`, msgID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *PrivateChatRepo) MarkMessagesAsReadByChatAndTime(ctx context.Context, pid, uid string, timestamp int64) (int64, error) {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return 0, fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx,
		`UPDATE private_message SET is_read = 1
		 WHERE pid = ? AND sender_uid != ? AND send_time <= FROM_UNIXTIME(?/1000)
		 AND (is_read IS NULL OR is_read = 0)`, pid, uid, timestamp)
	if err != nil {
		return 0, err
	}
	tx.Commit()
	n, _ := result.RowsAffected()
	return n, nil
}

func (r *PrivateChatRepo) MarkMessageAsRevoked(ctx context.Context, msgID string) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx, `UPDATE private_message SET is_revoked = 1 WHERE msg_id = ?`, msgID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *PrivateChatRepo) FindUnreadMessagesByChat(ctx context.Context, pid, uid string) ([]*entity.PrivateMessage, error) {
	var msgs []entity.PrivateMessage
	err := r.DB.SelectContext(ctx, &msgs,
		`SELECT msg_id, pid, content, sender_uid, seq_id,
		 UNIX_TIMESTAMP(send_time)*1000 AS send_time, is_revoked, is_read, type
		 FROM private_message WHERE pid = ? AND sender_uid != ?
		 AND (is_read IS NULL OR is_read = 0) ORDER BY send_time DESC`, pid, uid)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.PrivateMessage, len(msgs))
	for i := range msgs {
		ptr[i] = &msgs[i]
	}
	return ptr, nil
}

func (r *PrivateChatRepo) GetUnreadMessageCountByChat(ctx context.Context, pid, uid string) (int, error) {
	var count int
	err := r.DB.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM private_message WHERE pid = ? AND sender_uid != ?
		 AND (is_read IS NULL OR is_read = 0)`, pid, uid)
	return count, err
}

func (r *PrivateChatRepo) FindLatestMessageByChat(ctx context.Context, pid string) (*entity.PrivateMessage, error) {
	var m entity.PrivateMessage
	err := r.DB.GetContext(ctx, &m,
		`SELECT msg_id, pid, content, sender_uid, seq_id,
		 UNIX_TIMESTAMP(send_time)*1000 AS send_time, is_revoked, is_read, type
		 FROM private_message WHERE pid = ? ORDER BY send_time DESC LIMIT 1`, pid)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &m, err
}

func (r *PrivateChatRepo) GetMessageCountByChat(ctx context.Context, pid string) (int64, error) {
	var count int64
	err := r.DB.GetContext(ctx, &count, `SELECT COUNT(*) FROM private_message WHERE pid = ?`, pid)
	return count, err
}
