package main

import (
	"fmt"

	config "github.com/meetmonth/dance-elite/internal/configs"
	"github.com/meetmonth/dance-elite/internal/pkg/logger"
	"github.com/meetmonth/dance-elite/internal/pkg/wxpay"
)

// newPayGateway 按配置推导结果装配支付网关。
//
// 模式由 config 在启动时推导：没有 PAY_MOCK 开关。APP_ENV=dev 时支付材料
// 缺失/不可读（或 PEM 解析失败）自动回退 mock；APP_ENV=prod 时同类问题
// 直接返回错误，绝不静默使用假支付。
func newPayGateway(cfg *config.Config) (wxpay.Provider, error) {
	if cfg.Payment.Mode == config.PaymentModeMock {
		logger.Log.Warnw("支付通道为 mock：小程序不会拉起真实微信支付",
			"reason", cfg.Payment.MockReason)
		return wxpay.NewMockGateway(), nil
	}

	client, err := wxpay.NewClient(wxpay.ClientOptions{
		AppID:           cfg.Wechat.AppID,
		MchID:           cfg.Wechat.MchID,
		SerialNo:        cfg.Wechat.MchSerialNo,
		PrivateKeyPEM:   cfg.Secrets.WechatPayPrivateKeyPEM,
		APIv3Key:        cfg.Secrets.WechatPayAPIv3Key,
		PublicKeyID:     cfg.Wechat.PayPublicKeyID,
		PublicKeyPEM:    cfg.Secrets.WechatPayPublicKeyPEM,
		NotifyURL:       cfg.Payment.NotifyURL,
		RefundNotifyURL: cfg.Payment.RefundNotifyURL,
	})
	if err != nil {
		if cfg.IsDev() {
			logger.Log.Warnw("支付密钥解析失败，本地环境回退 mock", "error", err)
			return wxpay.NewMockGateway(), nil
		}
		return nil, fmt.Errorf("初始化微信支付客户端: %w", err)
	}

	logger.Log.Infow("支付通道为微信支付 V3",
		"appid", cfg.Wechat.AppID,
		"mchid", cfg.Wechat.MchID,
		"notify_url", cfg.Payment.NotifyURL,
		"verify_mode", "微信支付公钥",
	)
	return client, nil
}

// isMockGateway 判断网关是否为 mock 通道（退款看门狗据此跳过）。
func isMockGateway(gateway wxpay.Provider) bool {
	_, ok := gateway.(*wxpay.MockGateway)
	return ok
}
