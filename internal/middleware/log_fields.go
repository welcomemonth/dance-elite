package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/pkg/logger"
)

// attachLogFields 把认证得到的身份字段挂到请求级 logger 上：此后该请求的
// 所有日志（含 service 业务事件）自动带 user_id / admin_id，无需逐处传参。
func attachLogFields(c *gin.Context, kv ...any) {
	ctx := c.Request.Context()
	c.Request = c.Request.WithContext(logger.Ctx(ctx, logger.FromCtx(ctx).With(kv...)))
}
