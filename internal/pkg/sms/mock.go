package sms

import (
	"context"
	"strings"

	"github.com/meetmonth/dance-elite/internal/pkg/logger"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// mockClient 不真正发送短信，只记录一条脱敏日志，便于本地联调。
type mockClient struct{}

func (m *mockClient) Send(ctx context.Context, phone, templateCode string, params map[string]string) error {
	logger.FromCtx(ctx).Infow("sms.mock",
		"phone", utils.MaskPhone(phone),
		"template", templateCode,
		"params", redactSMSParams(params),
	)
	return nil
}

// redactSMSParams 对姓名类变量打码；手机号在调用处已打码。
func redactSMSParams(params map[string]string) map[string]string {
	if len(params) == 0 {
		return params
	}
	masked := make(map[string]string, len(params))
	for key, value := range params {
		if key == "name" {
			masked[key] = maskName(value)
			continue
		}
		masked[key] = value
	}
	return masked
}

func maskName(name string) string {
	runes := []rune(name)
	if len(runes) == 0 {
		return ""
	}
	if len(runes) == 1 {
		return "*"
	}
	return string(runes[0]) + strings.Repeat("*", len(runes)-1)
}
