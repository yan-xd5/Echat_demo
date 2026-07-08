package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"

	"echat/sdk/domain/entity"
)

type FriendRepo struct {
	DB *sqlx.DB
}

func NewFriendRepo(db *sqlx.DB) *FriendRepo { return &FriendRepo{DB: db} }

// ======================== 好友关系 ========================

func (r *FriendRepo) SaveFriendship(ctx context.Context, f *entity.Friends) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO friends (fid, uid, to_uid, is_blacklist, to_is_blacklist, remark, to_remark, groupby, to_groupby)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		 is_blacklist=VALUES(is_blacklist), to_is_blacklist=VALUES(to_is_blacklist),
		 remark=VALUES(remark), to_remark=VALUES(to_remark),
		 groupby=VALUES(groupby), to_groupby=VALUES(to_groupby)`,
		f.FID, f.UID, f.ToUID, f.IsBlacklist, f.ToIsBlacklist, f.Remark, f.ToRemark, f.GroupBy, f.ToGroupBy)
	if err != nil {
		return fmt.Errorf("save friendship: %w", err)
	}
	return tx.Commit()
}

func (r *FriendRepo) FindFriendshipByFID(ctx context.Context, fid string) (*entity.Friends, error) {
	var f entity.Friends
	err := r.DB.GetContext(ctx, &f,
		`SELECT fid, uid, to_uid, UNIX_TIMESTAMP(create_time)*1000 AS create_time,
		 is_blacklist, to_is_blacklist, remark, to_remark, groupby, to_groupby
		 FROM friends WHERE fid = ?`, fid)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &f, err
}

func (r *FriendRepo) FindFriendshipByUsers(ctx context.Context, uid1, uid2 string) (*entity.Friends, error) {
	a, b := uid1, uid2
	if a > b {
		a, b = b, a
	}
	var f entity.Friends
	err := r.DB.GetContext(ctx, &f,
		`SELECT fid, uid, to_uid, UNIX_TIMESTAMP(create_time)*1000 AS create_time,
		 is_blacklist, to_is_blacklist, remark, to_remark, groupby, to_groupby
		 FROM friends WHERE uid = ? AND to_uid = ?`, a, b)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &f, err
}

func (r *FriendRepo) FindFriendshipByUID(ctx context.Context, uid string) ([]*entity.Friends, error) {
	var fs []entity.Friends
	err := r.DB.SelectContext(ctx, &fs,
		`SELECT fid, uid, to_uid, UNIX_TIMESTAMP(create_time)*1000 AS create_time,
		 is_blacklist, to_is_blacklist, remark, to_remark, groupby, to_groupby
		 FROM friends WHERE uid = ? OR to_uid = ?`, uid, uid)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.Friends, len(fs))
	for i := range fs {
		ptr[i] = &fs[i]
	}
	return ptr, nil
}

func (r *FriendRepo) DeleteFriendship(ctx context.Context, fid string) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx, `DELETE FROM friends WHERE fid = ?`, fid)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *FriendRepo) DeleteFriendshipWithChat(ctx context.Context, fid string) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM private_message WHERE pid = ?`, fid); err != nil {
		return fmt.Errorf("delete messages: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM private_chat WHERE pid = ?`, fid); err != nil {
		return fmt.Errorf("delete chat: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM friends WHERE fid = ?`, fid); err != nil {
		return fmt.Errorf("delete friendship: %w", err)
	}
	return tx.Commit()
}

// ======================== 黑名单 ========================

func (r *FriendRepo) SaveBlacklist(ctx context.Context, fid, uid string, isBlacklist bool) error {
	f, err := r.FindFriendshipByFID(ctx, fid)
	if err != nil || f == nil {
		return fmt.Errorf("好友关系不存在: %s", fid)
	}
	field := "is_blacklist"
	if uid == f.ToUID {
		field = "to_is_blacklist"
	}
	val := 0
	if isBlacklist {
		val = 1
	}
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `UPDATE friends SET `+field+` = ? WHERE fid = ?`, val, fid)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *FriendRepo) FindBlacklistedFriends(ctx context.Context, uid string) ([]*entity.Friends, error) {
	var fs []entity.Friends
	err := r.DB.SelectContext(ctx, &fs,
		`SELECT fid, uid, to_uid, UNIX_TIMESTAMP(create_time)*1000 AS create_time,
		 is_blacklist, to_is_blacklist, remark, to_remark, groupby, to_groupby
		 FROM friends WHERE (uid = ? AND is_blacklist = 1) OR (to_uid = ? AND to_is_blacklist = 1)`, uid, uid)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.Friends, len(fs))
	for i := range fs {
		ptr[i] = &fs[i]
	}
	return ptr, nil
}

