package wxpay

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

// parseVerifyMaterial 解析微信支付公钥与公钥 ID（本项目唯一支持的验签方式）。
// 公钥 PEM 无法解析或公钥 ID 为空均视为配置错误，启动即失败。
func parseVerifyMaterial(opts ClientOptions) (*rsa.PublicKey, string, error) {
	publicKey, err := parsePublicKey(opts.PublicKeyPEM)
	if err != nil {
		return nil, "", err
	}
	keyID := strings.TrimSpace(opts.PublicKeyID)
	if keyID == "" {
		return nil, "", errors.New("缺少微信支付公钥 ID（WECHAT_PAY_PUBLIC_KEY_ID）")
	}
	return publicKey, keyID, nil
}

// parsePublicKey 解析微信支付公钥 PEM（PKIX "PUBLIC KEY"，兼容 PKCS#1
// "RSA PUBLIC KEY"）；环境变量中的字面量 "\n" 还原为真实换行。
func parsePublicKey(raw string) (*rsa.PublicKey, error) {
	normalized := strings.ReplaceAll(raw, `\n`, "\n")
	block, _ := pem.Decode([]byte(normalized))
	if block == nil {
		return nil, errors.New("PEM 块缺失（检查 WECHAT_PAY_PUBLIC_KEY_FILE 指向的公钥文件是否完整）")
	}
	if block.Type == "RSA PUBLIC KEY" {
		key, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("解析微信支付公钥: %w", err)
		}
		return key, nil
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析微信支付公钥: %w", err)
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("微信支付公钥不是 RSA 密钥")
	}
	return key, nil
}
