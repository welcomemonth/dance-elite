package config

import (
	"fmt"
	"os"
	"strings"
)

// 微信支付回调路径；完整地址由 public_base_url 拼接，不再单独配置。
const (
	paymentNotifyPath = "/api/v1/payments/wechat/notify"
	refundNotifyPath  = "/api/v1/payments/wechat/refund-notify"
)

// derivePayment 推导支付模式与回调地址。
//
//   - 材料齐全且文件可读 → wechat；
//   - dev 下缺失/不可读 → mock，并记录原因；
//   - prod 下缺失/不可读 → 启动失败，绝不静默回退 mock。
func derivePayment(cfg *Config) error {
	if base := cfg.Server.PublicBaseURL; base != "" {
		cfg.Payment.NotifyURL = base + paymentNotifyPath
		cfg.Payment.RefundNotifyURL = base + refundNotifyPath
	}

	issues := paymentMaterialIssues(cfg)
	if len(issues) == 0 {
		if err := readPaymentMaterial(cfg); err != nil {
			issues = append(issues, err.Error())
		} else {
			cfg.Payment.Mode = PaymentModeWechat
			return nil
		}
	}
	if cfg.Env == EnvProd {
		return fmt.Errorf("支付配置不完整（APP_ENV=prod，不会回退 mock）: %s", strings.Join(issues, "; "))
	}
	cfg.Payment.Mode = PaymentModeMock
	cfg.Payment.MockReason = strings.Join(issues, "; ")
	return nil
}

// deriveLotteryAuth 推导抽签 H5 访问模式。
//
//   - auth_mode=open 仅允许 dev，prod 直接拒绝；
//   - wechat 需要公众号 appid/secret 与 public_base_url：dev 缺失时回退 open
//     并记录原因，prod 缺失时启动失败（与支付一致，绝不静默降级）。
func deriveLotteryAuth(cfg *Config) error {
	mode := strings.ToLower(strings.TrimSpace(cfg.Lottery.AuthMode))
	if mode == "" {
		mode = LotteryAuthModeWechat
	}
	switch mode {
	case LotteryAuthModeOpen:
		if cfg.Env == EnvProd {
			return fmt.Errorf("配置文件 lottery.auth_mode=open 仅允许在 APP_ENV=dev 使用（生产必须开启微信授权）")
		}
		cfg.Lottery.AuthMode = mode
		return nil
	case LotteryAuthModeWechat:
	default:
		return fmt.Errorf("配置文件 lottery.auth_mode=%q 不合法（可用 wechat/open）", mode)
	}

	var issues []string
	if cfg.Wechat.MPAppID == "" {
		issues = append(issues, "缺少 WECHAT_MP_APPID")
	}
	if cfg.Secrets.WechatMPSecret == "" {
		issues = append(issues, "缺少 WECHAT_MP_SECRET")
	}
	if cfg.Server.PublicBaseURL == "" {
		issues = append(issues, "缺少 server.public_base_url")
	}
	if len(issues) == 0 {
		cfg.Lottery.AuthMode = LotteryAuthModeWechat
		return nil
	}
	if cfg.Env == EnvProd {
		return fmt.Errorf("抽签微信授权配置不完整（APP_ENV=prod，不会回退 open）: %s", strings.Join(issues, "; "))
	}
	cfg.Lottery.AuthMode = LotteryAuthModeOpen
	cfg.Lottery.AuthModeReason = strings.Join(issues, "; ")
	return nil
}

// paymentMaterialIssues 检查真实支付所需的全部环境变量（不读文件）。
func paymentMaterialIssues(cfg *Config) []string {
	var issues []string
	required := []struct{ env, value string }{
		{"WECHAT_MCHID", cfg.Wechat.MchID},
		{"WECHAT_MCH_SERIAL_NO", cfg.Wechat.MchSerialNo},
		{"WECHAT_PAY_PUBLIC_KEY_ID", cfg.Wechat.PayPublicKeyID},
		{"WECHAT_PAY_API_V3_KEY", cfg.Secrets.WechatPayAPIv3Key},
		{"WECHAT_PAY_PRIVATE_KEY_FILE", cfg.Secrets.WechatPrivateKeyFile},
		{"WECHAT_PAY_PUBLIC_KEY_FILE", cfg.Secrets.WechatPayPublicKeyFile},
	}
	for _, item := range required {
		if item.value == "" {
			issues = append(issues, "缺少 "+item.env)
		}
	}
	if key := cfg.Secrets.WechatPayAPIv3Key; key != "" && len(key) != 32 {
		issues = append(issues, fmt.Sprintf("WECHAT_PAY_API_V3_KEY 必须为 32 字节，当前 %d 字节", len(key)))
	}
	return issues
}

// readPaymentMaterial 读取只读挂载的商户私钥与微信支付公钥。
func readPaymentMaterial(cfg *Config) error {
	privateKey, err := readSecretFile("WECHAT_PAY_PRIVATE_KEY_FILE", cfg.Secrets.WechatPrivateKeyFile)
	if err != nil {
		return err
	}
	publicKey, err := readSecretFile("WECHAT_PAY_PUBLIC_KEY_FILE", cfg.Secrets.WechatPayPublicKeyFile)
	if err != nil {
		return err
	}
	cfg.Secrets.WechatPayPrivateKeyPEM = privateKey
	cfg.Secrets.WechatPayPublicKeyPEM = publicKey
	return nil
}

func readSecretFile(envKey, path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%s=%s 读取失败: %w", envKey, path, err)
	}
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return "", fmt.Errorf("%s=%s 内容为空", envKey, path)
	}
	return value, nil
}
