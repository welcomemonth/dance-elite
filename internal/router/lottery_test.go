package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/service"
	"github.com/meetmonth/dance-elite/internal/store"
)

const testLotteryRouteToken = "RouteToken1"

// newLotteryTestEngine 只装配抽签 H5 路由与统一错误渲染中间件。
func newLotteryTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	lotteryStore := store.NewLotteryStore(t.TempDir())
	data := &model.LotteryData{
		Version:   1,
		EventName: "测试赛事抽签",
		Genres:    []model.LotteryGenre{{Key: "classical", Label: "古典舞"}},
		Groups:    []model.LotteryGroup{{Genre: "classical", AgeGroup: "U11", Letter: "A", Size: 1}},
		Players: []model.LotteryPlayer{
			{Name: "张三", IDCard: "310101201501011234", Genre: "classical", AgeGroup: "U11"},
		},
	}
	if err := lotteryStore.Save(context.Background(), testLotteryRouteToken, data); err != nil {
		t.Fatalf("save lottery data: %v", err)
	}
	engine := gin.New()
	engine.Use(httpx.HandleErrors())
	registerLotteryH5Routes(engine.Group("/h5"), service.NewLotteryService(lotteryStore))
	return engine
}

func lotteryPost(t *testing.T, engine *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
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
	w := getJSON(t, engine, "/h5/lottery/"+testLotteryRouteToken, nil)
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
	if meta.EventName != "测试赛事抽签" || len(meta.Genres) != 1 {
		t.Fatalf("meta = %+v", meta)
	}
}

func TestLotteryDrawAndQueryRoutes(t *testing.T) {
	engine := newLotteryTestEngine(t)
	drawBody := `{"name":"张三","id_card":"310101201501011234","genre":"classical"}`

	w := lotteryPost(t, engine, "/h5/lottery/"+testLotteryRouteToken+"/draw", drawBody)
	if w.Code != http.StatusOK {
		t.Fatalf("draw status = %d, body=%s", w.Code, w.Body.String())
	}
	code, raw := decodeLotteryBody(t, w)
	if code != httpx.CodeOK {
		t.Fatalf("draw code = %d", code)
	}
	var first model.LotteryResult
	if err := json.Unmarshal(raw, &first); err != nil {
		t.Fatalf("decode draw: %v", err)
	}
	if first.Slot == nil || first.Slot.Letter != "A" || first.Slot.No != 1 || !first.JustDrawn {
		t.Fatalf("draw result = %+v", first)
	}

	// 重复抽签返回同一结果，just_drawn=false。
	w = lotteryPost(t, engine, "/h5/lottery/"+testLotteryRouteToken+"/draw", drawBody)
	_, raw = decodeLotteryBody(t, w)
	var second model.LotteryResult
	if err := json.Unmarshal(raw, &second); err != nil {
		t.Fatalf("decode second draw: %v", err)
	}
	if second.JustDrawn || second.Slot == nil || *second.Slot != *first.Slot {
		t.Fatalf("second draw = %+v", second)
	}

	// 查询只凭身份证号 + 舞种。
	w = lotteryPost(t, engine, "/h5/lottery/"+testLotteryRouteToken+"/query",
		`{"id_card":"310101201501011234","genre":"classical"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("query status = %d, body=%s", w.Code, w.Body.String())
	}
	_, raw = decodeLotteryBody(t, w)
	var queried model.LotteryResult
	if err := json.Unmarshal(raw, &queried); err != nil {
		t.Fatalf("decode query: %v", err)
	}
	if queried.Slot == nil || *queried.Slot != *first.Slot {
		t.Fatalf("query result = %+v", queried)
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
		`{"name":"李四","id_card":"310101201501011234","genre":"classical"}`)
	code, _ = decodeLotteryBody(t, w)
	if w.Code != http.StatusNotFound || code != httpx.CodeNotFound {
		t.Fatalf("mismatch: status=%d code=%d", w.Code, code)
	}

	w = getJSON(t, engine, "/h5/lottery/Unknown1", nil)
	code, _ = decodeLotteryBody(t, w)
	if w.Code != http.StatusNotFound || code != httpx.CodeNotFound {
		t.Fatalf("unknown token: status=%d code=%d", w.Code, code)
	}
}
