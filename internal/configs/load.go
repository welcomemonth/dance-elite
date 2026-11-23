package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// defaultConfigPath 是未设置 CONFIG_FILE 时读取的配置文件。
const defaultConfigPath = "config.yaml"

// Load 在启动时一次性确定全部配置，并在校验失败时直接返回错误。
//
// 顺序：默认值 → YAML（CONFIG_FILE，默认 config.yaml）→ 环境变量中的
// APP_ENV 与密钥。YAML 只承载非敏感参数；不提供按 key 的环境变量覆盖，
// 避免配置来源再次混用。
func Load() (*Config, error) {
	if err := loadDotEnv(); err != nil {
		return nil, err
	}
	cfg, err := loadYAML()
	if err != nil {
		return nil, err
	}
	cfg.Env = strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if cfg.Env == "" {
		cfg.Env = EnvProd
	}
	readSecrets(cfg)
	if err := validate(cfg); err != nil {
		return nil, err
	}
	if err := validateSMS(cfg); err != nil {
		return nil, err
	}
	if err := derivePayment(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// loadDotEnv 读取本地 .env（可选）；已存在的环境变量优先，生产由容器注入。
func loadDotEnv() error {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("读取 .env: %w", err)
	}
	return nil
}

// loadYAML 用 viper 读取非敏感配置，未出现在文件中的字段保留代码默认值。
func loadYAML() (*Config, error) {
	path := strings.TrimSpace(os.Getenv("CONFIG_FILE"))
	if path == "" {
		path = defaultConfigPath
	}
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件 %s: %w", path, err)
	}
	cfg := defaultConfig()
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件 %s: %w", path, err)
	}
	return cfg, nil
}

// defaultConfig 是 YAML 缺省时的安全默认值（不含任何密钥）。
func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:          8080,
			SlowRequestMS: 500,
		},
		Storage: StorageConfig{
			DBPath:    "data/dance-elite.db",
			UploadDir: "uploads",
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},
		Auth: AuthConfig{
			JWTExpireSeconds: 7200,
		},
		Admin: AdminConfig{
			InitUsername: "admin",
		},
		Scheduler: SchedulerConfig{
			StatusTickSeconds:     60,
			OrderExpireMinutes:    15,
			RefundWatchdogMinutes: 10,
		},
	}
}

// readSecrets 读取全部密钥与微信账户参数；只从环境变量读取，不读 YAML。
func readSecrets(cfg *Config) {
	cfg.Wechat = WechatConfig{
		AppID:          strings.TrimSpace(os.Getenv("WECHAT_APPID")),
		MchID:          strings.TrimSpace(os.Getenv("WECHAT_MCHID")),
		MchSerialNo:    strings.TrimSpace(os.Getenv("WECHAT_MCH_SERIAL_NO")),
		PayPublicKeyID: strings.TrimSpace(os.Getenv("WECHAT_PAY_PUBLIC_KEY_ID")),
	}
	cfg.Secrets = Secrets{
		JWTSecret:              os.Getenv("JWT_SECRET"),
		IDCardHMACKey:          os.Getenv("ID_CARD_HMAC_KEY"),
		WechatSecret:           os.Getenv("WECHAT_SECRET"),
		WechatPayAPIv3Key:      strings.TrimSpace(os.Getenv("WECHAT_PAY_API_V3_KEY")),
		WechatPrivateKeyFile:   strings.TrimSpace(os.Getenv("WECHAT_PAY_PRIVATE_KEY_FILE")),
		WechatPayPublicKeyFile: strings.TrimSpace(os.Getenv("WECHAT_PAY_PUBLIC_KEY_FILE")),
		AdminInitPassword:      os.Getenv("ADMIN_INIT_PASSWORD"),
		SMSAccessKeyID:         strings.TrimSpace(os.Getenv("SMS_ACCESS_KEY_ID")),
		SMSAccessKeySecret:     strings.TrimSpace(os.Getenv("SMS_ACCESS_KEY_SECRET")),
		SMSSignName:            strings.TrimSpace(os.Getenv("SMS_SIGN_NAME")),
	}
}

// validateSMS 校验短信凭据：允许整体不配置（功能关闭），但不允许只配一部分
// （易造成静默失败）；mock 模式忽略凭据。
func validateSMS(cfg *Config) error {
	if cfg.SMS.Mock {
		return nil
	}
	missing := 0
	for _, value := range []string{
		cfg.Secrets.SMSAccessKeyID, cfg.Secrets.SMSAccessKeySecret, cfg.Secrets.SMSSignName,
	} {
		if value == "" {
			missing++
		}
	}
	if missing > 0 && missing < 3 {
		return fmt.Errorf("短信配置不完整：SMS_ACCESS_KEY_ID、SMS_ACCESS_KEY_SECRET、SMS_SIGN_NAME 需同时提供，或在 config.yaml 设置 sms.mock=true")
	}
	return nil
}
