package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"echat/sdk/domain/entity"
)

// GroupMessageRepo 群消息数据访问实现。
type GroupMessageRepo struct {
	DB *sqlx.DB
}

func NewGroupMessageRepo(db *sqlx.DB) *GroupMessageRepo { return &GroupMessageRepo{DB: db} }

// ======================== 消息管理 ========================

func (r *GroupMessageRepo) SaveMessage(ctx context.Context, msg *entity.GroupMessage) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO group_message (msg_id, gid, seq_id, content, sender_uid, type, mentioned_uids, quote_msg_id, is_announcement)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE content=VALUES(content), type=VALUES(type),
		 mentioned_uids=VALUES(mentioned_uids), quote_msg_id=VALUES(quote_msg_id), is_announcement=VALUES(is_announcement)`,
		msg.MsgID, msg.GID, msg.SeqID, msg.Content, msg.SenderUID,
		string(msg.Type), msg.MentionedUIDs, msg.QuoteMsgID, msg.IsAnnouncement)
	if err != nil {
		return fmt.Errorf("save group_message: %w", err)
	}
	return tx.Commit()
}

func (r *GroupMessageRepo) FindMessageByID(ctx context.Context, msgID string) (*entity.GroupMessage, error) {
	var m entity.GroupMessage
	err := r.DB.GetContext(ctx, &m,
		`SELECT msg_id, gid, content, sender_uid, seq_id,
		 UNIX_TIMESTAMP(send_time)*1000 AS send_time, is_revoked, type,
		 mentioned_uids, quote_msg_id, is_announcement
		 FROM group_message WHERE msg_id = ?`, msgID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &m, err
}

func (r *GroupMessageRepo) FindMessagesByGroup(ctx context.Context, gid string) ([]*entity.GroupMessage, error) {
	var msgs []entity.GroupMessage
	err := r.DB.SelectContext(ctx, &msgs,
		`SELECT msg_id, gid, content, sender_uid, seq_id,
		 UNIX_TIMESTAMP(send_time)*1000 AS send_time, is_revoked, type,
		 mentioned_uids, quote_msg_id, is_announcement
		 FROM group_message WHERE gid = ? ORDER BY send_time DESC`, gid)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.GroupMessage, len(msgs))
	for i := range msgs {
		ptr[i] = &msgs[i]
	}
	return ptr, nil
}

func (r *GroupMessageRepo) FindMessagesByGroupWithPagination(ctx context.Context, gid string, limit, offset int64) ([]*entity.GroupMessage, error) {
	var msgs []entity.GroupMessage
	err := r.DB.SelectContext(ctx, &msgs,
		`SELECT msg_id, gid, content, sender_uid, seq_id,
		 UNIX_TIMESTAMP(send_time)*1000 AS send_time, is_revoked, type,
		 mentioned_uids, quote_msg_id, is_announcement
		 FROM group_message WHERE gid = ? ORDER BY send_time DESC LIMIT ? OFFSET ?`,
		gid, limit, offset)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.GroupMessage, len(msgs))
	for i := range msgs {
		ptr[i] = &msgs[i]
	}
	return ptr, nil
}

func (r *GroupMessageRepo) FindMessagesByGroupAndTimeRange(ctx context.Context, gid string, start, end int64) ([]*entity.GroupMessage, error) {
	var msgs []entity.GroupMessage
	err := r.DB.SelectContext(ctx, &msgs,
		`SELECT msg_id, gid, content, sender_uid, seq_id,
		 UNIX_TIMESTAMP(send_time)*1000 AS send_time, is_revoked, type,
		 mentioned_uids, quote_msg_id, is_announcement
		 FROM group_message WHERE gid = ? AND send_time BETWEEN FROM_UNIXTIME(?/1000) AND FROM_UNIXTIME(?/1000)
		 ORDER BY send_time DESC`, gid, start, end)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.GroupMessage, len(msgs))
	for i := range msgs {
		ptr[i] = &msgs[i]
	}
	return ptr, nil
}

func (r *GroupMessageRepo) GetMessageCountByGroup(ctx context.Context, gid string) (int64, error) {
	var count int64
	err := r.DB.GetContext(ctx, &count, `SELECT COUNT(*) FROM group_message WHERE gid = ?`, gid)
	return count, err
}

func (r *GroupMessageRepo) MarkMessageAsRevoked(ctx context.Context, msgID string) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx, `UPDATE group_message SET is_revoked = true WHERE msg_id = ?`, msgID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *GroupMessageRepo) FindAnnouncesByGroup(ctx context.Context, gid string) ([]*entity.GroupMessage, error) {
	var msgs []entity.GroupMessage
	err := r.DB.SelectContext(ctx, &msgs,
		`SELECT msg_id, gid, content, sender_uid, seq_id,
		 UNIX_TIMESTAMP(send_time)*1000 AS send_time, is_revoked, type,
		 mentioned_uids, quote_msg_id, is_announcement
		 FROM group_message WHERE gid = ? AND is_announcement = true ORDER BY send_time DESC`, gid)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.GroupMessage, len(msgs))
	for i := range msgs {
		ptr[i] = &msgs[i]
	}
	return ptr, nil
}

// ======================== 已读状态 ========================

func (r *GroupMessageRepo) MarkMessageAsRead(ctx context.Context, msgID, gid, uid string) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx,
		`INSERT IGNORE INTO group_message_read (msg_id, gid, uid) VALUES (?, ?, ?)`, msgID, gid, uid)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *GroupMessageRepo) FindReadUsersByMessage(ctx context.Context, msgID string) ([]string, error) {
	var uids []string
	err := r.DB.SelectContext(ctx, &uids, `SELECT uid FROM group_message_read WHERE msg_id = ?`, msgID)
	return uids, err
}

func (r *GroupMessageRepo) FindUnreadMessagesByUser(ctx context.Context, gid, uid string) ([]*entity.GroupMessage, error) {
	var msgs []entity.GroupMessage
	err := r.DB.SelectContext(ctx, &msgs,
		`SELECT gm.msg_id, gm.gid, gm.content, gm.sender_uid, gm.seq_id,
		 UNIX_TIMESTAMP(gm.send_time)*1000 AS send_time, gm.is_revoked, gm.type,
		 gm.mentioned_uids, gm.quote_msg_id, gm.is_announcement
		 FROM group_message gm
		 LEFT JOIN group_message_read gmr ON gm.msg_id = gmr.msg_id AND gmr.uid = ?
		 WHERE gm.gid = ? AND gmr.msg_id IS NULL ORDER BY gm.send_time DESC`, uid, gid)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.GroupMessage, len(msgs))
	for i := range msgs {
		ptr[i] = &msgs[i]
	}
	return ptr, nil
}

func (r *GroupMessageRepo) GetUnreadMessageCountByGroup(ctx context.Context, gid, uid string) (int, error) {
	var count int
	err := r.DB.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM group_message gm
		 LEFT JOIN group_message_read gmr ON gm.msg_id = gmr.msg_id AND gmr.uid = ?
		 WHERE gm.gid = ? AND gmr.msg_id IS NULL AND gm.sender_uid != ?`, uid, gid, uid)
	return count, err
}

