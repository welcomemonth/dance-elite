package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/pkg/token"
)

// 抽签 H5 微信授权相关的 cookie 与 context key。
const (
	// LotteryAuthCookieName 是公众号网页授权签发的身份 cookie（HttpOnly）。
	LotteryAuthCookieName = "de_lottery_auth"
	// lotteryStateCookieName 是授权跳转的 state cookie（双提交校验用）。
	lotteryStateCookieName = "de_lottery_state"

	lotteryOpenIDKey  = "lottery_openid"
	lotteryUnionIDKey = "lottery_unionid"
)

// RequireLotteryAuth 校验抽签 H5 授权 cookie：必须存在、签名有效、未过期，
// 且绑定到当前 URL 中的抽签 token。失败返回 40101，由前端跳转授权。
func RequireLotteryAuth(signer *token.Signer) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := lotteryClaims(c, signer)
		if !ok {
			httpx.AbortError(c, httpx.ErrUnauthorized("请重新在微信中打开链接完成授权后继续"))
			return
		}
		setLotteryIdentity(c, claims)
		c.Next()
	}
}

// OptionalLotteryAuth 在 cookie 有效时写入抽签身份，否则按匿名继续。
// 供 meta 接口使用：页面需要先加载再决定是否跳转授权。
func OptionalLotteryAuth(signer *token.Signer) gin.HandlerFunc {
	return func(c *gin.Context) {
		if claims, ok := lotteryClaims(c, signer); ok {
			setLotteryIdentity(c, claims)
		}
		c.Next()
	}
}

// GetLotteryIdentity 取出 OptionalLotteryAuth/RequireLotteryAuth 写入的
// 公众号身份；未授权返回 ok=false。
func GetLotteryIdentity(c *gin.Context) (openID, unionID string, ok bool) {
	value, exists := c.Get(lotteryOpenIDKey)
	if !exists {
		return "", "", false
	}
	openID, ok = value.(string)
	if !ok || openID == "" {
		return "", "", false
	}
	if raw, exists := c.Get(lotteryUnionIDKey); exists {
		unionID, _ = raw.(string)
	}
	return openID, unionID, true
}

// SetLotteryAuthCookie 写入抽签授权 cookie；maxAge 单位秒。
func SetLotteryAuthCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     LotteryAuthCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// SetLotteryStateCookie 写入授权 state cookie（10 分钟）。
func SetLotteryStateCookie(c *gin.Context, state string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     lotteryStateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// LotteryStateCookie 读取授权 state cookie。
func LotteryStateCookie(c *gin.Context) (string, bool) {
	value, err := c.Cookie(lotteryStateCookieName)
	if err != nil || value == "" {
		return "", false
	}
	return value, true
}

// ClearLotteryStateCookie 删除授权 state cookie。
func ClearLotteryStateCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     lotteryStateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// lotteryClaims 读取并校验授权 cookie，同时要求它绑定到路径上的抽签 token。
func lotteryClaims(c *gin.Context, signer *token.Signer) (*token.LotteryClaims, bool) {
	if signer == nil {
		return nil, false
	}
	raw, err := c.Cookie(LotteryAuthCookieName)
	if err != nil || raw == "" {
		return nil, false
	}
	claims, err := signer.ParseLottery(raw)
	if err != nil {
		return nil, false
	}
	tokenParam := strings.TrimSpace(c.Param("token"))
	expected := token.LotteryTokenHash(tokenParam)
	if subtle.ConstantTimeCompare([]byte(claims.TokenHash), []byte(expected)) != 1 {
		return nil, false
	}
	return claims, true
}

func setLotteryIdentity(c *gin.Context, claims *token.LotteryClaims) {
	c.Set(lotteryOpenIDKey, claims.OpenID)
	if claims.UnionID != "" {
		c.Set(lotteryUnionIDKey, claims.UnionID)
	}
	attachLogFields(c, "lottery_authorized", true)
}
