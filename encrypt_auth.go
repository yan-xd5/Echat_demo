//go:build ignore
// 加密注册/登录测试：获取公钥 → RSA-OAEP 加密 → 注册 → 登录
// 运行: cd g:/CodeLearning/Go/echat && go run encrypt_test.go

package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
)

func main() {
	// ① 获取公钥
	resp, err := http.Get("http://127.0.0.1:9001/api/v1/auth/public-key")
	if err != nil {
		log.Fatal("❌ 获取公钥失败:", err)
	}
	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()

	pubPEM := body["public_key"]
	fmt.Println("✅ 公钥已获取")

	// ② 解析公钥
	block, _ := pem.Decode([]byte(pubPEM))
	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		log.Fatal("❌ 解析公钥失败:", err)
	}
	rsaPub := pubKey.(*rsa.PublicKey)

	// ③ 加密密码
	password := "123456"
	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, rsaPub, []byte(password), nil)
	if err != nil {
		log.Fatal("❌ 加密失败:", err)
	}
	encPassword := base64.StdEncoding.EncodeToString(ciphertext)
	fmt.Println("✅ 密码已加密:", encPassword[:30]+"...")

	// ④ 注册
	regBody, _ := json.Marshal(map[string]string{
		"account":  "rsa_user",
		"password": encPassword,
		"username": "RSAUser",
	})
	resp, err = http.Post("http://127.0.0.1:9001/api/v1/user/register",
		"application/json", bytes.NewReader(regBody))
	if err != nil {
		log.Fatal("❌ 注册失败:", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	fmt.Println("✅ 注册: HTTP", resp.StatusCode)

	// ⑤ 登录（新加密一个密码，因为每次 OAEP 密文不同）
	ciphertext2, _ := rsa.EncryptOAEP(sha256.New(), rand.Reader, rsaPub, []byte(password), nil)
	encPassword2 := base64.StdEncoding.EncodeToString(ciphertext2)

	loginBody, _ := json.Marshal(map[string]string{
		"account":  "rsa_user",
		"password": encPassword2,
	})
	resp, err = http.Post("http://127.0.0.1:9001/api/v1/user/login",
		"application/json", bytes.NewReader(loginBody))
	if err != nil {
		log.Fatal("❌ 登录失败:", err)
	}
	var loginResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&loginResp)
	resp.Body.Close()

	if token, ok := loginResp["token"]; ok {
		fmt.Println("✅ 登录成功！Token:", token.(string)[:50]+"...")
	} else {
		fmt.Println("❌ 登录响应:", loginResp)
	}
}
