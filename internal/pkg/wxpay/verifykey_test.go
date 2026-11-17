package wxpay

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"
	"testing"
)

// testPublicKeyID 是测试用微信支付公钥 ID（商户平台形态：PUB_KEY_ID_ + 数字）。
const testPublicKeyID = "PUB_KEY_ID_0114232134912410000000000000"

// testPublicKeyPEM 把测试密钥对的公钥编码为 PKIX "PUBLIC KEY" PEM。
func testPublicKeyPEM(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

// TestParseNotify_PublicKeyMode 验证回调验签用公钥 ID 比对 Wechatpay-Serial。
func TestParseNotify_PublicKeyMode(t *testing.T) {
	keyPEM, key := testKey(t)
	client := testClient(t, keyPEM, key)
	if client.verifySerial != testPublicKeyID {
		t.Fatalf("verifySerial = %q, want %q", client.verifySerial, testPublicKeyID)
	}

	body, headers := signedNotify(t, client, key, testTransactionJSON)
	tx, err := client.ParseNotify(headers, body)
	if err != nil {
		t.Fatalf("ParseNotify: %v", err)
	}
	if tx.OutTradeNo != "2027010109300012345678" || tx.AmountTotal != 98000 {
		t.Fatalf("tx = %+v", tx)
	}

	// 公钥 ID 不匹配（例如误配成商户证书序列号）必须拒绝。
	bad := headers
	bad.Serial = "267CF456C3E8FF27E5F0E576E00AF050128A5043"
	if _, err := client.ParseNotify(bad, body); !errors.Is(err, ErrInvalidNotify) {
		t.Fatalf("err = %v, want ErrInvalidNotify", err)
	}
}

// TestParsePublicKey_PKCS1 验证兼容 PKCS#1 "RSA PUBLIC KEY" 与字面量 \n。
func TestParsePublicKey_PKCS1(t *testing.T) {
	_, key := testKey(t)
	pkcs1PEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&key.PublicKey),
	}))
	parsed, err := parsePublicKey(strings.ReplaceAll(pkcs1PEM, "\n", `\n`))
	if err != nil {
		t.Fatalf("parse PKCS#1 public key: %v", err)
	}
	if parsed.N.Cmp(key.PublicKey.N) != 0 || parsed.E != key.PublicKey.E {
		t.Fatal("parsed public key does not match")
	}
}

// TestNewClient_VerifyMaterialErrors 验证公钥与公钥 ID 配置错误在构造时即失败。
func TestNewClient_VerifyMaterialErrors(t *testing.T) {
	keyPEM, key := testKey(t)
	pubPEM := testPublicKeyPEM(t, key)
	base := ClientOptions{
		AppID:         "wx-appid",
		MchID:         "1900000109",
		SerialNo:      "MERCHANT-SERIAL-1",
		PrivateKeyPEM: keyPEM,
		APIv3Key:      testAPIv3Key,
	}

	cases := []struct {
		name    string
		mutate  func(opts *ClientOptions)
		wantErr string
	}{
		{"缺公钥 ID", func(o *ClientOptions) {
			o.PublicKeyPEM = pubPEM
		}, "WECHAT_PAY_PUBLIC_KEY_ID"},
		{"公钥不是合法 PEM", func(o *ClientOptions) {
			o.PublicKeyPEM = "not-a-pem"
			o.PublicKeyID = testPublicKeyID
		}, "WECHAT_PAY_PUBLIC_KEY"},
		{"缺公钥", func(o *ClientOptions) {
			o.PublicKeyID = testPublicKeyID
		}, "WECHAT_PAY_PUBLIC_KEY"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := base
			tc.mutate(&opts)
			_, err := NewClient(opts)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("NewClient error = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}
