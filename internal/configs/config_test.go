package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// clearEnv 清空指定环境变量，测试结束后恢复原值。
func clearEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		old, existed := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
		if existed {
			t.Cleanup(func() { os.Setenv(key, old) })
		} else {
			t.Cleanup(func() { os.Unsetenv(key) })
		}
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t, "PORT", "DEBUG", "DB_PATH", "UPLOAD_DIR", "PAY_MOCK", "JWT_EXPIRE_SECONDS", "ADMIN_INIT_USERNAME")

	cfg, err := read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want 8080", cfg.Port)
	}
	if cfg.Debug {
		t.Error("Debug = true, want false")
	}
	if cfg.DBPath != "data/dance-elite.db" {
		t.Errorf("DBPath = %q", cfg.DBPath)
	}
	if cfg.UploadDir != "uploads" {
		t.Errorf("UploadDir = %q, want uploads", cfg.UploadDir)
	}
	if cfg.PayMock {
		t.Error("PayMock = true, want false")
	}
	if cfg.JWTExpireSeconds != 7200 {
		t.Errorf("JWTExpireSeconds = %d, want 7200", cfg.JWTExpireSeconds)
	}
	if cfg.AdminInitUsername != "admin" {
		t.Errorf("AdminInitUsername = %q, want admin", cfg.AdminInitUsername)
	}
}

func TestEnvOverridesDefaults(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DEBUG", "true")
	t.Setenv("DB_PATH", "/tmp/dance.db")
	t.Setenv("UPLOAD_DIR", "/data/uploads")
	t.Setenv("PAY_MOCK", "1")
	t.Setenv("WECHAT_APPID", "wx_from_env")
	t.Setenv("ID_CARD_HMAC_KEY", "hmac_from_env")
	t.Setenv("ADMIN_INIT_PASSWORD", "admin_from_env")

	cfg, err := read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want env value", cfg.Port)
	}
	if !cfg.Debug {
		t.Error("Debug = false, want true")
	}
	if cfg.DBPath != "/tmp/dance.db" {
		t.Errorf("DBPath = %q", cfg.DBPath)
	}
	if cfg.UploadDir != "/data/uploads" {
		t.Errorf("UploadDir = %q", cfg.UploadDir)
	}
	if !cfg.PayMock {
		t.Error("PayMock = false, want true")
	}
	if cfg.WechatAppID != "wx_from_env" {
		t.Errorf("WechatAppID = %q", cfg.WechatAppID)
	}
	if cfg.IDCardHMACKey != "hmac_from_env" {
		t.Errorf("IDCardHMACKey = %q", cfg.IDCardHMACKey)
	}
	if cfg.AdminInitPassword != "admin_from_env" {
		t.Errorf("AdminInitPassword = %q", cfg.AdminInitPassword)
	}
}

func TestInvalidBoolEnvReturnsError(t *testing.T) {
	t.Setenv("PAY_MOCK", "yes-please")

	_, err := read()
	if err == nil || !strings.Contains(err.Error(), "PAY_MOCK") {
		t.Fatalf("read error = %v, want invalid PAY_MOCK error", err)
	}
}

// TestLogDefaultsFollowDebug 验证日志级别/格式未配置时随 DEBUG 推导。
func TestLogDefaultsFollowDebug(t *testing.T) {
	clearEnv(t, "DEBUG", "LOG_LEVEL", "LOG_FORMAT")

	cfg, err := read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if cfg.LogLevel != "info" || cfg.LogFormat != "json" {
		t.Fatalf("prod defaults = %s/%s, want info/json", cfg.LogLevel, cfg.LogFormat)
	}

	t.Setenv("DEBUG", "true")
	cfg, err = read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if cfg.LogLevel != "debug" || cfg.LogFormat != "console" {
		t.Fatalf("debug defaults = %s/%s, want debug/console", cfg.LogLevel, cfg.LogFormat)
	}
}

// TestInvalidLogEnvRejected 验证非法日志配置启动即失败。
func TestInvalidLogEnvRejected(t *testing.T) {
	for _, key := range []string{"LOG_LEVEL", "LOG_FORMAT"} {
		t.Run(key, func(t *testing.T) {
			for k, v := range map[string]string{
				"JWT_SECRET": "jwt", "ID_CARD_HMAC_KEY": "hmac",
				"WECHAT_APPID": "wx", "WECHAT_SECRET": "secret", "ADMIN_INIT_PASSWORD": "pass",
				"PAY_MOCK": "true", key: "invalid-value",
			} {
				t.Setenv(k, v)
			}
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("Load error = %v, want invalid %s error", err, key)
			}
		})
	}
}

