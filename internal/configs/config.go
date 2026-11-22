// Package config 负责在启动时确定全部运行配置。
//
// 配置全部来自环境变量：本地开发由 godotenv 读取 .env（可选，已存在的
// 环境变量优先），生产由容器运行时注入。不存在 YAML 配置文件，密钥永远
// 不落文件入库；微信支付公钥与商户私钥还可只给文件路径（*_FILE），由启动
// 时读取只读挂载的 PEM。必需项缺失时启动直接失败。
package config

// Config 是全部运行配置。字段名与环境变量一一对应（见 load.go）。
type Config struct {
	// 服务
	Port      string
	Debug     bool
	DBPath    string
	UploadDir string
	// LotteryDir 是抽签 JSON 文件目录（一个赛事一个 <token>.json）。
	LotteryDir string
	PayMock    bool
	// 日志（T36）：级别 debug/info/warn/error、格式 json/console；
	// 未显式配置时随 DEBUG 推导（debug → debug+console，否则 info+json）。
	LogLevel      string
	LogFormat     string
	SlowRequestMS int // 慢请求阈值（毫秒），超过时访问日志升为 WARN；0 表示关闭
	// PublicBaseURL 是对外可访问的服务地址（如 https://api.example.com），
	// 用于拼接音频审核等外链；为空时接口返回相对路径。
	PublicBaseURL string

	// 鉴权与安全
	JWTSecret        string
	JWTExpireSeconds int
	IDCardHMACKey    string

	// 微信小程序
	WechatAppID  string
	WechatSecret string

	// 微信支付
	WechatMchID       string
	WechatPayKey      string
	WechatMchSerialNo string
	// 微信支付公钥模式（本项目唯一支持的回调验签方式）：公钥 ID + 公钥 PEM。
	// PEM 文本与只读挂载文件二选一；文件方式下 *File 记录来源路径。
	WechatPayPublicKeyID   string
	WechatPayPublicKey     string
	WechatPayPublicKeyFile string
	// 商户 API 私钥：请求签名用。
	WechatPrivateKey      string
	WechatPrivateKeyFile  string
	WechatNotifyURL       string
	WechatRefundNotifyURL string

	// 后台初始超管（仅首次启动建档，已存在账号不覆盖）
	AdminInitUsername string
	AdminInitPassword string

	// 定时任务（T34）：调度器轮询间隔、待支付订单超时、退款卡死判定窗口（分钟）
	StatusTickSeconds     int
	OrderExpireMinutes    int
	RefundWatchdogMinutes int

	// 短信（阿里云）：抽签提醒/结果通知等复用同一通道。
	// 三项齐全才启用；SMS_MOCK=true 时只记录日志，不调用阿里云。
	SMSMock            bool
	SMSAccessKeyID     string
	SMSAccessKeySecret string
	SMSSignName        string
}

// requirement 是一个必需配置项及其当前值。
type requirement struct {
	env   string
	value string
}

// MissingSecrets 返回缺失的必需环境变量名（按固定顺序，便于日志阅读）。
// 返回空切片表示自检通过。
func (c *Config) MissingSecrets() []string {
	required := []requirement{
		{"JWT_SECRET", c.JWTSecret},
		{"ID_CARD_HMAC_KEY", c.IDCardHMACKey},
		{"WECHAT_APPID", c.WechatAppID},
		{"WECHAT_SECRET", c.WechatSecret},
		{"ADMIN_INIT_PASSWORD", c.AdminInitPassword},
	}
	if !c.PayMock {
		required = append(required,
			requirement{"WECHAT_MCHID", c.WechatMchID},
			requirement{"WECHAT_PAY_KEY", c.WechatPayKey},
			requirement{"WECHAT_MCH_SERIAL_NO", c.WechatMchSerialNo},
			requirement{"WECHAT_PAY_PRIVATE_KEY", c.WechatPrivateKey},
			requirement{"WECHAT_PAY_PUBLIC_KEY_ID", c.WechatPayPublicKeyID},
			requirement{"WECHAT_PAY_PUBLIC_KEY", c.WechatPayPublicKey},
			requirement{"WECHAT_PAY_NOTIFY_URL", c.WechatNotifyURL},
			requirement{"WECHAT_REFUND_NOTIFY_URL", c.WechatRefundNotifyURL},
		)
	}

	var missing []string
	for _, item := range required {
		if item.value == "" {
			missing = append(missing, item.env)
		}
	}
	return missing
}
