// Package auth 提供 Ticket 签发与验证。
package auth

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

// JWTSecret 签名密钥（环境变量 JWT_SECRET，默认仅开发环境使用）
var JWTSecret = func() []byte {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return []byte(s)
	}
	return []byte("echat-jwt-secret-key-2024")
}()

// Claims JWT 载荷
type Claims struct {
	UID      string `json:"uid"`
	Account  string `json:"account"`
	Platform string `json:"platform"`
	jwt.RegisteredClaims
}

// SignToken 签发 JWT Token，默认有效期 7 天。
func SignToken(uid, account, platform string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UID:      uid,
		Account:  account,
		Platform: platform,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

// ParseToken 解析并验证 JWT Token。
func ParseToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return JWTSecret, nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// ValidateTicket 本地验证 Ticket（签名+过期检查）。
func ValidateTicket(ticket string) (uid, platform string, err error) {
	claims, err := ParseToken(ticket)
	if err != nil {
		return "", "", fmt.Errorf("ticket invalid: %w", err)
	}
	return claims.UID, claims.Platform, nil
}

// CheckBlacklist 检查 uid 是否在 Redis 黑名单中（封号/踢人）。
func CheckBlacklist(ctx context.Context, rdb *redis.Client, uid string) (bool, error) {
	exists, err := rdb.SIsMember(ctx, "blacklist:users", uid).Result()
	if err != nil {
		return false, fmt.Errorf("check blacklist: %w", err)
	}
	return exists, nil
}

// HashPassword 使用 bcrypt 哈希密码。
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword 验证密码与 bcrypt 哈希是否匹配。
func VerifyPassword(hashed, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}
