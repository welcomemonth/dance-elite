package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/meetmonth/dance-elite/internal/pkg/logger"
)

// AccessLogOptions 控制访问日志行为。
type AccessLogOptions struct {
	// SlowRequestMS 是慢请求阈值（毫秒），超过时日志升为 WARN 并带 slow=true；
	// ≤0 使用默认值（500ms）。
	SlowRequestMS int
}

// AccessLog 记录每个请求一行结构化访问日志（zap）。
//
// 设计要点：
//   - 一行承载全部信息：身份（user_id/admin_id）、路由模板、路径参数、
//     业务注解（order_no/registration_id 等，由 logger.Add 注入）；
//   - 错误只在这里出现一次：HandleErrors 把 error_code/error/cause 挂到
//     请求字段，不再单独打日志；
//   - 降噪：健康检查、静态资源、SPA 页面导航降为 DEBUG；
//   - 慢请求升为 WARN，便于发现 SQLite 锁等待等问题。
func AccessLog(opts AccessLogOptions) gin.HandlerFunc {
	slowMS := opts.SlowRequestMS
	if slowMS <= 0 {
		slowMS = defaultSlowRequestMS
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		line := buildAccessLine(c, time.Since(start), slowMS)
		// 访问日志刻意不带 caller：它永远指向本中间件，无法定位业务代码，
		// 每行还会多出几十字节。出错位置由 route/error_code/cause/业务字段表达；
		// caller 只保留给业务事件、定时任务与 panic 日志（那些指向真实代码）。
		l := logger.FromCtx(c.Request.Context()).WithOptions(zap.WithCaller(false))
		switch line.level {
		case zapcore.ErrorLevel:
			l.Errorw(accessLogMessage, line.fields...)
		case zapcore.WarnLevel:
			l.Warnw(accessLogMessage, line.fields...)
		case zapcore.DebugLevel:
			l.Debugw(accessLogMessage, line.fields...)
		default:
			l.Infow(accessLogMessage, line.fields...)
		}
	}
}
