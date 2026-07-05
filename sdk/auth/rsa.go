package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"sync"
)

var (
	rsaOnce      sync.Once
	rsaKey       *rsa.PrivateKey
	publicPEM    string
)

// InitRSA 初始化 RSA 密钥对（2048 位），仅执行一次。
func InitRSA() error {
	var initErr error
	rsaOnce.Do(func() {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			initErr = fmt.Errorf("生成 RSA 密钥对失败: %w", err)
			return
		}
		rsaKey = key

		pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
		if err != nil {
			initErr = fmt.Errorf("序列化公钥失败: %w", err)
			return
		}
		publicPEM = string(pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: pubBytes,
		}))
	})
	return initErr
}

// GetPublicKeyPEM 返回 PEM 格式的 RSA 公钥。
func GetPublicKeyPEM() string {
	return publicPEM
}

// DecryptPassword 解密客户端用公钥加密的密码。
// 输入: base64(RSA_OAEP_SHA256_encrypt(password))
// 输出: 明文密码
func DecryptPassword(encryptedB64 string) (string, error) {
	if rsaKey == nil {
		return "", fmt.Errorf("RSA 密钥未初始化")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedB64)
	if err != nil {
		return "", fmt.Errorf("base64 解码失败: %w", err)
	}
	plaintext, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, rsaKey, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("RSA 解密失败: %w", err)
	}
	return string(plaintext), nil
}