func TestInvalidIntEnvReturnsError(t *testing.T) {
	t.Setenv("JWT_EXPIRE_SECONDS", "two-hours")

	_, err := read()
	if err == nil || !strings.Contains(err.Error(), "JWT_EXPIRE_SECONDS") {
		t.Fatalf("read error = %v, want invalid JWT_EXPIRE_SECONDS error", err)
	}
}

func TestNonPositiveJWTExpireRejected(t *testing.T) {
	t.Setenv("JWT_EXPIRE_SECONDS", "0")
	t.Setenv("PAY_MOCK", "true")
	for key, value := range map[string]string{
		"JWT_SECRET": "jwt", "ID_CARD_HMAC_KEY": "hmac",
		"WECHAT_APPID": "wx", "WECHAT_SECRET": "secret", "ADMIN_INIT_PASSWORD": "pass",
	} {
		t.Setenv(key, value)
	}

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "JWT_EXPIRE_SECONDS") {
		t.Fatalf("Load error = %v, want non-positive JWT_EXPIRE_SECONDS error", err)
	}
}

// TestLoadFailsFastOnMissingSecrets 验证启动自检在缺项时返回错误并列出全部缺失项。
func TestLoadFailsFastOnMissingSecrets(t *testing.T) {
	required := []string{"JWT_SECRET", "ID_CARD_HMAC_KEY", "WECHAT_APPID", "WECHAT_SECRET", "ADMIN_INIT_PASSWORD"}
	clearEnv(t, required...)
	t.Setenv("PAY_MOCK", "true")

	_, err := Load()
	if err == nil {
		t.Fatal("Load should fail without required secrets")
	}
	for _, want := range required {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not list %s", err, want)
		}
	}
}

// TestLoadReadsDotEnv 验证本地 .env 会被读取，且已存在的环境变量优先。
func TestLoadReadsDotEnv(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	body := "PORT=8081\nJWT_SECRET=jwt\nID_CARD_HMAC_KEY=hmac\n" +
		"WECHAT_APPID=wx\nWECHAT_SECRET=secret\nADMIN_INIT_PASSWORD=pass\nPAY_MOCK=true\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(body), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	clearEnv(t, "PORT", "JWT_SECRET", "ID_CARD_HMAC_KEY", "WECHAT_APPID", "WECHAT_SECRET", "ADMIN_INIT_PASSWORD", "PAY_MOCK")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "8081" {
		t.Errorf("Port = %q, want value from .env", cfg.Port)
	}
	if cfg.JWTSecret != "jwt" {
		t.Errorf("JWTSecret = %q, want value from .env", cfg.JWTSecret)
	}
}

func TestRealEnvWinsOverDotEnv(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	body := "PORT=8081\nJWT_SECRET=jwt\nID_CARD_HMAC_KEY=hmac\n" +
		"WECHAT_APPID=wx\nWECHAT_SECRET=secret\nADMIN_INIT_PASSWORD=pass\nPAY_MOCK=true\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(body), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	clearEnv(t, "PORT", "JWT_SECRET", "ID_CARD_HMAC_KEY", "WECHAT_APPID", "WECHAT_SECRET", "ADMIN_INIT_PASSWORD", "PAY_MOCK")
	t.Setenv("PORT", "9090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want existing env to win", cfg.Port)
	}
}

func TestMissingSecretsOrder(t *testing.T) {
	got := (&Config{PayMock: true}).MissingSecrets()
	want := []string{"JWT_SECRET", "ID_CARD_HMAC_KEY", "WECHAT_APPID", "WECHAT_SECRET", "ADMIN_INIT_PASSWORD"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MissingSecrets = %v, want %v", got, want)
	}
}

func TestMissingSecretsAllPresent(t *testing.T) {
	cfg := &Config{
		JWTSecret:         "jwt",
		IDCardHMACKey:     "hmac",
		WechatAppID:       "wx",
		WechatSecret:      "secret",
		PayMock:           true,
		AdminInitPassword: "pass",
	}
	if got := cfg.MissingSecrets(); len(got) != 0 {
		t.Fatalf("MissingSecrets = %v, want none", got)
	}
}

func TestMissingSecretsRealPay(t *testing.T) {
	cfg := &Config{
		JWTSecret:         "jwt",
		IDCardHMACKey:     "hmac",
		WechatAppID:       "wx",
		WechatSecret:      "secret",
		PayMock:           false,
		AdminInitPassword: "pass",
	}
	got := cfg.MissingSecrets()
	if len(got) != 7 {
		t.Fatalf("MissingSecrets = %v, want 7 pay-related items", got)
	}
	for _, item := range got {
		if !strings.HasPrefix(item, "WECHAT_") {
			t.Errorf("unexpected missing item %q", item)
		}
	}
}

