package router

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/pkg/token"
	"github.com/meetmonth/dance-elite/internal/pkg/wechat"
	"github.com/meetmonth/dance-elite/internal/service"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
)

const (
	testLotteryRouteToken = "RouteToken1"
	testLotteryRouteKey   = "route-test-key"
	testWeChatUA          = "Mozilla/5.0 (iPhone) AppleWebKit/605.1.15 MicroMessenger/8.0.47"
)

// newLotteryTestEngine 打开内存库、写入一场抽签，只装配抽签 H5 路由。
func newLotteryTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	stores := newLotteryRouteStores(t)
	return New(Deps{Stores: stores, Lottery: service.NewLotteryService(stores, testLotteryRouteKey)})
}

// newLotteryAuthRouteEngine 装配开启微信授权的抽签 H5 路由（open 模式的反面）。
func newLotteryAuthRouteEngine(t *testing.T) (*gin.Engine, *store.Store) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	stores := newLotteryRouteStores(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sns/oauth2/access_token" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("code") != "good-code" {
			_, _ = w.Write([]byte(`{"errcode":40029,"errmsg":"invalid code"}`))
			return
		}
		_, _ = w.Write([]byte(`{"openid":"mp-openid-1","unionid":"unionid-1","scope":"snsapi_base"}`))
	}))
	t.Cleanup(server.Close)

	signer := token.NewSigner("jwt-secret", time.Hour)
	oauth := wechat.NewClient("mp-app", "mp-secret", wechat.WithBaseURL(server.URL))
	authSvc := service.NewLotteryAuthService(stores, oauth, signer, "https://api.example.com", "wechat")
	engine := New(Deps{
		Stores:      stores,
		Lottery:     service.NewLotteryService(stores, testLotteryRouteKey),
		LotteryAuth: authSvc,
		Tokens:      signer,
	})
	return engine, stores
}

