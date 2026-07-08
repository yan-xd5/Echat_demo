package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"echat/sdk/domain/entity"
)

type GroupRepo struct {
	DB *sqlx.DB
}

func NewGroupRepo(db *sqlx.DB) *GroupRepo { return &GroupRepo{DB: db} }

// ======================== 群聊基础 ========================

func (r *GroupRepo) SaveGroup(ctx context.Context, g *entity.GroupChat) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO group_chat (gid, group_name, manager_uid, group_avatar, group_intro)
		 VALUES (?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE group_name=VALUES(group_name), manager_uid=VALUES(manager_uid),
		 group_avatar=VALUES(group_avatar), group_intro=VALUES(group_intro)`,
		g.GID, g.GroupName, g.ManagerUID, g.GroupAvatar, g.GroupIntro)
	if err != nil {
		return fmt.Errorf("save group: %w", err)
	}
	return tx.Commit()
}

func (r *GroupRepo) FindGroupByGID(ctx context.Context, gid string) (*entity.GroupChat, error) {
	var g entity.GroupChat
	err := r.DB.GetContext(ctx, &g,
		`SELECT gid, group_name, manager_uid, group_avatar, group_intro,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time FROM group_chat WHERE gid = ?`, gid)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &g, err
}

func (r *GroupRepo) FindGroupsByGIDs(ctx context.Context, gids []string) (map[string]*entity.GroupChat, error) {
	if len(gids) == 0 {
		return nil, nil
	}
	query, args, _ := sqlx.In(
		`SELECT gid, group_name, manager_uid, group_avatar, group_intro,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time FROM group_chat WHERE gid IN (?)`, gids)
	var gs []entity.GroupChat
	if err := r.DB.SelectContext(ctx, &gs, r.DB.Rebind(query), args...); err != nil {
		return nil, err
	}
	m := make(map[string]*entity.GroupChat, len(gs))
	for i := range gs {
		m[gs[i].GID] = &gs[i]
	}
	return m, nil
}

func (r *GroupRepo) GetMemberCounts(ctx context.Context, gids []string) (map[string]int, error) {
	if len(gids) == 0 {
		return nil, nil
	}
	query, args, _ := sqlx.In(
		`SELECT gid, COUNT(*) FROM group_member WHERE gid IN (?) GROUP BY gid`, gids)
	rows, err := r.DB.QueryContext(ctx, r.DB.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]int)
	for rows.Next() {
		var gid string
		var cnt int
		if err := rows.Scan(&gid, &cnt); err != nil {
			return nil, err
		}
		m[gid] = cnt
	}
	return m, rows.Err()
}

func (r *GroupRepo) FindGroupsByOwner(ctx context.Context, ownerUID string) ([]*entity.GroupChat, error) {
	var gs []entity.GroupChat
	err := r.DB.SelectContext(ctx, &gs,
		`SELECT gid, group_name, manager_uid, group_avatar, group_intro,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time
		 FROM group_chat WHERE manager_uid = ?`, ownerUID)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.GroupChat, len(gs))
	for i := range gs {
		ptr[i] = &gs[i]
	}
	return ptr, nil
}

func (r *GroupRepo) FindGroupByName(ctx context.Context, name string) ([]*entity.GroupChat, error) {
	var gs []entity.GroupChat
	err := r.DB.SelectContext(ctx, &gs,
		`SELECT gid, group_name, manager_uid, group_avatar, group_intro,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time
		 FROM group_chat WHERE group_name LIKE ?`, name)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.GroupChat, len(gs))
	for i := range gs {
		ptr[i] = &gs[i]
	}
	return ptr, nil
}

func (r *GroupRepo) DeleteGroup(ctx context.Context, gid string) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx, `DELETE FROM group_chat WHERE gid = ?`, gid)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ======================== 成员管理 ========================

func (r *GroupRepo) SaveMember(ctx context.Context, m *entity.GroupMember) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO group_member (uid, gid, role, nickname, level, do_not_disturb, group_by, remark, is_pinned)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE role=VALUES(role), nickname=VALUES(nickname), level=VALUES(level),
		 do_not_disturb=VALUES(do_not_disturb), group_by=VALUES(group_by), remark=VALUES(remark), is_pinned=VALUES(is_pinned)`,
		m.UID, m.GID, string(m.Role), m.Nickname, m.Level, m.DoNotDisturb, m.GroupBy, m.Remark, m.IsPinned)
	if err != nil {
		return fmt.Errorf("save member: %w", err)
	}
	return tx.Commit()
}

