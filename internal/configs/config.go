// Package config 负责在启动时确定全部运行配置。
//
// 配置来源分为两类，互不混用：
//   - 非敏感运行参数：viper 读取 YAML（默认 config.yaml，可用 CONFIG_FILE 指定）；
//   - 密钥与微信账户参数：只来自环境变量（本地由 godotenv 读取 .env）。
//
// APP_ENV=dev/prod 标记运行环境：dev 在支付密钥缺失或不可读时自动回退
// mock；prod 要求支付材料完整，否则启动失败。不存在 PAY_MOCK / DEBUG 开关。
package config

// EnvDev / EnvProd 是 APP_ENV 的合法取值。
const (
	EnvDev  = "dev"
	EnvProd = "prod"
)

// 支付模式：由支付材料是否完整推导，不可手工配置。
const (
	PaymentModeWechat = "wechat"
	PaymentModeMock   = "mock"
)

// Config 是全部运行配置。非敏感字段来自 YAML，密钥字段来自环境变量。
type Config struct {
	// Env 是运行环境（APP_ENV），取值 dev/prod，默认 prod（安全的失败方向）。
	Env string

	Server    ServerConfig
	Storage   StorageConfig
	Log       LogConfig
	Auth      AuthConfig
	Admin     AdminConfig
	Scheduler SchedulerConfig
	SMS       SMSConfig

	// Wechat 是微信账户标识（非密钥，但统一经环境变量注入）。
	Wechat WechatConfig

	// Secrets 是密钥材料，只经环境变量注入。
	Secrets Secrets

	// Payment 是启动时推导出的支付通道状态，不可从配置直接设置。
	Payment PaymentConfig
}

// ServerConfig 是 HTTP 服务配置。
type ServerConfig struct {
	Port          int    `mapstructure:"port"`
	PublicBaseURL string `mapstructure:"public_base_url"` // 对外地址（音频审核/抽签外链、微信支付回调），末尾斜杠已去除
	SlowRequestMS int    `mapstructure:"slow_request_ms"` // 慢请求阈值（毫秒），0 关闭
}

// StorageConfig 是本地存储路径。
type StorageConfig struct {
	DBPath    string `mapstructure:"db_path"`
	UploadDir string `mapstructure:"upload_dir"`
}

// LogConfig 是日志配置；Level 同时决定 Gin 运行模式（debug → debug，否则 release）。
type LogConfig struct {
	Level  string `mapstructure:"level"`  // debug/info/warn/error
	Format string `mapstructure:"format"` // json/console
}

// AuthConfig 是登录态配置。
type AuthConfig struct {
	JWTExpireSeconds int `mapstructure:"jwt_expire_seconds"`
}

// AdminConfig 是后台账号初始化配置。
type AdminConfig struct {
	InitUsername string `mapstructure:"init_username"`
}

// SchedulerConfig 是定时任务配置。
type SchedulerConfig struct {
	StatusTickSeconds     int `mapstructure:"status_tick_seconds"`
	OrderExpireMinutes    int `mapstructure:"order_expire_minutes"`
	RefundWatchdogMinutes int `mapstructure:"refund_watchdog_minutes"`
}

// SMSConfig 是抽签短信的非敏感开关；阿里云凭据只从环境变量读取。
//
// Mock=true 时不调用阿里云，只记录脱敏日志（本地联调）。凭据三项齐全
// 才启用真实发送；只配一部分视为配置错误。
type SMSConfig struct {
	Mock bool `mapstructure:"mock"`
}

// WechatConfig 是微信账户标识（全部来自环境变量）。
type WechatConfig struct {
	AppID          string
	MchID          string
	MchSerialNo    string
	PayPublicKeyID string
}

// Secrets 是密钥材料（全部来自环境变量）；PEM 内容由 *_FILE 指向的文件读取。
type Secrets struct {
	JWTSecret         string
	IDCardHMACKey     string
	WechatSecret      string
	WechatPayAPIv3Key string
	// 微信支付密钥只接受文件路径（只读挂载），不接受内联 PEM 文本。
	WechatPrivateKeyFile   string
	WechatPayPublicKeyFile string
	// WechatPayPrivateKeyPEM / WechatPayPublicKeyPEM 是上述文件的内容，
	// 启动时读取，不落库、不写日志。
	WechatPayPrivateKeyPEM string
	WechatPayPublicKeyPEM  string
	AdminInitPassword      string
	// 阿里云短信凭据（抽签提醒/结果通知）；三项齐全才启用。
	SMSAccessKeyID     string
	SMSAccessKeySecret string
	SMSSignName        string
}

// PaymentConfig 是支付通道的推导结果。
type PaymentConfig struct {
	Mode            string // wechat / mock
	MockReason      string // mock 时说明回退原因，启动日志输出
	NotifyURL       string // = PublicBaseURL + /api/v1/payments/wechat/notify
	RefundNotifyURL string // = PublicBaseURL + /api/v1/payments/wechat/refund-notify
}

// IsDev 报告当前是否为本地开发环境。
func (c *Config) IsDev() bool { return c.Env == EnvDev }
