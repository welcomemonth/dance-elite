package token

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// 抽签 H5 的两种凭据：
//   - auth cookie：公众号网页授权换取的身份（subject=lottery），绑定具体抽签 token；
//   - state：跳转微信授权时的防伪参数，短有效期、一次性，同时写入同站 cookie 做双提交。

const (
	subjectLottery      = "lottery"
	subjectLotteryState = "lottery_state"

	// lotteryStateNonceBytes 是 state 中随机 nonce 的字节数（hex 后 32 字符）。
	lotteryStateNonceBytes = 16
	// lotteryStateSigBytes 是 HMAC 截断后的字节数（hex 后 32 字符）。
	lotteryStateSigBytes = 16
	// lotteryStateExpHexDigits 是过期时间戳的固定 hex 位数（16 位）。
	lotteryStateExpHexDigits = 16
	// lotteryStateLen 是完整 state 的字符长度：nonce + exp + sig。
	lotteryStateLen = lotteryStateNonceBytes*2 + lotteryStateExpHexDigits + lotteryStateSigBytes*2
)

// ErrInvalidLotteryState 表示授权 state 不合法、被篡改或已过期。
var ErrInvalidLotteryState = errors.New("token: invalid lottery state")

// LotteryClaims 是抽签 H5 授权 cookie 的 payload。
// OpenID/UnionID 来自公众号网页授权；TokenHash 绑定到具体抽签链接。
type LotteryClaims struct {
	OpenID    string `json:"openid"`
	UnionID   string `json:"unionid,omitempty"`
	TokenHash string `json:"token_hash"`
	jwt.RegisteredClaims
}

// LotteryTokenHash 返回抽签 token 的稳定摘要，用于把授权 cookie 绑定到链接。
func LotteryTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// IssueLottery 为抽签 H5 签发授权 cookie（HS256）；ttl<=0 时使用签名器默认有效期。
func (s *Signer) IssueLottery(openID, unionID, tokenHash string, ttl time.Duration) (string, error) {
	if openID == "" || tokenHash == "" {
		return "", errors.New("token: empty lottery identity")
	}
	if ttl <= 0 {
		ttl = s.ttl
	}
	now := time.Now()
	claims := LotteryClaims{
		OpenID:    openID,
		UnionID:   unionID,
		TokenHash: tokenHash,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   subjectLottery,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign lottery token: %w", err)
	}
	return signed, nil
}

// ParseLottery 校验抽签 H5 授权 cookie。与用户/后台 token 共用签名器，
// 但锁定 subject=lottery，互不冒用。
func (s *Signer) ParseLottery(raw string) (*LotteryClaims, error) {
	claims := &LotteryClaims{}
	_, err := jwt.ParseWithClaims(raw, claims,
		func(*jwt.Token) (any, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithSubject(subjectLottery),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if claims.OpenID == "" || claims.TokenHash == "" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// IssueLotteryState 生成跳转微信授权用的 state。
//
// state 只含 a-zA-Z0-9（微信要求），格式为
// nonce(32 hex) + exp(16 hex unix 秒) + hmac(32 hex)，HMAC 覆盖 tokenHash、
// nonce 与 exp，保证不能被伪造。ttl<=0 时使用 10 分钟。
func (s *Signer) IssueLotteryState(tokenHash string, ttl time.Duration) (string, error) {
	if tokenHash == "" {
		return "", errors.New("token: empty lottery token hash")
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	nonce := make([]byte, lotteryStateNonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate lottery state nonce: %w", err)
	}
	nonceHex := hex.EncodeToString(nonce)
	expHex := fmt.Sprintf("%0*x", lotteryStateExpHexDigits, time.Now().Add(ttl).Unix())
	return nonceHex + expHex + s.lotteryStateSig(tokenHash, nonceHex, expHex), nil
}

// ParseLotteryState 校验 state：长度、HMAC、过期时间与绑定 token 均需匹配。
func (s *Signer) ParseLotteryState(state, tokenHash string) error {
	if tokenHash == "" || len(state) != lotteryStateLen {
		return ErrInvalidLotteryState
	}
	nonceHex := state[:lotteryStateNonceBytes*2]
	expHex := state[lotteryStateNonceBytes*2 : lotteryStateNonceBytes*2+lotteryStateExpHexDigits]
	sig := state[lotteryStateNonceBytes*2+lotteryStateExpHexDigits:]

	expected := s.lotteryStateSig(tokenHash, nonceHex, expHex)
	if subtle.ConstantTimeCompare([]byte(sig), []byte(expected)) != 1 {
		return ErrInvalidLotteryState
	}
	exp, err := strconv.ParseInt(expHex, 16, 64)
	if err != nil {
		return ErrInvalidLotteryState
	}
	if time.Now().Unix() > exp {
		return ErrInvalidLotteryState
	}
	return nil
}

// lotteryStateSig 计算 state 的 HMAC 截断签名（hex）。
func (s *Signer) lotteryStateSig(tokenHash, nonceHex, expHex string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(tokenHash))
	mac.Write([]byte("|"))
	mac.Write([]byte(nonceHex))
	mac.Write([]byte("|"))
	mac.Write([]byte(expHex))
	return hex.EncodeToString(mac.Sum(nil)[:lotteryStateSigBytes])
}