func (r *GroupRepo) FindMember(ctx context.Context, gid, uid string) (*entity.GroupMember, error) {
	var m entity.GroupMember
	err := r.DB.GetContext(ctx, &m,
		`SELECT uid, gid, role, nickname, level, UNIX_TIMESTAMP(join_time)*1000 AS join_time,
		 do_not_disturb, group_by, remark, is_pinned
		 FROM group_member WHERE uid = ? AND gid = ?`, uid, gid)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &m, err
}

func (r *GroupRepo) FindMembersByGroup(ctx context.Context, gid string) ([]*entity.GroupMember, error) {
	var ms []entity.GroupMember
	err := r.DB.SelectContext(ctx, &ms,
		`SELECT uid, gid, role, nickname, level, UNIX_TIMESTAMP(join_time)*1000 AS join_time,
		 do_not_disturb, group_by, remark, is_pinned
		 FROM group_member WHERE gid = ?`, gid)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.GroupMember, len(ms))
	for i := range ms {
		ptr[i] = &ms[i]
	}
	return ptr, nil
}

func (r *GroupRepo) FindGroupsByUser(ctx context.Context, uid string) ([]*entity.GroupMember, error) {
	var ms []entity.GroupMember
	err := r.DB.SelectContext(ctx, &ms,
		`SELECT uid, gid, role, nickname, level, UNIX_TIMESTAMP(join_time)*1000 AS join_time,
		 do_not_disturb, group_by, remark, is_pinned
		 FROM group_member WHERE uid = ?`, uid)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.GroupMember, len(ms))
	for i := range ms {
		ptr[i] = &ms[i]
	}
	return ptr, nil
}

func (r *GroupRepo) UpdateMemberRole(ctx context.Context, role entity.Role, gid, uid string) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx,
		`UPDATE group_member SET role = ? WHERE gid = ? AND uid = ?`, string(role), gid, uid)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *GroupRepo) RemoveMember(ctx context.Context, gid, uid string) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx, `DELETE FROM group_member WHERE uid = ? AND gid = ?`, uid, gid)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *GroupRepo) GetMemberCount(ctx context.Context, gid string) (int, error) {
	var count int
	err := r.DB.GetContext(ctx, &count, `SELECT COUNT(*) FROM group_member WHERE gid = ?`, gid)
	return count, err
}

// ======================== 禁言管理 ========================

func (r *GroupRepo) AddMuteRecord(ctx context.Context, mute *entity.MuteRecord) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO mute_record (ban_id, gid, uid, mute_duration, start_time)
		 VALUES (?, ?, ?, ?, FROM_UNIXTIME(?))
		 ON DUPLICATE KEY UPDATE mute_duration=VALUES(mute_duration), start_time=VALUES(start_time)`,
		mute.BanID, mute.GID, mute.UID, mute.MuteDuration, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("add mute: %w", err)
	}
	return tx.Commit()
}

func (r *GroupRepo) FindMuteRecordsByGroup(ctx context.Context, gid string) ([]*entity.MuteRecord, error) {
	var rs []entity.MuteRecord
	err := r.DB.SelectContext(ctx, &rs,
		`SELECT ban_id, gid, uid, mute_duration, UNIX_TIMESTAMP(start_time)*1000 AS start_time
		 FROM mute_record WHERE gid = ?`, gid)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.MuteRecord, len(rs))
	for i := range rs {
		ptr[i] = &rs[i]
	}
	return ptr, nil
}

func (r *GroupRepo) FindMuteRecordByUser(ctx context.Context, gid, uid string) (*entity.MuteRecord, error) {
	var m entity.MuteRecord
	err := r.DB.GetContext(ctx, &m,
		`SELECT ban_id, gid, uid, mute_duration, UNIX_TIMESTAMP(start_time)*1000 AS start_time
		 FROM mute_record WHERE gid = ? AND uid = ?`, gid, uid)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &m, err
}

func (r *GroupRepo) RemoveMute(ctx context.Context, banID string) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx, `UPDATE mute_record SET mute_duration = 0 WHERE ban_id = ?`, banID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *GroupRepo) FindExpiredMuteRecords(ctx context.Context) ([]*entity.MuteRecord, error) {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return nil, fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()

	var rs []entity.MuteRecord
	err := tx.SelectContext(ctx, &rs,
		`SELECT ban_id, gid, uid, mute_duration, UNIX_TIMESTAMP(start_time)*1000 AS start_time
		 FROM mute_record WHERE start_time IS NOT NULL AND DATE_ADD(start_time, INTERVAL mute_duration SECOND) < NOW()`)
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM mute_record WHERE start_time IS NOT NULL AND DATE_ADD(start_time, INTERVAL mute_duration SECOND) < NOW()`); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	ptr := make([]*entity.MuteRecord, len(rs))
	for i := range rs {
		ptr[i] = &rs[i]
	}
	return ptr, nil
}