func cookieNamed(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

// TestLotteryWeChatAuthFlow 覆盖：未授权 meta → 跳转授权 → 回调种 cookie →
// 带 cookie 抽签并记录公众号身份；无 cookie 的查询/抽签被拒绝。
func TestLotteryWeChatAuthFlow(t *testing.T) {
	engine, stores := newLotteryAuthRouteEngine(t)
	base := "/h5/lottery/" + testLotteryRouteToken

	// 1) 未授权：meta 正常返回，但 authorized=false。
	w := lotteryGet(t, engine, base)
	_, raw := decodeLotteryBody(t, w)
	var meta model.LotteryMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("decode meta: %v", err)
	}
	if meta.Authorized {
		t.Fatal("meta should be unauthorized before OAuth")
	}

	// 2) 授权跳转：302 到微信，并种下 state cookie。
	req := httptest.NewRequest(http.MethodGet, base+"/auth/start", nil)
	req.Header.Set("User-Agent", testWeChatUA)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("auth start status = %d, body=%s", w.Code, w.Body.String())
	}
	location := w.Header().Get("Location")
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse location: %v", err)
	}
	if parsed.Host != "open.weixin.qq.com" {
		t.Fatalf("auth start host = %q", parsed.Host)
	}
	state := parsed.Query().Get("state")
	stateCookie := cookieNamed(w.Result().Cookies(), "de_lottery_state")
	if state == "" || stateCookie == nil || stateCookie.Value != state {
		t.Fatalf("state = %q, cookie = %+v", state, stateCookie)
	}

	// 3) 回调：换 openid/unionid，种授权 cookie，回到抽签页。
	req = httptest.NewRequest(http.MethodGet, base+"/auth/callback?code=good-code&state="+url.QueryEscape(state), nil)
	req.Header.Set("User-Agent", testWeChatUA)
	req.AddCookie(stateCookie)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/lottery/"+testLotteryRouteToken {
		t.Fatalf("callback status=%d location=%q body=%s",
			w.Code, w.Header().Get("Location"), w.Body.String())
	}
	authCookie := cookieNamed(w.Result().Cookies(), "de_lottery_auth")
	if authCookie == nil || authCookie.Value == "" {
		t.Fatal("auth cookie not set")
	}
	if !authCookie.HttpOnly || !authCookie.Secure || authCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("auth cookie flags = %+v", authCookie)
	}

	// 4) 带 cookie：meta authorized=true。
	req = httptest.NewRequest(http.MethodGet, base, nil)
	req.Header.Set("User-Agent", testWeChatUA)
	req.AddCookie(authCookie)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	_, raw = decodeLotteryBody(t, w)
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("decode authorized meta: %v", err)
	}
	if !meta.Authorized {
		t.Fatal("meta should be authorized with valid cookie")
	}

	// 5) 不带 cookie 的查询与抽签被拒绝（40101）。
	w = lotteryPost(t, engine, base+"/query", `{"id_card":"310101201501011234"}`)
	if code, _ := decodeLotteryBody(t, w); w.Code != http.StatusUnauthorized || code != httpx.CodeUnauthorized {
		t.Fatalf("unauthorized query status=%d code=%d", w.Code, code)
	}
	w = lotteryPost(t, engine, base+"/draw", `{"name":"张三","id_card":"310101201501011234","genre":"classical"}`)
	if code, _ := decodeLotteryBody(t, w); w.Code != http.StatusUnauthorized || code != httpx.CodeUnauthorized {
		t.Fatalf("unauthorized draw status=%d code=%d", w.Code, code)
	}

	// 6) 带 cookie 抽签成功，并把身份写入选手记录。
	req = httptest.NewRequest(http.MethodPost, base+"/draw",
		bytes.NewBufferString(`{"name":"张三","id_card":"310101201501011234","genre":"classical"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", testWeChatUA)
	req.AddCookie(authCookie)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("authorized draw status = %d, body=%s", w.Code, w.Body.String())
	}

	_, _, players, err := stores.Lottery.GetByToken(context.Background(), testLotteryRouteToken)
	if err != nil {
		t.Fatalf("GetByToken: %v", err)
	}
	var found bool
	for _, player := range players {
		if player.Genre == model.DanceTypeClassical && player.SlotNo != nil {
			found = true
			if player.DrawnOpenID == nil || *player.DrawnOpenID != "mp-openid-1" ||
				player.DrawnUnionID == nil || *player.DrawnUnionID != "unionid-1" {
				t.Fatalf("drawn identity = %+v / %+v", player.DrawnOpenID, player.DrawnUnionID)
			}
		}
	}
	if !found {
		t.Fatal("classical player was not drawn")
	}
}

// TestLotteryWeChatAuthCallbackFailureRedirects 校验回调失败时回到页面并带 auth_error。
func TestLotteryWeChatAuthCallbackFailureRedirects(t *testing.T) {
	engine, _ := newLotteryAuthRouteEngine(t)
	base := "/h5/lottery/" + testLotteryRouteToken

	req := httptest.NewRequest(http.MethodGet, base+"/auth/callback?code=bad-code&state=whatever", nil)
	req.Header.Set("User-Agent", testWeChatUA)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/lottery/"+testLotteryRouteToken+"?auth_error=1" {
		t.Fatalf("callback failure status=%d location=%q", w.Code, w.Header().Get("Location"))
	}
	if cookie := cookieNamed(w.Result().Cookies(), "de_lottery_auth"); cookie != nil && cookie.Value != "" {
		t.Fatalf("failure must not set auth cookie: %+v", cookie)
	}
}

// newLotteryRouteStores 打开内存库、完成迁移并写入测试抽签。
func newLotteryRouteStores(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db.SQL()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	stores := store.New(db)
	players := []store.LotteryPlayerParams{
		mustRoutePlayer("张三", "310101201501011234", model.DanceTypeClassical),
		mustRoutePlayer("张三", "310101201501011234", model.DanceTypeFolk),
	}
	err = stores.WithTx(context.Background(), func(tx *sql.Tx) error {
		_, err := stores.Lottery.Create(context.Background(), tx, store.LotterySessionParams{
			EventName: "测试赛事抽签", Token: testLotteryRouteToken, Source: model.LotterySourceExcel,
			RemindBeforeMinutes: 120,
		}, []model.LotteryGroup{
			{Genre: model.DanceTypeClassical, AgeGroup: model.GroupLevelU11, Letter: "A", Size: 1},
			{Genre: model.DanceTypeFolk, AgeGroup: model.GroupLevelU11, Letter: "A", Size: 1},
		}, players)
		return err
	})
	if err != nil {
		t.Fatalf("seed lottery: %v", err)
	}
	return stores
}

func mustRoutePlayer(name, idCard, genre string) store.LotteryPlayerParams {
	return store.LotteryPlayerParams{
		Name:       name,
		IDCardHash: utils.HashIDNumber([]byte(testLotteryRouteKey), utils.IDTypeIDCard, idCard),
		Genre:      genre,
		AgeGroup:   model.GroupLevelU11,
	}
}

func lotteryPost(t *testing.T, engine *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", testWeChatUA)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func lotteryGet(t *testing.T, engine *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	return getJSON(t, engine, path, map[string]string{"User-Agent": testWeChatUA})
}

func decodeLotteryBody(t *testing.T, w *httptest.ResponseRecorder) (int, []byte) {
	t.Helper()
	var envelope struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
	return envelope.Code, envelope.Data
}

func TestLotteryMetaRoute(t *testing.T) {
	engine := newLotteryTestEngine(t)
	w := lotteryGet(t, engine, "/h5/lottery/"+testLotteryRouteToken)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	code, raw := decodeLotteryBody(t, w)
	if code != httpx.CodeOK {
		t.Fatalf("code = %d", code)
	}
	var meta model.LotteryMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("decode meta: %v", err)
	}
	if meta.EventName != "测试赛事抽签" || len(meta.Genres) != 2 {
		t.Fatalf("meta = %+v", meta)
	}
	if !meta.Authorized {
		t.Fatal("open mode meta should be authorized")
	}
}

// 不带舞种抽签先返回 choose_genre；指定舞种后返回 drawn；查询只凭身份证号。
func TestLotteryDrawLookupAndQueryRoutes(t *testing.T) {
	engine := newLotteryTestEngine(t)
	base := "/h5/lottery/" + testLotteryRouteToken

	w := lotteryPost(t, engine, base+"/draw", `{"name":"张三","id_card":"310101201501011234"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("draw status = %d, body=%s", w.Code, w.Body.String())
	}
	_, raw := decodeLotteryBody(t, w)
	var choose model.LotteryDrawResult
	if err := json.Unmarshal(raw, &choose); err != nil {
		t.Fatalf("decode choose: %v", err)
	}
	if choose.Status != model.LotteryOutcomeChooseGenre || len(choose.Choices) != 2 {
		t.Fatalf("choose = %+v", choose)
	}

	w = lotteryPost(t, engine, base+"/draw",
		`{"name":"张三","id_card":"310101201501011234","genre":"classical"}`)
	_, raw = decodeLotteryBody(t, w)
	var drawn model.LotteryDrawResult
	if err := json.Unmarshal(raw, &drawn); err != nil {
		t.Fatalf("decode draw: %v", err)
	}
	if drawn.Status != model.LotteryOutcomeDrawn || len(drawn.Entries) != 2 {
		t.Fatalf("draw = %+v", drawn)
	}

	// 重复抽签返回 already_drawn，不改变槽位。
	w = lotteryPost(t, engine, base+"/draw",
		`{"name":"张三","id_card":"310101201501011234","genre":"classical"}`)
	_, raw = decodeLotteryBody(t, w)
	var again model.LotteryDrawResult
	if err := json.Unmarshal(raw, &again); err != nil {
		t.Fatalf("decode again: %v", err)
	}
	if again.Status != model.LotteryOutcomeAlreadyDrawn {
		t.Fatalf("again status = %q", again.Status)
	}

	// 查询只需要身份证号，返回全部记录。
	w = lotteryPost(t, engine, base+"/query", `{"id_card":"310101201501011234"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("query status = %d, body=%s", w.Code, w.Body.String())
	}
	_, raw = decodeLotteryBody(t, w)
	var queried model.LotteryLookupResult
	if err := json.Unmarshal(raw, &queried); err != nil {
		t.Fatalf("decode query: %v", err)
	}
	if len(queried.Entries) != 2 {
		t.Fatalf("query entries = %d, want 2", len(queried.Entries))
	}

	// lookup 支持可选姓名。
	w = lotteryPost(t, engine, base+"/lookup", `{"name":"李四","id_card":"310101201501011234"}`)
	code, _ := decodeLotteryBody(t, w)
	if w.Code != http.StatusNotFound || code != httpx.CodeNotFound {
		t.Fatalf("lookup mismatch: status=%d code=%d", w.Code, code)
	}
}

func TestLotteryRouteErrors(t *testing.T) {
	engine := newLotteryTestEngine(t)

	w := lotteryPost(t, engine, "/h5/lottery/"+testLotteryRouteToken+"/draw", `{"name":"张三"}`)
	code, _ := decodeLotteryBody(t, w)
	if w.Code != http.StatusBadRequest || code != httpx.CodeBadRequest {
		t.Fatalf("bad request: status=%d code=%d", w.Code, code)
	}

	w = lotteryPost(t, engine, "/h5/lottery/"+testLotteryRouteToken+"/draw",
		`{"name":"李四","id_card":"310101201501011234"}`)
	code, _ = decodeLotteryBody(t, w)
	if w.Code != http.StatusNotFound || code != httpx.CodeNotFound {
		t.Fatalf("mismatch: status=%d code=%d", w.Code, code)
	}

	w = lotteryGet(t, engine, "/h5/lottery/Unknown1")
	code, _ = decodeLotteryBody(t, w)
	if w.Code != http.StatusNotFound || code != httpx.CodeNotFound {
		t.Fatalf("unknown token: status=%d code=%d", w.Code, code)
	}
}

// TestLotteryRejectsNonWeChat 校验抽签 H5 接口只允许微信内置浏览器访问。
func TestLotteryRejectsNonWeChat(t *testing.T) {
	engine := newLotteryTestEngine(t)

	req := httptest.NewRequest(http.MethodGet, "/h5/lottery/"+testLotteryRouteToken, nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if code, _ := decodeLotteryBody(t, w); code != httpx.CodeNotFound {
		t.Fatalf("non-wechat meta code=%d body=%s", code, w.Body)
	}

	req = httptest.NewRequest(http.MethodPost, "/h5/lottery/"+testLotteryRouteToken+"/query",
		bytes.NewBufferString(`{"id_card":"310101201501011234"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/120.0")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if code, _ := decodeLotteryBody(t, w); code != httpx.CodeNotFound {
		t.Fatalf("non-wechat query code=%d body=%s", code, w.Body)
	}
}
