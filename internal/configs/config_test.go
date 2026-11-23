package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// envKeys 是 config 读取的全部环境变量；每个用例开始前清空，避免本机干扰。
var envKeys = []string{
	"APP_ENV", "CONFIG_FILE",
	"JWT_SECRET", "ID_CARD_HMAC_KEY", "ADMIN_INIT_PASSWORD",
	"WECHAT_APPID", "WECHAT_SECRET", "WECHAT_MCHID", "WECHAT_MCH_SERIAL_NO",
	"WECHAT_PAY_PUBLIC_KEY_ID", "WECHAT_PAY_API_V3_KEY",
	"WECHAT_PAY_PRIVATE_KEY_FILE", "WECHAT_PAY_PUBLIC_KEY_FILE",
	// 已移除的旧配置：必须不再生效
	"DEBUG", "PAY_MOCK", "PORT", "DB_PATH", "UPLOAD_DIR", "LOG_LEVEL", "LOG_FORMAT",
	"SLOW_REQUEST_MS", "JWT_EXPIRE_SECONDS", "STATUS_TICK_SECONDS",
	"ORDER_EXPIRE_MINUTES", "REFUND_WATCHDOG_MINUTES",
	"WECHAT_PAY_KEY", "WECHAT_PAY_PRIVATE_KEY", "WECHAT_PAY_PUBLIC_KEY",
	"WECHAT_PAY_NOTIFY_URL", "WECHAT_REFUND_NOTIFY_URL",
}

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

// prepare 创建隔离工作目录与配置文件，设置始终必需的密钥，默认 APP_ENV=dev。
func prepare(t *testing.T, yaml string) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	clearEnv(t, envKeys...)
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	t.Setenv("CONFIG_FILE", path)
	t.Setenv("APP_ENV", EnvDev)
	setRequiredSecrets(t)
	return dir
}

func setRequiredSecrets(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "jwt")
	t.Setenv("ID_CARD_HMAC_KEY", "hmac")
	t.Setenv("WECHAT_APPID", "wx")
	t.Setenv("WECHAT_SECRET", "secret")
	t.Setenv("ADMIN_INIT_PASSWORD", "pass")
}

func writeSecretFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// setPaymentEnv 配置一整套真实支付材料（默认值可被调用方覆盖）。
func setPaymentEnv(t *testing.T, dir string) {
	t.Helper()
	keyPath := writeSecretFile(t, dir, "apiclient_key.pem",
		"-----BEGIN PRIVATE KEY-----\nMIIB\n-----END PRIVATE KEY-----\n")
	pubPath := writeSecretFile(t, dir, "wechatpay_pub_key.pem",
		"-----BEGIN PUBLIC KEY-----\nMIIB\n-----END PUBLIC KEY-----\n")
	t.Setenv("WECHAT_MCHID", "mch")
	t.Setenv("WECHAT_MCH_SERIAL_NO", "serial")
	t.Setenv("WECHAT_PAY_PUBLIC_KEY_ID", "PUB_KEY_ID_0114232134912410000000000000")
	t.Setenv("WECHAT_PAY_API_V3_KEY", strings.Repeat("k", 32))
	t.Setenv("WECHAT_PAY_PRIVATE_KEY_FILE", keyPath)
	t.Setenv("WECHAT_PAY_PUBLIC_KEY_FILE", pubPath)
}

func TestLoadDefaultsAndDevMock(t *testing.T) {
	prepare(t, "{}\n")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Storage.DBPath != "data/dance-elite.db" {
		t.Errorf("DBPath = %q", cfg.Storage.DBPath)
	}
	if cfg.Storage.UploadDir != "uploads" {
		t.Errorf("UploadDir = %q", cfg.Storage.UploadDir)
	}
	if cfg.Log.Level != "info" || cfg.Log.Format != "json" {
		t.Errorf("log = %s/%s, want info/json", cfg.Log.Level, cfg.Log.Format)
	}
	if cfg.Auth.JWTExpireSeconds != 7200 {
		t.Errorf("JWTExpireSeconds = %d, want 7200", cfg.Auth.JWTExpireSeconds)
	}
	if cfg.Admin.InitUsername != "admin" {
		t.Errorf("InitUsername = %q, want admin", cfg.Admin.InitUsername)
	}
	if cfg.Scheduler.StatusTickSeconds != 60 || cfg.Scheduler.OrderExpireMinutes != 15 || cfg.Scheduler.RefundWatchdogMinutes != 10 {
		t.Errorf("scheduler = %d/%d/%d", cfg.Scheduler.StatusTickSeconds, cfg.Scheduler.OrderExpireMinutes, cfg.Scheduler.RefundWatchdogMinutes)
	}
	if cfg.Payment.Mode != PaymentModeMock {
		t.Errorf("Payment.Mode = %q, want mock", cfg.Payment.Mode)
	}
	if cfg.Payment.MockReason == "" {
		t.Error("MockReason is empty")
	}
}

