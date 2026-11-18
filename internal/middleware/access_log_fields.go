package middleware

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap/zapcore"

	"github.com/meetmonth/dance-elite/internal/pkg/logger"
)

const (
	accessLogMessage     = "http.access"
	defaultSlowRequestMS = 500
)

// sensitiveParamNames 是不允许进日志的路径参数名（音频审核 token 等）。
var sensitiveParamNames = map[string]struct{}{"token": {}}

// accessLine 是一行访问日志的级别与字段。
type accessLine struct {
	level  zapcore.Level
	fields []any
}

// buildAccessLine 把请求事实组装成一行日志。字段顺序固定，便于文档化与 grep。
func buildAccessLine(c *gin.Context, elapsed time.Duration, slowMS int) accessLine {
	route := c.FullPath()
	path := sanitizePath(route, c.Request.URL.Path)
	status := c.Writer.Status()
	latencyMS := math.Round(float64(elapsed.Microseconds())/10) / 100

	fields := make([]any, 0, 24)
	fields = append(fields,
		"method", c.Request.Method,
		"route", route,
		"path", path,
		"status", status,
		"error_code", 0, // 由 HandleErrors 经请求字段覆盖；0 表示成功
		"latency_ms", latencyMS,
	)
	if c.Request.ContentLength > 0 {
		fields = append(fields, "req_bytes", c.Request.ContentLength)
	}
	if n := c.Writer.Size(); n > 0 {
		fields = append(fields, "resp_bytes", n)
	}
	fields = append(fields, "client_ip", c.ClientIP())

	// 路径参数自动成为日志字段（event_id/order_id/registration_id 等），
	// 排除 token 一类密钥参数。
	for _, p := range c.Params {
		if !loggableParam(p.Key, p.Value) {
			continue
		}
		fields = append(fields, p.Key, p.Value)
	}
	// 认证/路由/service 注入的业务上下文；同 key 以后者为准。
	fields = mergeFields(fields, logger.RequestFields(c.Request.Context()))

	level := zapcore.InfoLevel
	switch {
	case status >= 500:
		level = zapcore.ErrorLevel
	case status >= 400:
		level = zapcore.WarnLevel
	}
	quiet := isQuietRequest(c, route)
	switch {
	case quiet && status < 400:
		level = zapcore.DebugLevel
	case !quiet && level != zapcore.ErrorLevel && slowMS > 0 && latencyMS >= float64(slowMS):
		level = zapcore.WarnLevel
		fields = append(fields, "slow", true)
	}
	return accessLine{level: level, fields: fields}
}

// isQuietRequest 判断是否属于高频低价值请求（健康检查、静态资源、SPA 导航），
// 这类请求只在 DEBUG 级别输出。
func isQuietRequest(c *gin.Context, route string) bool {
	if c.Request.Method == http.MethodOptions {
		return true
	}
	p := c.Request.URL.Path
	switch {
	case p == "/api/v1/ping":
		return true
	case strings.HasPrefix(p, "/static/"):
		return true
	case route == "" && !strings.HasPrefix(p, "/api/") && !strings.HasPrefix(p, "/h5/"):
		return true // 前端页面与静态资源
	}
	return false
}

// loggableParam 过滤空值与敏感路径参数。
func loggableParam(key, value string) bool {
	if key == "" || value == "" {
		return false
	}
	if _, sensitive := sensitiveParamNames[strings.ToLower(key)]; sensitive {
		return false
	}
	return true
}

// sanitizePath 按路由模板把敏感路径参数替换为 ***。
// 未命中路由（404/SPA 回退）时按前缀兜底，避免审核 token 落入日志。
func sanitizePath(route, path string) string {
	if route == "" {
		return maskSecretPathPrefix(path)
	}
	rp := strings.Split(route, "/")
	pp := strings.Split(path, "/")
	if len(rp) != len(pp) {
		return maskSecretPathPrefix(path)
	}
	for i, seg := range rp {
		switch {
		case strings.HasPrefix(seg, ":") && isSensitiveParam(seg[1:]):
			pp[i] = "***"
		case strings.HasPrefix(seg, "*"):
			pp[i] = "***"
		}
	}
	return strings.Join(pp, "/")
}

// isSensitiveParam 判断路径参数名是否涉及密钥。
func isSensitiveParam(name string) bool {
	name = strings.ToLower(name)
	if _, ok := sensitiveParamNames[name]; ok {
		return true
	}
	return strings.Contains(name, "token") || strings.Contains(name, "secret")
}

// maskSecretPathPrefix 处理没有路由模板时的敏感前缀（SPA 审核页导航等）。
func maskSecretPathPrefix(path string) string {
	for _, prefix := range []string{"/audio-review/", "/h5/audio-review/", "/lottery/", "/h5/lottery/"} {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		rest := path[len(prefix):]
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			return prefix + "***" + rest[i:]
		}
		return prefix + "***"
	}
	return path
}

// mergeFields 把 extra 合并进 base：同 key 以后者为准（保持首次出现的位置），
// 避免 JSON 中出现重复键。
func mergeFields(base, extra []any) []any {
	if len(extra) == 0 {
		return base
	}
	out := append([]any(nil), base...)
	index := make(map[string]int, len(out)/2)
	for i := 0; i+1 < len(out); i += 2 {
		index[fieldKey(out[i])] = i
	}
	for i := 0; i+1 < len(extra); i += 2 {
		k := fieldKey(extra[i])
		if j, ok := index[k]; ok {
			out[j+1] = extra[i+1]
			continue
		}
		index[k] = len(out)
		out = append(out, extra[i], extra[i+1])
	}
	return out
}

func fieldKey(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}