// ======================== 好友申请 ========================

func (r *FriendRepo) SaveFriendRequest(ctx context.Context, req *entity.FriendRequest) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO friend_request (req_id, sender_uid, receiver_uid, status, apply_text)
		 VALUES (?, ?, ?, ?, ?)`,
		req.ReqID, req.SenderUID, req.ReceiverUID, string(req.Status), req.ApplyText)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *FriendRepo) FindFriendRequestByID(ctx context.Context, reqID string) (*entity.FriendRequest, error) {
	var req entity.FriendRequest
	err := r.DB.GetContext(ctx, &req,
		`SELECT req_id, sender_uid, receiver_uid, status, apply_text,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time,
		 UNIX_TIMESTAMP(handle_time)*1000 AS handle_time
		 FROM friend_request WHERE req_id = ?`, reqID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &req, err
}

func (r *FriendRepo) FindFriendRequestByReceiver(ctx context.Context, receiverUID string) ([]*entity.FriendRequest, error) {
	var reqs []entity.FriendRequest
	err := r.DB.SelectContext(ctx, &reqs,
		`SELECT req_id, sender_uid, receiver_uid, status, apply_text,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time
		 FROM friend_request WHERE receiver_uid = ? ORDER BY create_time DESC`, receiverUID)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.FriendRequest, len(reqs))
	for i := range reqs {
		ptr[i] = &reqs[i]
	}
	return ptr, nil
}

func (r *FriendRepo) FindFriendRequestBySender(ctx context.Context, senderUID string) ([]*entity.FriendRequest, error) {
	var reqs []entity.FriendRequest
	err := r.DB.SelectContext(ctx, &reqs,
		`SELECT req_id, sender_uid, receiver_uid, status, apply_text,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time
		 FROM friend_request WHERE sender_uid = ? ORDER BY create_time DESC`, senderUID)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.FriendRequest, len(reqs))
	for i := range reqs {
		ptr[i] = &reqs[i]
	}
	return ptr, nil
}

func (r *FriendRepo) UpdateRequestStatus(ctx context.Context, reqID string, status entity.ReqStatus) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx,
		`UPDATE friend_request SET status = ?, handle_time = NOW() WHERE req_id = ?`, string(status), reqID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *FriendRepo) AcceptFriendRequestWithChat(ctx context.Context, reqID string, handleTime int64, friendship *entity.Friends, chat *entity.PrivateChat) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE friend_request SET status = 'accepted', handle_time = NOW() WHERE req_id = ?`, reqID); err != nil {
		return fmt.Errorf("update request: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO friends (fid, uid, to_uid) VALUES (?, ?, ?)`, friendship.FID, friendship.UID, friendship.ToUID); err != nil {
		return fmt.Errorf("insert friendship: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO private_chat (pid, uid1, uid2) VALUES (?, ?, ?)`, chat.PID, chat.UID1, chat.UID2); err != nil {
		return fmt.Errorf("insert chat: %w", err)
	}

	return tx.Commit()
}

// ======================== 权限校验 ========================

func (r *FriendRepo) ValidatePrivateMessagePermission(ctx context.Context, senderUID, receiverUID string) error {
	f, err := r.FindFriendshipByUsers(ctx, senderUID, receiverUID)
	if err != nil {
		return err
	}
	if f == nil {
		return fmt.Errorf("不是好友关系")
	}

	// 判断哪个字段对应接收者，检查接收者是否拉黑了发送者
	if f.UID == senderUID && f.ToUID == receiverUID {
		if f.ToIsBlacklist != nil && *f.ToIsBlacklist {
			return fmt.Errorf("已被对方拉黑")
		}
	} else if f.UID == receiverUID && f.ToUID == senderUID {
		if f.IsBlacklist != nil && *f.IsBlacklist {
			return fmt.Errorf("已被对方拉黑")
		}
	}

	return nil
}
