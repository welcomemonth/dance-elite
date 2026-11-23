package sms

import (
	"context"
	"encoding/json"
	"fmt"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/v3/client"
	"github.com/alibabacloud-go/tea/tea"
)

// aliyunClient 封装阿里云短信 SendSms 接口。
type aliyunClient struct {
	client   *dysmsapi.Client
	signName string
}

func newAliyunClient(cfg Config) (*aliyunClient, error) {
	client, err := dysmsapi.NewClient(&openapi.Config{
		AccessKeyId:     tea.String(cfg.AccessKeyID),
		AccessKeySecret: tea.String(cfg.AccessKeySecret),
		Endpoint:        tea.String("dysmsapi.aliyuncs.com"),
	})
	if err != nil {
		return nil, fmt.Errorf("create aliyun sms client: %w", err)
	}
	return &aliyunClient{client: client, signName: cfg.SignName}, nil
}

// Send 调用阿里云 SendSms；Code 非 OK 一律视为失败并带上错误码与 request_id。
func (c *aliyunClient) Send(ctx context.Context, phone, templateCode string, params map[string]string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	templateParam := "{}"
	if len(params) > 0 {
		raw, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("encode sms params: %w", err)
		}
		templateParam = string(raw)
	}
	resp, err := c.client.SendSms(&dysmsapi.SendSmsRequest{
		PhoneNumbers:  tea.String(phone),
		SignName:      tea.String(c.signName),
		TemplateCode:  tea.String(templateCode),
		TemplateParam: tea.String(templateParam),
	})
	if err != nil {
		return fmt.Errorf("call aliyun sms: %w", err)
	}
	body := resp.Body
	if body == nil {
		return fmt.Errorf("call aliyun sms: empty response")
	}
	if tea.StringValue(body.Code) != "OK" {
		return fmt.Errorf("aliyun sms rejected: code=%s message=%s request_id=%s",
			tea.StringValue(body.Code), tea.StringValue(body.Message), tea.StringValue(body.RequestId))
	}
	return nil
}
