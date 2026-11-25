package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/pkg/token"
)

func newLotteryAuthTestEngine(t *testing.T, signer *token.Signer, require bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(httpx.HandleErrors())
	handler := func(c *gin.Context) {
		openID, unionID, ok := GetLotteryIdentity(c)
		c.JSON(http.StatusOK, gin.H{"openid": openID, "unionid": unionID, "ok": ok})
	}
	mw := OptionalLotteryAuth(signer)
	if require {
		mw = RequireLotteryAuth(signer)
	}
	engine.GET("/lottery/:token/check", mw, handler)
	return engine
}

func lotteryCookieRequest(engine *gin.Engine, path, cookie string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: LotteryAuthCookieName, Value: cookie})
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func TestRequireLotteryAuth(t *testing.T) {
	signer := token.NewSigner("jwt-secret", time.Hour)
	engine := newLotteryAuthTestEngine(t, signer, true)
	tokenHash := token.LotteryTokenHash("LotteryToken1")
	raw, err := signer.IssueLottery("openid-1", "unionid-1", tokenHash, time.Hour)
	if err != nil {
		t.Fatalf("IssueLottery: %v", err)
	}

	w := lotteryCookieRequest(engine, "/lottery/LotteryToken1/check", raw)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if !containsAll(w.Body.String(), `"openid":"openid-1"`, `"unionid":"unionid-1"`, `"ok":true`) {
		t.Fatalf("body = %s", w.Body.String())
	}

	if w := lotteryCookieRequest(engine, "/lottery/LotteryToken1/check", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("missing cookie status = %d, want 401", w.Code)
	}
	if w := lotteryCookieRequest(engine, "/lottery/OtherToken1/check", raw); w.Code != http.StatusUnauthorized {
		t.Fatalf("other token status = %d, want 401 (cookie bound to token)", w.Code)
	}
	if w := lotteryCookieRequest(engine, "/lottery/LotteryToken1/check", raw+"x"); w.Code != http.StatusUnauthorized {
		t.Fatalf("tampered cookie status = %d, want 401", w.Code)
	}
}

func TestOptionalLotteryAuth(t *testing.T) {
	signer := token.NewSigner("jwt-secret", time.Hour)
	engine := newLotteryAuthTestEngine(t, signer, false)

	w := lotteryCookieRequest(engine, "/lottery/LotteryToken1/check", "")
	if w.Code != http.StatusOK || !containsAll(w.Body.String(), `"ok":false`) {
		t.Fatalf("anonymous optional status = %d, body=%s", w.Code, w.Body.String())
	}

	raw, err := signer.IssueLottery("openid-1", "", token.LotteryTokenHash("LotteryToken1"), time.Hour)
	if err != nil {
		t.Fatalf("IssueLottery: %v", err)
	}
	w = lotteryCookieRequest(engine, "/lottery/LotteryToken1/check", raw)
	if w.Code != http.StatusOK || !containsAll(w.Body.String(), `"openid":"openid-1"`, `"ok":true`) {
		t.Fatalf("authorized optional status = %d, body=%s", w.Code, w.Body.String())
	}
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}
