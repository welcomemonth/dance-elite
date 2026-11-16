// Package logger wraps zap with a process-wide logger plus context-scoped
// request logging.
//
// Layering rule: only wiring code (main / router / middleware) logs, via
// the global Log or a context logger from FromCtx. Business code
// (service / model) stays log-free — it returns errors and the handler
// layer records them via httpx.BizError.Cause — keeping services
// unit-testable without log capture. The one exception is business state
// transitions: service code may call Event (which only annotates the
// current request, or emits a single line outside a request).
package logger

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Log is the process-wide logger. Defaults to a no-op logger so packages
// stay safe to use in tests before Init runs; main calls Init once.
var Log = zap.NewNop().Sugar()

// Options controls log level and encoding.
type Options struct {
	// Level is the minimum level to emit; zero value is InfoLevel.
	Level zapcore.Level
	// Format is "json" (default, production) or "console" (development).
	Format string
}

// ParseLevel parses a textual level (debug/info/warn/error).
func ParseLevel(s string) (zapcore.Level, error) {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(s)); err != nil {
		return 0, fmt.Errorf("解析日志级别 %q: %w", s, err)
	}
	return level, nil
}

// New builds a logger: JSON to stdout in production, colored console in debug.
// Timestamps are ISO 8601 local time (the container sets TZ=Asia/Shanghai).
// Automatic stacktraces are disabled so a 5xx access log line does not carry a
// useless stack pointing at the middleware; panics log their stack explicitly.
func New(opts Options) (*zap.SugaredLogger, error) {
	format := opts.Format
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "console" {
		return nil, fmt.Errorf("不支持的日志格式 %q（可用 json/console）", format)
	}

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encCfg.EncodeLevel = zapcore.LowercaseLevelEncoder
	if format == "console" {
		encCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	cfg := zap.NewProductionConfig()
	cfg.Encoding = format
	cfg.EncoderConfig = encCfg
	cfg.Level = zap.NewAtomicLevelAt(opts.Level)
	cfg.OutputPaths = []string{"stdout"}
	cfg.ErrorOutputPaths = []string{"stderr"}
	cfg.DisableStacktrace = true
	// 关闭采样：单机低流量下不应静默丢弃重复日志（尤其是访问日志）。
	cfg.Sampling = nil

	l, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}
	return l.Sugar(), nil
}

// Init builds and installs the global logger. Called once from main
// (composition root), after config is loaded.
func Init(opts Options) error {
	l, err := New(opts)
	if err != nil {
		return err
	}
	Log = l
	return nil
}

type ctxKey struct{}

// Ctx returns a context carrying l, for propagating a request-scoped
// logger (e.g. with request_id attached) downstream.
func Ctx(ctx context.Context, l *zap.SugaredLogger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromCtx returns the logger stored in ctx, falling back to the global Log.
// Wiring code should log with FromCtx so request-scoped fields flow through.
func FromCtx(ctx context.Context) *zap.SugaredLogger {
	if l, ok := ctx.Value(ctxKey{}).(*zap.SugaredLogger); ok && l != nil {
		return l
	}
	return Log
}
