package main

import (
	"fmt"

	config "github.com/meetmonth/dance-elite/internal/configs"
	"github.com/meetmonth/dance-elite/internal/pkg/logger"
	"github.com/meetmonth/dance-elite/internal/pkg/wxpay"
)

// newPayGateway 按配置装配支付网关：PAY_MOCK=true 走 fake 通道（R2，证书未到
// 位不阻塞联调）；否则构造真实微信支付 V3 客户端（商户私钥/微信支付公钥/
// APIv3 密钥解析失败在启动时 fail-fast，敏感配置只经环境变量注入，见 T3）。
func newPayGateway(cfg *config.Config) (wxpay.Provider, error) {
	if cfg.PayMock {
		logger.Log.Warn("支付通道为 mock（PAY_MOCK=true）：小程序不会拉起真实微信支付")
		return wxpay.NewMockGateway(), nil
	}
	logger.Log.Infow("支付通道为微信支付 V3",
		"appid", cfg.WechatAppID,
		"mchid", cfg.WechatMchID,
		"notify_url", cfg.WechatNotifyURL,
		"verify_mode", "微信支付公钥",
	)
	client, err := wxpay.NewClient(wxpay.ClientOptions{
		AppID:           cfg.WechatAppID,
		MchID:           cfg.WechatMchID,
		SerialNo:        cfg.WechatMchSerialNo,
		PrivateKeyPEM:   cfg.WechatPrivateKey,
		APIv3Key:        cfg.WechatPayKey,
		PublicKeyID:     cfg.WechatPayPublicKeyID,
		PublicKeyPEM:    cfg.WechatPayPublicKey,
		NotifyURL:       cfg.WechatNotifyURL,
		RefundNotifyURL: cfg.WechatRefundNotifyURL,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化微信支付客户端: %w", err)
	}
	return client, nil
}