func TestYAMLValues(t *testing.T) {
	prepare(t, `
server:
  port: 9090
  slow_request_ms: 0
storage:
  db_path: /tmp/dance.db
  upload_dir: /tmp/uploads
log:
  level: warn
  format: json
auth:
  jwt_expire_seconds: 60
admin:
  init_username: root
scheduler:
  status_tick_seconds: 5
  order_expire_minutes: 3
  refund_watchdog_minutes: 4
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Port != 9090 || cfg.Server.SlowRequestMS != 0 {
		t.Errorf("server = %d/%d", cfg.Server.Port, cfg.Server.SlowRequestMS)
	}
	if cfg.Storage.DBPath != "/tmp/dance.db" || cfg.Storage.UploadDir != "/tmp/uploads" {
		t.Errorf("storage = %s/%s", cfg.Storage.DBPath, cfg.Storage.UploadDir)
	}
	if cfg.Log.Level != "warn" || cfg.Log.Format != "json" {
		t.Errorf("log = %s/%s", cfg.Log.Level, cfg.Log.Format)
	}
	if cfg.Auth.JWTExpireSeconds != 60 {
		t.Errorf("JWTExpireSeconds = %d", cfg.Auth.JWTExpireSeconds)
	}
	if cfg.Admin.InitUsername != "root" {
		t.Errorf("InitUsername = %q", cfg.Admin.InitUsername)
	}
	if cfg.Scheduler.StatusTickSeconds != 5 || cfg.Scheduler.OrderExpireMinutes != 3 || cfg.Scheduler.RefundWatchdogMinutes != 4 {
		t.Errorf("scheduler = %d/%d/%d", cfg.Scheduler.StatusTickSeconds, cfg.Scheduler.OrderExpireMinutes, cfg.Scheduler.RefundWatchdogMinutes)
	}
}

func TestDerivedNotifyURLs(t *testing.T) {
	prepare(t, `
server:
  public_base_url: https://api.example.com/
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.PublicBaseURL != "https://api.example.com" {
		t.Errorf("PublicBaseURL = %q, want trailing slash removed", cfg.Server.PublicBaseURL)
	}
	if cfg.Payment.NotifyURL != "https://api.example.com/api/v1/payments/wechat/notify" {
		t.Errorf("NotifyURL = %q", cfg.Payment.NotifyURL)
	}
	if cfg.Payment.RefundNotifyURL != "https://api.example.com/api/v1/payments/wechat/refund-notify" {
		t.Errorf("RefundNotifyURL = %q", cfg.Payment.RefundNotifyURL)
	}
}

func TestPaymentMaterialFromFiles(t *testing.T) {
	dir := prepare(t, `
server:
  public_base_url: https://api.example.com
`)
	setPaymentEnv(t, dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Payment.Mode != PaymentModeWechat {
		t.Fatalf("Payment.Mode = %q, want wechat (reason: %s)", cfg.Payment.Mode, cfg.Payment.MockReason)
	}
	if !strings.Contains(cfg.Secrets.WechatPayPrivateKeyPEM, "BEGIN PRIVATE KEY") {
		t.Errorf("private key PEM = %q", cfg.Secrets.WechatPayPrivateKeyPEM)
	}
	if !strings.Contains(cfg.Secrets.WechatPayPublicKeyPEM, "BEGIN PUBLIC KEY") {
		t.Errorf("public key PEM = %q", cfg.Secrets.WechatPayPublicKeyPEM)
	}
}

func TestDevFallsBackToMockWhenKeyFileUnreadable(t *testing.T) {
	dir := prepare(t, "{}\n")
	setPaymentEnv(t, dir)
	missing := filepath.Join(dir, "missing.pem")
	t.Setenv("WECHAT_PAY_PRIVATE_KEY_FILE", missing)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Payment.Mode != PaymentModeMock {
		t.Fatalf("Payment.Mode = %q, want mock", cfg.Payment.Mode)
	}
	if !strings.Contains(cfg.Payment.MockReason, "读取失败") || !strings.Contains(cfg.Payment.MockReason, missing) {
		t.Errorf("MockReason = %q, want unreadable file with path", cfg.Payment.MockReason)
	}
}

func TestDevFallsBackToMockWhenAPIv3KeyInvalid(t *testing.T) {
	dir := prepare(t, "{}\n")
	setPaymentEnv(t, dir)
	t.Setenv("WECHAT_PAY_API_V3_KEY", strings.Repeat("k", 31))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Payment.Mode != PaymentModeMock {
		t.Fatalf("Payment.Mode = %q, want mock", cfg.Payment.Mode)
	}
	if !strings.Contains(cfg.Payment.MockReason, "32 字节") {
		t.Errorf("MockReason = %q", cfg.Payment.MockReason)
	}
}

func TestProdRequiresPaymentMaterial(t *testing.T) {
	prepare(t, `
server:
  public_base_url: https://api.example.com
`)
	t.Setenv("APP_ENV", EnvProd)

	_, err := Load()
	if err == nil {
		t.Fatal("Load should fail in prod without payment material")
	}
	for _, want := range []string{"WECHAT_MCHID", "不会回退 mock"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestProdRejectsUnreadableKeyFile(t *testing.T) {
	dir := prepare(t, `
server:
  public_base_url: https://api.example.com
`)
	setPaymentEnv(t, dir)
	t.Setenv("APP_ENV", EnvProd)
	t.Setenv("WECHAT_PAY_PRIVATE_KEY_FILE", filepath.Join(dir, "missing.pem"))

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "读取失败") {
		t.Fatalf("Load error = %v, want unreadable key file error", err)
	}
}

func TestProdRequiresPublicBaseURL(t *testing.T) {
	dir := prepare(t, "{}\n")
	setPaymentEnv(t, dir)
	t.Setenv("APP_ENV", EnvProd)

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "public_base_url") {
		t.Fatalf("Load error = %v, want public_base_url error", err)
	}
}

func TestProdRequiresHTTPSBaseURL(t *testing.T) {
	dir := prepare(t, `
server:
  public_base_url: http://api.example.com
`)
	setPaymentEnv(t, dir)
	t.Setenv("APP_ENV", EnvProd)

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("Load error = %v, want https error", err)
	}
}

func TestAlwaysRequiredSecrets(t *testing.T) {
	prepare(t, "{}\n")
	for _, key := range []string{"JWT_SECRET", "ID_CARD_HMAC_KEY", "WECHAT_APPID", "WECHAT_SECRET", "ADMIN_INIT_PASSWORD"} {
		t.Setenv(key, "")
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load should fail without required secrets")
	}
	for _, want := range []string{"JWT_SECRET", "ID_CARD_HMAC_KEY", "WECHAT_APPID", "WECHAT_SECRET", "ADMIN_INIT_PASSWORD"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not list %s", err, want)
		}
	}
}

