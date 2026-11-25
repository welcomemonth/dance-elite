package router

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
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
	return New(Deps{Stores: stores, Lottery: service.NewLotteryService(stores, testLotteryRouteKey)})
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
