// Package secureid 为抽签等需要回显证件号的场景提供加密存储能力。
//
// 报名库只存 HMAC；抽签需要导出完整证件号，因此 Excel 导入时额外写入
// AES-256-GCM 密文。密钥从 ID_CARD_HMAC_KEY 派生（不同用途使用不同标签），
// 不新增环境变量；密文格式为 base64(nonce || ciphertext)。
package secureid

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

// deriveLabel 区分同一主密钥下的不同用途，避免 HMAC 与 AES 密钥相同。
const deriveLabel = "dance-elite/lottery-id/v1:"

// DeriveKey 从主密钥派生 32 字节 AES 密钥。
func DeriveKey(master string) []byte {
	sum := sha256.Sum256([]byte(deriveLabel + master))
	return sum[:]
}

// Encrypt 加密明文字符串；返回 base64 编码的 nonce+密文。
func Encrypt(key []byte, plain string) (string, error) {
	aead, err := newGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	sealed := aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt 解密 Encrypt 的输出；密文为空或损坏时返回错误。
func Decrypt(key []byte, encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode id ciphertext: %w", err)
	}
	aead, err := newGCM(key)
	if err != nil {
		return "", err
	}
	if len(raw) < aead.NonceSize() {
		return "", fmt.Errorf("id ciphertext too short")
	}
	nonce, ciphertext := raw[:aead.NonceSize()], raw[aead.NonceSize():]
	plain, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt id: %w", err)
	}
	return string(plain), nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new aes gcm: %w", err)
	}
	return aead, nil
}
