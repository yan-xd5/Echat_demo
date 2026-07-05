package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// AuthChecker 消息权限校验实现。
type AuthChecker struct {
	DB *sqlx.DB
}

// NewAuthChecker 创建权限校验器。
func NewAuthChecker(db *sqlx.DB) *AuthChecker {
	return &AuthChecker{DB: db}
}

// CheckFriend 校验两人是否为好友且未被拉黑。
func (a *AuthChecker) CheckFriend(ctx context.Context, uid1, uid2 string) error {
	a1, b1 := uid1, uid2
	if a1 > b1 {
		a1, b1 = b1, a1
	}

	var fid string
	var black1, black2 int
	err := a.DB.QueryRowContext(ctx,
		`SELECT fid, is_blacklist, to_is_blacklist
		 FROM friends WHERE uid = ? AND to_uid = ?`,
		a1, b1,
	).Scan(&fid, &black1, &black2)
	if err == sql.ErrNoRows {
		return fmt.Errorf("不是好友")
	}
	if err != nil {
		return fmt.Errorf("查好友关系失败: %w", err)
	}

	if uid1 == a1 && black2 != 0 {
		return fmt.Errorf("已被对方拉黑")
	}
	if uid1 == b1 && black1 != 0 {
		return fmt.Errorf("已被对方拉黑")
	}

	return nil
}

// EnsurePrivateChat 确保私聊会话存在，返回 pid。
// 如果 private_chat 记录不存在则创建（INSERT IGNORE），uid 自动排序。
func (a *AuthChecker) EnsurePrivateChat(ctx context.Context, uid1, uid2 string) (string, error) {
	a1, b1 := uid1, uid2
	if a1 > b1 {
		a1, b1 = b1, a1
	}

	// 先查已有记录
	var pid string
	err := a.DB.QueryRowContext(ctx,
		`SELECT pid FROM private_chat WHERE uid1 = ? AND uid2 = ?`, a1, b1,
	).Scan(&pid)
	if err == nil {
		return pid, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("查 private_chat 失败: %w", err)
	}

	// 不存在则创建（pid = sessionID 格式，与 sessionID 一致）
	pid = a1 + "_" + b1
	_, err = a.DB.ExecContext(ctx,
		`INSERT IGNORE INTO private_chat (pid, uid1, uid2) VALUES (?, ?, ?)`,
		pid, a1, b1,
	)
	if err != nil {
		return "", fmt.Errorf("创建 private_chat 失败: %w", err)
	}
	return pid, nil
}

// GetGroupMemberUIDs 获取群所有成员 UID 列表（转发用）。
func (a *AuthChecker) GetGroupMemberUIDs(ctx context.Context, gid string) ([]string, error) {
	rows, err := a.DB.QueryContext(ctx,
		`SELECT uid FROM group_member WHERE gid = ?`, gid)
	if err != nil {
		return nil, fmt.Errorf("查群成员列表失败: %w", err)
	}
	defer rows.Close()

	var uids []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		uids = append(uids, uid)
	}
	return uids, rows.Err()
}

// CheckGroupMember 校验用户是否为群成员且未被禁言。
func (a *AuthChecker) CheckGroupMember(ctx context.Context, uid, gid string) error {
	var role string
	err := a.DB.QueryRowContext(ctx,
		`SELECT role FROM group_member WHERE uid = ? AND gid = ?`,
		uid, gid,
	).Scan(&role)
	if err == sql.ErrNoRows {
		return fmt.Errorf("不是群成员")
	}
	if err != nil {
		return fmt.Errorf("查群成员失败: %w", err)
	}

	var muteDuration int
	var startTime int64
	err = a.DB.QueryRowContext(ctx,
		`SELECT mute_duration, UNIX_TIMESTAMP(start_time)
		 FROM mute_record WHERE gid = ? AND uid = ?`,
		gid, uid,
	).Scan(&muteDuration, &startTime)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("查禁言状态失败: %w", err)
	}
	if err == nil {
		if muteDuration == -1 {
			return fmt.Errorf("已被永久禁言")
		}
		if startTime+int64(muteDuration) > time.Now().Unix() {
			return fmt.Errorf("已被禁言，剩余 %d 秒", startTime+int64(muteDuration)-time.Now().Unix())
		}
	}

	return nil
}
