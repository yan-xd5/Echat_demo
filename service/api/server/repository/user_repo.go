package repository

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

// UserRow 对应数据库 user 表
type UserRow struct {
	UID        string  `db:"uid"`
	Username   string  `db:"username"`
	Account    string  `db:"account"`
	Password   string  `db:"password"`
	Gender     string  `db:"gender"`
	Region     *string `db:"region"`     // 可为 NULL
	Email      *string `db:"email"`      // 可为 NULL
	Avatar     *string `db:"avatar"`     // 可为 NULL
	Bio        *string `db:"bio"`        // 可为 NULL
	CreateTime string  `db:"create_time"` // timestamp 类型
}

// UserRepo 用户数据访问层
type UserRepo struct {
	DB *sqlx.DB
}

// NewUserRepo 创建 UserRepo
func NewUserRepo(db *sqlx.DB) *UserRepo {
	return &UserRepo{DB: db}
}

// FindByAccount 根据账号查询用户（登录用）
func (r *UserRepo) FindByAccount(account string) (*UserRow, error) {
	var user UserRow
	err := r.DB.Get(&user,
		"SELECT uid, username, account, password, gender, region, email, avatar, bio, create_time FROM user WHERE account = ?",
		account)
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	return &user, nil
}

// FindByUID 根据 uid 查询用户
func (r *UserRepo) FindByUID(uid string) (*UserRow, error) {
	var user UserRow
	err := r.DB.Get(&user,
		"SELECT uid, username, account, password, gender, region, email, avatar, bio, create_time FROM user WHERE uid = ?",
		uid)
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	return &user, nil
}

// CreateUser 创建新用户
func (r *UserRepo) CreateUser(user *UserRow) error {
	_, err := r.DB.Exec(
		`INSERT INTO user (uid, username, account, password, gender, region, email, avatar, bio)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.UID, user.Username, user.Account, user.Password,
		user.Gender, user.Region, user.Email, user.Avatar, user.Bio)
	if err != nil {
		return fmt.Errorf("创建用户失败: %w", err)
	}
	return nil
}
