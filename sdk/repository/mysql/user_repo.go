package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"

	"echat/sdk/domain/entity"
)

// UserRepo 用户数据访问实现（对应 chat UserRepository + OnlineRepository）。
type UserRepo struct {
	DB *sqlx.DB
}

func NewUserRepo(db *sqlx.DB) *UserRepo { return &UserRepo{DB: db} }

// ======================== 用户查询 ========================

func (r *UserRepo) FindUserByUID(ctx context.Context, uid string) (*entity.User, error) {
	var u entity.User
	err := r.DB.GetContext(ctx, &u,
		`SELECT uid, account, username, password, gender, region, email, avatar, bio,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time FROM user WHERE uid = ?`, uid)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("用户不存在: %s", uid)
	}
	return &u, err
}

func (r *UserRepo) FindUsersByUIDs(ctx context.Context, uids []string) (map[string]*entity.User, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	query, args, _ := sqlx.In(
		`SELECT uid, account, username, password, gender, region, email, avatar, bio,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time FROM user WHERE uid IN (?)`, uids)
	var users []entity.User
	if err := r.DB.SelectContext(ctx, &users, r.DB.Rebind(query), args...); err != nil {
		return nil, err
	}
	m := make(map[string]*entity.User)
	for i := range users {
		m[users[i].UID] = &users[i]
	}
	return m, nil
}

func (r *UserRepo) FindUserByAccount(ctx context.Context, account string) (*entity.User, error) {
	var u entity.User
	err := r.DB.GetContext(ctx, &u,
		`SELECT uid, account, username, password, gender, region, email, avatar, bio,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time FROM user WHERE account = ?`, account)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("用户不存在: %s", account)
	}
	return &u, err
}

func (r *UserRepo) FindUserByUsername(ctx context.Context, username string) ([]*entity.User, error) {
	var users []entity.User
	err := r.DB.SelectContext(ctx, &users,
		`SELECT uid, account, username, password, gender, region, email, avatar, bio,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time FROM user WHERE username LIKE ?`,
		"%"+username+"%")
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.User, len(users))
	for i := range users {
		ptr[i] = &users[i]
	}
	return ptr, nil
}

func (r *UserRepo) FindUserByRegion(ctx context.Context, region string) ([]*entity.User, error) {
	var users []entity.User
	err := r.DB.SelectContext(ctx, &users,
		`SELECT uid, account, username, password, gender, region, email, avatar, bio,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time FROM user WHERE region = ?`, region)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.User, len(users))
	for i := range users {
		ptr[i] = &users[i]
	}
	return ptr, nil
}

func (r *UserRepo) FindUserByCreateTimeRange(ctx context.Context, start, end int64) ([]*entity.User, error) {
	var users []entity.User
	err := r.DB.SelectContext(ctx, &users,
		`SELECT uid, account, username, password, gender, region, email, avatar, bio,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time FROM user
		 WHERE create_time BETWEEN FROM_UNIXTIME(?/1000) AND FROM_UNIXTIME(?/1000) ORDER BY create_time DESC`,
		start, end)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.User, len(users))
	for i := range users {
		ptr[i] = &users[i]
	}
	return ptr, nil
}

// ======================== 用户更新 ========================

func (r *UserRepo) InsertUser(ctx context.Context, user *entity.User) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO user (uid, account, password, username, gender, region, email, avatar, bio)
		 VALUES (?, ?, ?, ?, COALESCE(NULLIF(?, ''), 'other'), ?, ?, ?, ?)`,
		user.UID, user.Account, user.Password, user.Username, user.Gender, user.Region, user.Email, user.Avatar, user.Bio)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return tx.Commit()
}

func (r *UserRepo) SaveUser(ctx context.Context, user *entity.User) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx,
		`INSERT INTO user (uid, account, password, username, gender, region, email, avatar, bio)
		 VALUES (?, ?, ?, ?, COALESCE(NULLIF(?, ''), 'other'), ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		 account=VALUES(account), password=VALUES(password), username=VALUES(username),
		 gender=VALUES(gender), region=VALUES(region), email=VALUES(email),
		 avatar=VALUES(avatar), bio=VALUES(bio)`,
		user.UID, user.Account, user.Password, user.Username, user.Gender, user.Region, user.Email, user.Avatar, user.Bio)
	if err != nil {
		return fmt.Errorf("save user: %w", err)
	}
	return tx.Commit()
}

func (r *UserRepo) UpdateUser(ctx context.Context, user *entity.User) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
	if beginErr != nil {
		return fmt.Errorf("begin tx: %w", beginErr)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx,
		`UPDATE user SET username=?, gender=COALESCE(NULLIF(?, ''), gender), region=?, email=?, avatar=?, bio=? WHERE uid=?`,
		user.Username, user.Gender, user.Region, user.Email, user.Avatar, user.Bio, user.UID)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("用户不存在: %s", user.UID)
	}
	return tx.Commit()
}

func (r *UserRepo) DeleteUser(ctx context.Context, uid string) error {
	tx, beginErr := r.DB.BeginTxx(ctx, nil)
if beginErr != nil {
	return fmt.Errorf("begin tx: %w", beginErr)
}
	defer tx.Rollback()
	_, err := tx.ExecContext(ctx, `DELETE FROM user WHERE uid = ?`, uid)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return tx.Commit()
}

func (r *UserRepo) ExistsByAccount(ctx context.Context, account string) (bool, error) {
	var count int
	err := r.DB.GetContext(ctx, &count, `SELECT COUNT(*) FROM user WHERE account = ?`, account)
	return count > 0, err
}
