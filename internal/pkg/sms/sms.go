// Package sms 提供可复用的短信发送能力（阿里云短信 + 本地 mock）。
//
// 首个使用方是抽签提醒/结果通知；后续的手机号登录、赛事提醒等复用同一客户端。
// 业务层只依赖 Client 接口，测试可注入 fake。
package sms

import (
	"context"
	"regexp"
)

// Client 是短信发送的最小接口。
type Client interface {
	// Send 发送一条模板短信；params 的键必须与模板变量名一致（不含 ${}）。
	Send(ctx context.Context, phone, templateCode string, params map[string]string) error
}

// Config 是短信通道配置，全部来自环境变量。
type Config struct {
	AccessKeyID     string
	AccessKeySecret string
	SignName        string
	// Mock 为 true 时不调用阿里云，只记录日志（本地开发/联调）。
	Mock bool
}

// Configured 判断是否配置了完整的阿里云短信参数。
func (c Config) Configured() bool {
	return c.AccessKeyID != "" && c.AccessKeySecret != "" && c.SignName != ""
}

// New 按配置构造客户端：
//   - Mock=true 返回 mock 客户端；
//   - 阿里云参数完整时返回阿里云客户端；
//   - 两者都不满足返回 (nil, nil)，调用方应把 nil 视为「短信未启用」。
func New(cfg Config) (Client, error) {
	if cfg.Mock {
		return &mockClient{}, nil
	}
	if !cfg.Configured() {
		return nil, nil
	}
	return newAliyunClient(cfg)
}

// phonePattern 匹配大陆手机号（11 位，1[3-9] 开头）。
var phonePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)

// ValidPhone 判断是否为合法的大陆手机号。
func ValidPhone(phone string) bool {
	return phonePattern.MatchString(phone)
}
