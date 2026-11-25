package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/pkg/token"
	"github.com/meetmonth/dance-elite/internal/pkg/wechat"
	"github.com/meetmonth/dance-elite/internal/store"
)

func newLotteryAuthTestService(t *testing.T) (*LotteryAuthService, *store.Store, *token.Signer) {
	t.Helper()
	stores := newLotteryTestStores(t)
	mustSeedLottery(t, stores, testLotterySeed{
		Token: testLotteryToken, EventName: "测试赛事抽签",
		Groups: []model.LotteryGroup{
			{Genre: "classical", AgeGroup: "U11", Letter: "A", Size: 1},
		},
		Players: []testLotteryPlayer{
			{Name: "张三", IDCard: "310101201501011234", Genre: "classical", AgeGroup: "U11"},
		},
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sns/oauth2/access_token" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("code"); got != "good-code" {
			_, _ = w.Write([]byte(`{"errcode":40029,"errmsg":"invalid code"}`))
			return
		}
		_, _ = w.Write([]byte(`{"openid":"mp-openid-1","unionid":"unionid-1","scope":"snsapi_base"}`))
	}))
	t.Cleanup(server.Close)
	signer := token.NewSigner("jwt-secret", time.Hour)
	oauth := wechat.NewClient("mp-app", "mp-secret", wechat.WithBaseURL(server.URL))
	svc := NewLotteryAuthService(stores, oauth, signer, "https://api.example.com", "wechat")
	return svc, stores, signer
}

func TestLotteryAuthStartBuildsWeChatURL(t *testing.T) {
	svc, _, _ := newLotteryAuthTestService(t)
	redirect, state, err := svc.Start(context.Background(), testLotteryToken)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if state == "" {
		t.Fatal("state is empty")
	}
	parsed, err := url.Parse(redirect)
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}
	if parsed.Host != "open.weixin.qq.com" {
		t.Fatalf("host = %q", parsed.Host)
	}
	q := parsed.Query()
	if q.Get("appid") != "mp-app" || q.Get("scope") != "snsapi_base" || q.Get("response_type") != "code" {
		t.Fatalf("query = %v", q)
	}
	wantRedirect := "https://api.example.com/h5/lottery/" + testLotteryToken + "/auth/callback"
	if q.Get("redirect_uri") != wantRedirect {
		t.Fatalf("redirect_uri = %q, want %q", q.Get("redirect_uri"), wantRedirect)
	}
	if q.Get("state") != state {
		t.Fatalf("state param = %q, want %q", q.Get("state"), state)
	}
}

func TestLotteryAuthStartRejectsUnknownToken(t *testing.T) {
	svc, _, _ := newLotteryAuthTestService(t)
	_, _, err := svc.Start(context.Background(), "UnknownToken1")
	assertLotteryBizCode(t, err, httpx.CodeNotFound)
}

func TestLotteryAuthCallbackIssuesCookie(t *testing.T) {
	svc, _, signer := newLotteryAuthTestService(t)
	_, state, err := svc.Start(context.Background(), testLotteryToken)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	result, err := svc.Callback(context.Background(), testLotteryToken, "good-code", state, state)
	if err != nil {
		t.Fatalf("Callback: %v", err)
	}
	if result.OpenID != "mp-openid-1" || result.UnionID != "unionid-1" {
		t.Fatalf("identity = %+v", result)
	}
	claims, err := signer.ParseLottery(result.Cookie)
	if err != nil {
		t.Fatalf("ParseLottery: %v", err)
	}
	if claims.OpenID != "mp-openid-1" || claims.UnionID != "unionid-1" {
		t.Fatalf("claims = %+v", claims)
	}
	if claims.TokenHash != token.LotteryTokenHash(testLotteryToken) {
		t.Fatalf("token hash not bound to lottery token")
	}
}

func TestLotteryAuthCallbackRejectsBadState(t *testing.T) {
	svc, _, _ := newLotteryAuthTestService(t)
	_, state, err := svc.Start(context.Background(), testLotteryToken)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	flipped := state[:len(state)-1] + flippedHexChar(state[len(state)-1])
	if flipped == state {
		t.Fatal("tampered state equals original")
	}

	cases := []struct {
		name        string
		code        string
		state       string
		stateCookie string
	}{
		{"empty code", "", state, state},
		{"state cookie mismatch", "good-code", state, flipped},
		{"empty state", "good-code", "", state},
		{"tampered state", "good-code", flipped, flipped},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Callback(context.Background(), testLotteryToken, tc.code, tc.state, tc.stateCookie)
			if err == nil {
				t.Fatal("Callback should fail")
			}
			var biz *httpx.BizError
			if !errors.As(err, &biz) || biz.Code != httpx.CodeBadRequest {
				t.Fatalf("error = %v, want 40001", err)
			}
		})
	}
}

// flippedHexChar 返回一个不同的 hex 字符，用于篡改 state。
func flippedHexChar(c byte) string {
	if c == '0' {
		return "1"
	}
	return "0"
}
