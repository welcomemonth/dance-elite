package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
)

func testWebAssets() fstest.MapFS {
	return fstest.MapFS{
		"index.html":                   {Data: []byte("<html>root</html>")},
		"umi.12345678.js":              {Data: []byte("console.log(1)")},
		"scripts/loading.js":           {Data: []byte("console.log('loading')")},
		"event/list/index.html":        {Data: []byte("<html>list</html>")},
		"event/list/chunk.abcdef12.js": {Data: []byte("console.log(2)")},
	}
}

func newWebAssetsTestEngine(assets fstest.MapFS) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(httpx.HandleErrors())
	registerWebAssets(engine, assets)
	return engine
}

func serveWebRequest(engine *gin.Engine, method, target, accept string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func TestWebAssetsServesPageAndSpaFallback(t *testing.T) {
	engine := newWebAssetsTestEngine(testWebAssets())

	cases := []struct {
		path string
		body string
	}{
		{"/", "<html>root</html>"},
		{"/user/login", "<html>root</html>"},          // exportStatic 未覆盖的路径
		{"/audio-review/tok123", "<html>root</html>"}, // 动态路由必须回退
		{"/event/list/", "<html>list</html>"},         // exportStatic 生成的页面
	}
	for _, tc := range cases {
		w := serveWebRequest(engine, http.MethodGet, tc.path, "text/html")
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), tc.body) {
			t.Fatalf("%s: code = %d, body = %q", tc.path, w.Code, w.Body.String())
		}
	}
}

func TestWebAssetsRedirectsStaticRouteWithoutSlash(t *testing.T) {
	engine := newWebAssetsTestEngine(testWebAssets())

	w := serveWebRequest(engine, http.MethodGet, "/event/list", "text/html")
	if w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != "list/" {
		t.Fatalf("code = %d, location = %q", w.Code, w.Header().Get("Location"))
	}
}

func TestWebAssetsKeepsBackendPathsJSON404(t *testing.T) {
	engine := newWebAssetsTestEngine(testWebAssets())

	for _, p := range []string{"/api/v1/missing", "/h5/missing", "/static/materials/missing.png"} {
		w := serveWebRequest(engine, http.MethodGet, p, "text/html")
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s: code = %d, want 404", p, w.Code)
		}
		var resp struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Code != httpx.CodeNotFound {
			t.Fatalf("%s: body = %q, err = %v", p, w.Body.String(), err)
		}
	}
}

func TestWebAssetsDoesNotFallbackForAssetLikePaths(t *testing.T) {
	engine := newWebAssetsTestEngine(testWebAssets())

	for _, p := range []string{"/audio-review/missing.js", "/icons/missing.svg"} {
		w := serveWebRequest(engine, http.MethodGet, p, "text/html")
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s: code = %d, want 404", p, w.Code)
		}
	}
}

func TestWebAssetsDoesNotFallbackForNonNavigationRequests(t *testing.T) {
	engine := newWebAssetsTestEngine(testWebAssets())

	// fetch/XHR 的 Accept 通常不是 text/html，不能返回页面。
	w := serveWebRequest(engine, http.MethodGet, "/some-page", "*/*")
	if w.Code != http.StatusNotFound {
		t.Fatalf("fetch fallback: code = %d, want 404", w.Code)
	}

	// 非 GET/HEAD 请求一律按接口 404 处理。
	w = serveWebRequest(engine, http.MethodPost, "/some-page", "text/html")
	if w.Code != http.StatusNotFound {
		t.Fatalf("POST fallback: code = %d, want 404", w.Code)
	}
}

func TestWebAssetsCacheHeaders(t *testing.T) {
	engine := newWebAssetsTestEngine(testWebAssets())

	w := serveWebRequest(engine, http.MethodGet, "/", "text/html")
	if got := w.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("index cache = %q, want no-cache", got)
	}

	w = serveWebRequest(engine, http.MethodGet, "/event/list/", "text/html")
	if got := w.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("exported route cache = %q, want no-cache", got)
	}

	w = serveWebRequest(engine, http.MethodGet, "/umi.12345678.js", "*/*")
	if got := w.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Fatalf("hashed asset cache = %q, want immutable", got)
	}

	w = serveWebRequest(engine, http.MethodGet, "/scripts/loading.js", "*/*")
	if got := w.Header().Get("Cache-Control"); !strings.Contains(got, "max-age=3600") {
		t.Fatalf("plain asset cache = %q, want max-age=3600", got)
	}
}

func TestWebAssetsSkipsWhenIndexMissing(t *testing.T) {
	engine := newWebAssetsTestEngine(fstest.MapFS{".gitkeep": {Data: []byte{}}})

	w := serveWebRequest(engine, http.MethodGet, "/", "text/html")
	if w.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", w.Code)
	}
}