// ======================== 群申请 ========================

func (r *GroupRepo) SaveGroupRequest(ctx context.Context, req *entity.GroupJoinRequest) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO group_join_request (req_id, gid, applicant_uid, approver_uid, status, apply_text)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		req.ReqID, req.GID, req.ApplicantUID, req.ApproverUID, string(req.Status), req.ApplyText)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *GroupRepo) FindGroupRequestByID(ctx context.Context, reqID string) (*entity.GroupJoinRequest, error) {
	var req entity.GroupJoinRequest
	err := r.DB.GetContext(ctx, &req,
		`SELECT req_id, gid, applicant_uid, approver_uid, status, apply_text,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time, UNIX_TIMESTAMP(handle_time)*1000 AS handle_time
		 FROM group_join_request WHERE req_id = ?`, reqID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &req, err
}

func (r *GroupRepo) FindPendingRequestsByGroup(ctx context.Context, gid string) ([]*entity.GroupJoinRequest, error) {
	var reqs []entity.GroupJoinRequest
	err := r.DB.SelectContext(ctx, &reqs,
		`SELECT req_id, gid, applicant_uid, approver_uid, status, apply_text,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time
		 FROM group_join_request WHERE gid = ? AND status = 'pending'`, gid)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.GroupJoinRequest, len(reqs))
	for i := range reqs {
		ptr[i] = &reqs[i]
	}
	return ptr, nil
}

func (r *GroupRepo) FindAllRequestsByGroup(ctx context.Context, gid string) ([]*entity.GroupJoinRequest, error) {
	var reqs []entity.GroupJoinRequest
	err := r.DB.SelectContext(ctx, &reqs,
		`SELECT req_id, gid, applicant_uid, approver_uid, status, apply_text,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time
		 FROM group_join_request WHERE gid = ? ORDER BY create_time DESC`, gid)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.GroupJoinRequest, len(reqs))
	for i := range reqs {
		ptr[i] = &reqs[i]
	}
	return ptr, nil
}

func (r *GroupRepo) FindRequestsByUser(ctx context.Context, uid string) ([]*entity.GroupJoinRequest, error) {
	var reqs []entity.GroupJoinRequest
	err := r.DB.SelectContext(ctx, &reqs,
		`SELECT req_id, gid, applicant_uid, approver_uid, status, apply_text,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time
		 FROM group_join_request WHERE applicant_uid = ? ORDER BY create_time DESC`, uid)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.GroupJoinRequest, len(reqs))
	for i := range reqs {
		ptr[i] = &reqs[i]
	}
	return ptr, nil
}

func (r *GroupRepo) UpdateGroupRequestStatus(ctx context.Context, reqID string, status entity.ReqStatus, approverUID string, handleTime int64) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx,
		`UPDATE group_join_request SET status = ?, approver_uid = ?, handle_time = NOW() WHERE req_id = ?`,
		string(status), approverUID, reqID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ======================== 权限校验 ========================

func (r *GroupRepo) ValidateGroupMessagePermission(ctx context.Context, senderUID, gid string) error {
	m, err := r.FindMember(ctx, gid, senderUID)
	if err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("不是群成员")
	}

	mute, err := r.FindMuteRecordByUser(ctx, gid, senderUID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if mute != nil {
		if mute.MuteDuration == -1 {
			return fmt.Errorf("已被永久禁言")
		}
		if mute.MuteDuration > 0 && mute.StartTime != nil {
			endTime := *mute.StartTime + int64(mute.MuteDuration)*1000
			if time.Now().UnixMilli() < endTime {
				return fmt.Errorf("已被禁言")
			}
		}
	}

	return nil
}
