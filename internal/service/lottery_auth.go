package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/pkg/token"
	"github.com/meetmonth/dance-elite/internal/pkg/wechat"
	"github.com/meetmonth/dance-elite/internal/store"
)

// 抽签 H5 微信授权的有效期。
const (
	lotteryStateTTL      = 10 * time.Minute
	lotteryAuthCookieTTL = 7 * 24 * time.Hour
)

// LotteryAuthService 承载抽签 H5 的公众号网页授权：
// 生成跳转地址、校验 state、用 code 换 openid/unionid 并签发 cookie。
type LotteryAuthService struct {
	stores  *store.Store
	oauth   *wechat.Client
	signer  *token.Signer
	baseURL string
	mode    string
}

// LotteryAuthResult 是授权回调成功后的结果。
type LotteryAuthResult struct {
	// Cookie 是种给浏览器的抽签授权凭据（JWT）。
	Cookie  string
	OpenID  string
	UnionID string
}

// NewLotteryAuthService 构造抽签授权服务；mode 为 wechat/open。
func NewLotteryAuthService(stores *store.Store, oauth *wechat.Client, signer *token.Signer,
	baseURL, mode string) *LotteryAuthService {
	return &LotteryAuthService{
		stores:  stores,
		oauth:   oauth,
		signer:  signer,
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		mode:    mode,
	}
}

// Enabled 报告是否强制微信网页授权。
func (s *LotteryAuthService) Enabled() bool { return s.mode == "wechat" }

// CookieMaxAge 返回授权 cookie 的有效期（秒）。
func (s *LotteryAuthService) CookieMaxAge() int {
	return int(lotteryAuthCookieTTL / time.Second)
}

// Start 生成公众号网页授权跳转地址与 state。
func (s *LotteryAuthService) Start(ctx context.Context, rawToken string) (redirectURL, state string, err error) {
	lotteryToken, err := s.validToken(ctx, rawToken)
	if err != nil {
		return "", "", err
	}
	if s.baseURL == "" {
		return "", "", httpx.ErrInternal(nil, "抽签授权地址未配置，请联系管理员")
	}
	tokenHash := token.LotteryTokenHash(lotteryToken)
	state, err = s.signer.IssueLotteryState(tokenHash, lotteryStateTTL)
	if err != nil {
		return "", "", httpx.ErrInternal(err, "生成授权参数失败，请稍后重试")
	}

	q := url.Values{}
	q.Set("appid", s.oauth.AppID())
	q.Set("redirect_uri", s.baseURL+"/h5/lottery/"+url.PathEscape(lotteryToken)+"/auth/callback")
	q.Set("response_type", "code")
	q.Set("scope", "snsapi_base")
	q.Set("state", state)
	redirectURL = "https://open.weixin.qq.com/connect/oauth2/authorize?" + q.Encode() + "#wechat_redirect"
	return redirectURL, state, nil
}

// Callback 校验 state，用微信 code 换身份并签发授权 cookie。
func (s *LotteryAuthService) Callback(ctx context.Context, rawToken, code, state, stateCookie string) (*LotteryAuthResult, error) {
	lotteryToken, err := s.validToken(ctx, rawToken)
	if err != nil {
		return nil, err
	}
	code = strings.TrimSpace(code)
	state = strings.TrimSpace(state)
	stateCookie = strings.TrimSpace(stateCookie)
	if code == "" {
		return nil, httpx.ErrBadRequest("微信授权未完成，请重新打开链接")
	}
	if state == "" || stateCookie == "" ||
		subtle.ConstantTimeCompare([]byte(state), []byte(stateCookie)) != 1 {
		return nil, httpx.ErrBadRequest("授权校验失败，请重新打开链接")
	}

	tokenHash := token.LotteryTokenHash(lotteryToken)
	if err := s.signer.ParseLotteryState(state, tokenHash); err != nil {
		return nil, httpx.ErrBadRequest("授权已过期，请重新打开链接")
	}

	user, err := s.oauth.OAuthAccessToken(ctx, code)
	if err != nil {
		if errors.Is(err, wechat.ErrInvalidCode) {
			return nil, httpx.ErrBadRequest("授权已过期，请重新打开链接").WithCause(err)
		}
		return nil, httpx.ErrInternal(err, "微信授权失败，请稍后重试")
	}
	cookie, err := s.signer.IssueLottery(user.OpenID, user.UnionID, tokenHash, lotteryAuthCookieTTL)
	if err != nil {
		return nil, httpx.ErrInternal(err, "签发授权凭据失败，请稍后重试")
	}
	return &LotteryAuthResult{Cookie: cookie, OpenID: user.OpenID, UnionID: user.UnionID}, nil
}

// validToken 校验抽签 token 格式并确认会话存在，返回规范化后的 token。
func (s *LotteryAuthService) validToken(ctx context.Context, rawToken string) (string, error) {
	if !store.ValidLotteryToken(rawToken) {
		return "", httpx.ErrNotFound("抽签链接无效或已失效")
	}
	normalized := store.NormalizeLotteryToken(rawToken)
	exists, err := s.stores.Lottery.ExistsToken(ctx, normalized)
	if err != nil {
		return "", httpx.ErrInternal(err, "读取抽签信息失败，请稍后重试")
	}
	if !exists {
		return "", httpx.ErrNotFound("抽签链接无效或已失效")
	}
	return normalized, nil
}