func (r *GroupMessageRepo) GetMessageReadCount(ctx context.Context, msgID string) (int64, error) {
	var count int64
	err := r.DB.GetContext(ctx, &count, `SELECT COUNT(*) FROM group_message_read WHERE msg_id = ?`, msgID)
	return count, err
}

func (r *GroupMessageRepo) GetMessageReadCounts(ctx context.Context, msgIDs []string) (map[string]int64, error) {
	if len(msgIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(msgIDs))
	args := make([]interface{}, len(msgIDs))
	for i, id := range msgIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf(`SELECT msg_id, COUNT(*) AS cnt FROM group_message_read WHERE msg_id IN (%s) GROUP BY msg_id`,
		strings.Join(placeholders, ","))
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]int64)
	for rows.Next() {
		var mid string
		var cnt int64
		if err := rows.Scan(&mid, &cnt); err != nil {
		continue
	}
	m[mid] = cnt
}
if err := rows.Err(); err != nil {
	return nil, err
}
return m, nil
}

func (r *GroupMessageRepo) FindLatestMessageByGroup(ctx context.Context, gid string) (*entity.GroupMessage, error) {
	var m entity.GroupMessage
	err := r.DB.GetContext(ctx, &m,
		`SELECT msg_id, gid, content, sender_uid, seq_id,
		 UNIX_TIMESTAMP(send_time)*1000 AS send_time, is_revoked, type,
		 mentioned_uids, quote_msg_id, is_announcement
		 FROM group_message WHERE gid = ? ORDER BY send_time DESC LIMIT 1`, gid)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &m, err
}

func (r *GroupMessageRepo) MarkMessagesAsReadByGroupAndTime(ctx context.Context, gid, uid string, timestamp int64) (int64, error) {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return 0, fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx,
		`INSERT IGNORE INTO group_message_read (msg_id, gid, uid)
		 SELECT gm.msg_id, gm.gid, ? FROM group_message gm
		 LEFT JOIN group_message_read gmr ON gm.msg_id = gmr.msg_id AND gmr.uid = ?
		 WHERE gm.gid = ? AND gm.send_time <= FROM_UNIXTIME(?/1000) AND gmr.msg_id IS NULL`,
		uid, uid, gid, timestamp)
	if err != nil {
		return 0, err
	}
	tx.Commit()
	n, _ := result.RowsAffected()
	return n, nil
}
