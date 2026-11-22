package config

import (
	"fmt"
	"net/url"
	"strings"
)

// validate 校验非密钥配置与始终必需的密钥。支付材料由 derivePayment 处理。
func validate(cfg *Config) error {
	switch cfg.Env {
	case EnvDev, EnvProd:
	default:
		return fmt.Errorf("环境变量 APP_ENV=%q 不合法（可用 dev/prod）", cfg.Env)
	}
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("配置文件 server.port=%d 不合法（可用 1-65535）", cfg.Server.Port)
	}
	if cfg.Server.SlowRequestMS < 0 {
		return fmt.Errorf("配置文件 server.slow_request_ms=%d 不能为负数（0 表示关闭慢请求告警）", cfg.Server.SlowRequestMS)
	}
	if strings.TrimSpace(cfg.Storage.DBPath) == "" {
		return fmt.Errorf("配置文件 storage.db_path 不能为空")
	}
	if strings.TrimSpace(cfg.Storage.UploadDir) == "" {
		return fmt.Errorf("配置文件 storage.upload_dir 不能为空")
	}
	switch cfg.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("配置文件 log.level=%q 不合法（可用 debug/info/warn/error）", cfg.Log.Level)
	}
	switch cfg.Log.Format {
	case "json", "console":
	default:
		return fmt.Errorf("配置文件 log.format=%q 不合法（可用 json/console）", cfg.Log.Format)
	}
	if cfg.Auth.JWTExpireSeconds <= 0 {
		return fmt.Errorf("配置文件 auth.jwt_expire_seconds=%d 必须为正整数（秒）", cfg.Auth.JWTExpireSeconds)
	}
	if strings.TrimSpace(cfg.Admin.InitUsername) == "" {
		return fmt.Errorf("配置文件 admin.init_username 不能为空")
	}
	if cfg.Scheduler.StatusTickSeconds <= 0 {
		return fmt.Errorf("配置文件 scheduler.status_tick_seconds=%d 必须为正整数（秒）", cfg.Scheduler.StatusTickSeconds)
	}
	if cfg.Scheduler.OrderExpireMinutes <= 0 {
		return fmt.Errorf("配置文件 scheduler.order_expire_minutes=%d 必须为正整数（分钟）", cfg.Scheduler.OrderExpireMinutes)
	}
	if cfg.Scheduler.RefundWatchdogMinutes <= 0 {
		return fmt.Errorf("配置文件 scheduler.refund_watchdog_minutes=%d 必须为正整数（分钟）", cfg.Scheduler.RefundWatchdogMinutes)
	}
	if err := normalizePublicBaseURL(cfg); err != nil {
		return err
	}

	var missing []string
	for _, item := range []struct{ env, value string }{
		{"JWT_SECRET", cfg.Secrets.JWTSecret},
		{"ID_CARD_HMAC_KEY", cfg.Secrets.IDCardHMACKey},
		{"WECHAT_APPID", cfg.Wechat.AppID},
		{"WECHAT_SECRET", cfg.Secrets.WechatSecret},
		{"ADMIN_INIT_PASSWORD", cfg.Secrets.AdminInitPassword},
	} {
		if item.value == "" {
			missing = append(missing, item.env)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("启动自检失败，缺少必需环境变量: %s", strings.Join(missing, ", "))
	}
	return nil
}

// normalizePublicBaseURL 校验并规范化对外地址；生产环境必须配置。
func normalizePublicBaseURL(cfg *Config) error {
	base := strings.TrimSpace(cfg.Server.PublicBaseURL)
	if base == "" {
		if cfg.Env == EnvProd {
			return fmt.Errorf("配置文件 server.public_base_url 在 APP_ENV=prod 时不能为空（用于微信支付回调与音频审核外链）")
		}
		return nil
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("配置文件 server.public_base_url=%q 不合法（需形如 https://api.example.com）", base)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("配置文件 server.public_base_url=%q 不能包含 query 或 fragment", base)
	}
	if cfg.Env == EnvProd && u.Scheme != "https" {
		return fmt.Errorf("配置文件 server.public_base_url=%q 在生产环境必须使用 https", base)
	}
	cfg.Server.PublicBaseURL = strings.TrimRight(base, "/")
	return nil
}
