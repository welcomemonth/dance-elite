package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/pkg/logger"
)

// captureLogs 把全局 logger 指向内存缓冲，返回读取输出的函数。
// 访问日志与 httpx.HandleErrors 都经 logger.FromCtx 取全局日志，故测试可捕获。
func captureLogs(t *testing.T) func() map[string]any {
	t.Helper()
	buf := &bytes.Buffer{}
	encCfg := zapcore.EncoderConfig{
		MessageKey:  "msg",
		LevelKey:    "level",
		TimeKey:     "ts",
		EncodeLevel: zapcore.LowercaseLevelEncoder,
		EncodeTime:  zapcore.EpochTimeEncoder,
	}
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encCfg), zapcore.AddSync(buf), zapcore.DebugLevel)
	logger.Log = zap.New(core).Sugar()
	t.Cleanup(func() { logger.Log = zap.NewNop().Sugar() })

	return func() map[string]any {
		t.Helper()
		lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
		if len(lines) == 0 || len(lines[0]) == 0 {
			t.Fatal("no log line captured")
		}
		var entry map[string]any
		if err := json.Unmarshal(lines[len(lines)-1], &entry); err != nil {
			t.Fatalf("parse log line: %v (%s)", err, lines[len(lines)-1])
		}
		return entry
	}
}

// newAccessEngine 构造只含日志链路的测试引擎（路由模板用于 route 字段）。
func newAccessEngine(method, route string, slowMS int, h gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(RequestID(), AccessLog(AccessLogOptions{SlowRequestMS: slowMS}), httpx.HandleErrors())
	e.Handle(method, route, h)
	return e
}

func TestAccessLogSuccessCarriesIdentityAndContext(t *testing.T) {
	line := captureLogs(t)
	e := newAccessEngine(http.MethodGet, "/api/v1/events/:event_id", 500, func(c *gin.Context) {
		attachLogFields(c, "user_id", int64(42))
		logger.Add(c.Request.Context(), "order_no", "2026010100000012345678")
		httpx.OK(c, nil)
	})

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/events/1", nil))

	entry := line()
	assertField(t, entry, "msg", "http.access")
	assertField(t, entry, "level", "info")
	assertField(t, entry, "method", "GET")
	assertField(t, entry, "route", "/api/v1/events/:event_id")
	assertField(t, entry, "path", "/api/v1/events/1")
	assertField(t, entry, "status", float64(200))
	assertField(t, entry, "error_code", float64(0))
	assertField(t, entry, "event_id", "1")
	assertField(t, entry, "order_no", "2026010100000012345678")
	assertField(t, entry, "user_id", float64(42))
	if _, ok := entry["request_id"]; !ok {
		t.Error("request_id missing")
	}
}

func TestAccessLogBizErrorBecomesWarnWithCode(t *testing.T) {
	line := captureLogs(t)
	e := newAccessEngine(http.MethodPost, "/api/v1/events/:event_id/registrations", 500, func(c *gin.Context) {
		httpx.AbortError(c, httpx.ErrQuotaFull("该组别名额已满", nil))
	})

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/events/1/registrations", nil))

	entry := line()
	assertField(t, entry, "level", "warn")
	assertField(t, entry, "status", float64(400))
	assertField(t, entry, "error_code", float64(httpx.CodeQuotaFull))
	assertField(t, entry, "error", "该组别名额已满")
}

func TestAccessLogInternalErrorBecomesErrorWithCause(t *testing.T) {
	line := captureLogs(t)
	e := newAccessEngine(http.MethodGet, "/api/v1/events/:event_id", 500, func(c *gin.Context) {
		httpx.AbortError(c, httpx.ErrInternal(errStub("database is locked"), ""))
	})

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/events/1", nil))

	entry := line()
	assertField(t, entry, "level", "error")
	assertField(t, entry, "error_code", float64(httpx.CodeInternal))
	assertField(t, entry, "cause", "database is locked")
}

func TestAccessLogQuietPathsDropToDebug(t *testing.T) {
	line := captureLogs(t)
	e := newAccessEngine(http.MethodGet, "/api/v1/ping", 500, func(c *gin.Context) {
		httpx.OK(c, gin.H{"message": "pong"})
	})

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil))

	assertField(t, line(), "level", "debug")
}

func TestAccessLogMasksReviewToken(t *testing.T) {
	line := captureLogs(t)
	e := newAccessEngine(http.MethodGet, "/h5/audio-review/:token", 500, func(c *gin.Context) {
		httpx.OK(c, nil)
	})

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/h5/audio-review/secret-token-123", nil))

	entry := line()
	assertField(t, entry, "path", "/h5/audio-review/***")
	if _, leaked := entry["token"]; leaked {
		t.Error("review token must not be logged as a field")
	}
	if got := entry["path"]; got == "/h5/audio-review/secret-token-123" {
		t.Error("raw review token leaked into path")
	}
}

func TestAccessLogSlowRequestWarns(t *testing.T) {
	line := captureLogs(t)
	e := newAccessEngine(http.MethodGet, "/api/v1/events/:event_id", 1, func(c *gin.Context) {
		time.Sleep(5 * time.Millisecond)
		httpx.OK(c, nil)
	})

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/events/1", nil))

	entry := line()
	assertField(t, entry, "level", "warn")
	assertField(t, entry, "slow", true)
}

func TestSanitizePath(t *testing.T) {
	cases := []struct {
		route, path, want string
	}{
		{"/h5/audio-review/:token", "/h5/audio-review/abc", "/h5/audio-review/***"},
		{"/api/v1/events/:event_id", "/api/v1/events/1", "/api/v1/events/1"},
		{"", "/audio-review/abc", "/audio-review/***"},
		{"", "/audio-review/abc/sub", "/audio-review/***/sub"},
		{"", "/dashboard", "/dashboard"},
	}
	for _, tc := range cases {
		if got := sanitizePath(tc.route, tc.path); got != tc.want {
			t.Errorf("sanitizePath(%q, %q) = %q, want %q", tc.route, tc.path, got, tc.want)
		}
	}
}

func assertField(t *testing.T, entry map[string]any, key string, want any) {
	t.Helper()
	got, ok := entry[key]
	if !ok {
		t.Fatalf("field %q missing in %v", key, entry)
	}
	if got != want {
		t.Fatalf("field %q = %v (%T), want %v (%T)", key, got, got, want, want)
	}
}

// errStub 是测试用 error。
type errStub string

func (e errStub) Error() string { return string(e) }
