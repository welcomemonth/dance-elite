package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
)

// weChatBrowserToken 是微信内置浏览器 User-Agent 的固定标识。
// 微信 iOS/Android/桌面客户端及小程序 web-view 均包含该片段。
const weChatBrowserToken = "MicroMessenger"

// IsWeChatBrowser 判断请求是否来自微信内置浏览器。
//
// 仅用于限制抽签 H5 的访问入口，不能作为安全边界：User-Agent 可被伪造，
// 真正的数据保护仍依赖链接内不可枚举的 token。
func IsWeChatBrowser(userAgent string) bool {
	return strings.Contains(strings.ToLower(userAgent), strings.ToLower(weChatBrowserToken))
}

// WeChatOnly 只允许微信内置浏览器访问，其余请求按 404 处理（不暴露页面存在性）。
func WeChatOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsWeChatBrowser(c.GetHeader("User-Agent")) {
			httpx.AbortError(c, httpx.ErrNotFound("资源不存在"))
			return
		}
		c.Next()
	}
}