func TestInvalidAppEnv(t *testing.T) {
	prepare(t, "{}\n")
	t.Setenv("APP_ENV", "staging")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "APP_ENV") {
		t.Fatalf("Load error = %v, want APP_ENV error", err)
	}
}

func TestInlinePEMIsNotSupported(t *testing.T) {
	dir := prepare(t, "{}\n")
	setPaymentEnv(t, dir)
	t.Setenv("WECHAT_PAY_PRIVATE_KEY_FILE", "")
	t.Setenv("WECHAT_PAY_PRIVATE_KEY", "inline-pem")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Payment.Mode != PaymentModeMock {
		t.Fatalf("Payment.Mode = %q, want mock", cfg.Payment.Mode)
	}
	if !strings.Contains(cfg.Payment.MockReason, "WECHAT_PAY_PRIVATE_KEY_FILE") {
		t.Errorf("MockReason = %q, want missing file env", cfg.Payment.MockReason)
	}
}

func TestInvalidYAMLValues(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{"port", "server:\n  port: 0\n", "server.port"},
		{"slow request", "server:\n  slow_request_ms: -1\n", "slow_request_ms"},
		{"log level", "log:\n  level: verbose\n", "log.level"},
		{"log format", "log:\n  format: text\n", "log.format"},
		{"jwt expire", "auth:\n  jwt_expire_seconds: 0\n", "jwt_expire_seconds"},
		{"empty admin username", "admin:\n  init_username: \"\"\n", "init_username"},
		{"status tick", "scheduler:\n  status_tick_seconds: 0\n", "status_tick_seconds"},
		{"order expire", "scheduler:\n  order_expire_minutes: 0\n", "order_expire_minutes"},
		{"refund watchdog", "scheduler:\n  refund_watchdog_minutes: 0\n", "refund_watchdog_minutes"},
		{"empty db path", "storage:\n  db_path: \"\"\n", "db_path"},
		{"empty upload dir", "storage:\n  upload_dir: \"\"\n", "upload_dir"},
		{"bad base url", "server:\n  public_base_url: api.example.com\n", "public_base_url"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prepare(t, tt.yaml)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load error = %v, want %s error", err, tt.want)
			}
		})
	}
}

func TestMissingConfigFile(t *testing.T) {
	prepare(t, "{}\n")
	path := filepath.Join(t.TempDir(), "nope.yaml")
	t.Setenv("CONFIG_FILE", path)

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("Load error = %v, want path %s", err, path)
	}
}

func TestLoadReadsDotEnv(t *testing.T) {
	dir := prepare(t, "{}\n")
	body := "JWT_SECRET=from_dotenv\nAPP_ENV=dev\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(body), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	os.Unsetenv("JWT_SECRET")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Secrets.JWTSecret != "from_dotenv" {
		t.Errorf("JWTSecret = %q, want value from .env", cfg.Secrets.JWTSecret)
	}
}

func TestRealEnvWinsOverDotEnv(t *testing.T) {
	dir := prepare(t, "{}\n")
	body := "JWT_SECRET=from_dotenv\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(body), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Secrets.JWTSecret != "jwt" {
		t.Errorf("JWTSecret = %q, want existing env to win", cfg.Secrets.JWTSecret)
	}
}