// writeSecretFile 写入测试用密钥文件并返回路径。
func writeSecretFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// TestWechatPEMFromFiles 验证 *_FILE 方式读取只读挂载的 PEM 内容。
func TestWechatPEMFromFiles(t *testing.T) {
	keyPEM := "-----BEGIN PRIVATE KEY-----\nMIIB\n-----END PRIVATE KEY-----\n"
	certPEM := "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"
	keyPath := writeSecretFile(t, "apiclient_key.pem", keyPEM)
	certPath := writeSecretFile(t, "platform_cert.pem", certPEM)
	clearEnv(t, "WECHAT_PAY_PRIVATE_KEY", "WECHAT_PAY_CERT", "WECHAT_PAY_PRIVATE_KEY_FILE", "WECHAT_PAY_CERT_FILE")
	t.Setenv("WECHAT_PAY_PRIVATE_KEY_FILE", keyPath)
	t.Setenv("WECHAT_PAY_CERT_FILE", certPath)

	cfg, err := read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if cfg.WechatPrivateKey != strings.TrimSpace(keyPEM) {
		t.Errorf("WechatPrivateKey = %q, want file content", cfg.WechatPrivateKey)
	}
	if cfg.WechatPrivateKeyFile != keyPath {
		t.Errorf("WechatPrivateKeyFile = %q, want %q", cfg.WechatPrivateKeyFile, keyPath)
	}
	if cfg.WechatPayCert != strings.TrimSpace(certPEM) {
		t.Errorf("WechatPayCert = %q, want file content", cfg.WechatPayCert)
	}
	if cfg.WechatPayCertFile != certPath {
		t.Errorf("WechatPayCertFile = %q, want %q", cfg.WechatPayCertFile, certPath)
	}
}

// TestWechatPEMFileConflictsWithInline 验证直接值与该文件路径不能同时配置。
func TestWechatPEMFileConflictsWithInline(t *testing.T) {
	t.Setenv("WECHAT_PAY_PRIVATE_KEY", "inline-pem")
	t.Setenv("WECHAT_PAY_PRIVATE_KEY_FILE", filepath.Join(t.TempDir(), "unused.pem"))

	_, err := read()
	if err == nil || !strings.Contains(err.Error(), "WECHAT_PAY_PRIVATE_KEY") {
		t.Fatalf("read error = %v, want conflict error", err)
	}
}

// TestWechatPEMFileUnreadable 验证密钥文件不存在时启动报错并带上路径。
func TestWechatPEMFileUnreadable(t *testing.T) {
	clearEnv(t, "WECHAT_PAY_CERT")
	path := filepath.Join(t.TempDir(), "missing.pem")
	t.Setenv("WECHAT_PAY_CERT_FILE", path)

	_, err := read()
	if err == nil || !strings.Contains(err.Error(), "WECHAT_PAY_CERT_FILE") || !strings.Contains(err.Error(), path) {
		t.Fatalf("read error = %v, want unreadable file error with path", err)
	}
}

// TestWechatPEMFileEmpty 验证空白密钥文件视为配置错误。
func TestWechatPEMFileEmpty(t *testing.T) {
	clearEnv(t, "WECHAT_PAY_CERT")
	path := writeSecretFile(t, "empty.pem", " \n")
	t.Setenv("WECHAT_PAY_CERT_FILE", path)

	_, err := read()
	if err == nil || !strings.Contains(err.Error(), "内容为空") {
		t.Fatalf("read error = %v, want empty file error", err)
	}
}

// TestLoadAcceptsFileOnlyPaySecrets 验证只配 *_FILE 时启动自检通过。
func TestLoadAcceptsFileOnlyPaySecrets(t *testing.T) {
	clearEnv(t, "WECHAT_PAY_PRIVATE_KEY", "WECHAT_PAY_CERT")
	keyPath := writeSecretFile(t, "key.pem", "key-pem")
	certPath := writeSecretFile(t, "cert.pem", "cert-pem")
	for key, value := range map[string]string{
		"JWT_SECRET":                  "jwt",
		"ID_CARD_HMAC_KEY":            "hmac",
		"WECHAT_APPID":                "wx",
		"WECHAT_SECRET":               "secret",
		"ADMIN_INIT_PASSWORD":         "pass",
		"PAY_MOCK":                    "false",
		"WECHAT_MCHID":                "mch",
		"WECHAT_PAY_KEY":              strings.Repeat("k", 32),
		"WECHAT_MCH_SERIAL_NO":        "serial",
		"WECHAT_PAY_NOTIFY_URL":       "https://example.com/notify",
		"WECHAT_REFUND_NOTIFY_URL":    "https://example.com/refund",
		"WECHAT_PAY_PRIVATE_KEY_FILE": keyPath,
		"WECHAT_PAY_CERT_FILE":        certPath,
	} {
		t.Setenv(key, value)
	}

	if _, err := Load(); err != nil {
		t.Fatalf("Load with secret files: %v", err)
	}
}
