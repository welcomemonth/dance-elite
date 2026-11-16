package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/pkg/logger"
)

// Recovery 捕获处理链中的 panic：记录堆栈与请求 ID，返回统一 50001 响应。
// 替代 gin.Default() 内置的 Recovery，输出走 zap 并复用统一响应包装。
//
// panic 是唯一单独成行的错误日志（带完整堆栈）；普通业务/内部错误由
// HandleErrors 注入请求字段，随访问日志输出一行。
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		ctx := c.Request.Context()
		logger.Add(ctx, "error_code", httpx.CodeInternal)
		logger.FromCtx(ctx).Errorw("http.panic",
			"method", c.Request.Method,
			"path", sanitizePath(c.FullPath(), c.Request.URL.Path),
			"error", logger.RedactText(fmt.Sprint(recovered)),
			"stack", string(debug.Stack()),
		)
		if c.Writer.Written() {
			return
		}
		c.JSON(http.StatusInternalServerError, httpx.Response{
			Code:    httpx.CodeInternal,
			Message: "服务器内部错误",
			Data:    nil,
		})
	})
}
