package router

import (
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/middleware"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
)

// hashedAssetName 匹配 Umi 产物的内容哈希文件名，例如
// "_0dl27-qp8vovu.fa6ea6cd.css"、"umi.fade2c7b.js"。
var hashedAssetName = regexp.MustCompile(`\.[0-9a-f]{8}\.`)

// registerWebAssets 托管内嵌的前端单页应用（后台管理端 + 音频审核 H5）。
//
// 规则：
//   - /api、/h5、/static 前缀始终返回统一 JSON 404，不参与页面回退；
//   - 真实存在的文件/目录（含 exportStatic 生成的各路由 index.html）交给文件服务；
//   - 其余 GET/HEAD 导航请求回退根 index.html，支持 /audio-review/{token} 刷新；
//   - 带扩展名的路径不回退，避免把 HTML 当作缺失的 JS/CSS 返回。
func registerWebAssets(engine *gin.Engine, assets fs.FS) {
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		return // 前端尚未构建：不注册任何托管行为
	}
	fileServer := http.FileServer(http.FS(assets))

	engine.NoRoute(func(c *gin.Context) {
		raw := c.Request.URL.Path
		p := path.Clean(raw)
		if isBackendPath(p) || !isReadMethod(c.Request.Method) {
			httpx.AbortError(c, httpx.ErrNotFound("资源不存在"))
			return
		}
		// 抽签 H5 只允许在微信内打开；其他浏览器/来源直接 404。
		if isLotteryPage(p) && !middleware.IsWeChatBrowser(c.GetHeader("User-Agent")) {
			httpx.AbortError(c, httpx.ErrNotFound("资源不存在"))
			return
		}
		if assetExists(assets, p) {
			// 用原始路径判断缓存：path.Clean 会去掉目录路径末尾的斜杠。
			setAssetCache(c, raw)
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		if path.Ext(p) != "" || !wantsHTML(c) {
			httpx.AbortError(c, httpx.ErrNotFound("资源不存在"))
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	})
}

// isBackendPath 判断是否属于后端接口或上传文件路径，这些路径不参与 SPA 回退。
func isBackendPath(p string) bool {
	for _, prefix := range []string{"/api/", "/h5/", "/static/"} {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return p == "/api" || p == "/h5" || p == "/static"
}

// isLotteryPage 判断是否为选手抽签 H5 的页面路由（/lottery/{token}）。
func isLotteryPage(p string) bool {
	return p == "/lottery" || strings.HasPrefix(p, "/lottery/")
}

func isReadMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

// assetExists 判断请求路径是否对应构建产物中的文件，或含 index.html 的目录。
// 不含 index.html 的目录返回 false，避免 http.FileServer 输出目录列表。
func assetExists(assets fs.FS, p string) bool {
	name := strings.TrimPrefix(p, "/")
	if name == "" {
		name = "."
	}
	info, err := fs.Stat(assets, name)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return true
	}
	_, err = fs.Stat(assets, path.Join(name, "index.html"))
	return err == nil
}

// setAssetCache 设置静态资源缓存：入口 HTML 不缓存，带哈希的文件长期缓存。
// 发版后 index.html 必须立即更新，否则会引用已删除的旧哈希文件。
func setAssetCache(c *gin.Context, p string) {
	switch {
	case p == "/" || strings.HasSuffix(p, "/") || strings.HasSuffix(p, ".html"):
		c.Header("Cache-Control", "no-cache")
	case hashedAssetName.MatchString(path.Base(p)):
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	default:
		c.Header("Cache-Control", "public, max-age=3600")
	}
}

// wantsHTML 区分页面导航与资源请求：浏览器地址栏请求带 Accept: text/html，
// 动态加载的 JS/CSS 通常带 */*，后者不应回退 index.html。
func wantsHTML(c *gin.Context) bool {
	accept := c.GetHeader("Accept")
	return accept == "" || strings.Contains(accept, "text/html")
}
