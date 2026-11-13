// Package token 负责签发与校验 JWT（小程序用户与后台管理员共用签名器）。
//
// 只使用 HS256；校验时锁定算法、签发方与过期时间，防止 alg 混淆与过期 token。
package token

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const issuer = "dance-elite"

const (
	subjectUser  = "user"
	subjectAdmin = "admin"
)

// ErrInvalidToken 表示 token 缺失、签名错误、已过期或声明不完整。
var ErrInvalidToken = errors.New("token: invalid")

// UserClaims 是小程序用户 JWT 的 payload：user_id + openid。
type UserClaims struct {
	UserID int64  `json:"user_id"`
	OpenID string `json:"openid"`
	jwt.RegisteredClaims
}

// AdminClaims 是后台管理员 JWT 的 payload：admin_id + role
// （super_admin / admin / cs）。
type AdminClaims struct {
	AdminID int64  `json:"admin_id"`
	Role    string `json:"role"`
	jwt.RegisteredClaims
}

// Signer 按固定密钥与有效期签发/校验 token。
type Signer struct {
	secret []byte
	ttl    time.Duration

	// revokedAdmins 记录被停用管理员的踢下线标记（API 文档 14.1：
	// 停用立即踢下线）。ParseAdmin 拒绝名单中的 admin_id；
	// 重新激活时从名单移除。进程重启后名单清空，残留 token 最迟在
	// ttl（默认 2h）内自然过期。
	mu            sync.Mutex
	revokedAdmins map[int64]struct{}
}

// NewSigner 构造签名器；ttl 为 token 有效期（秒级精度，来自配置）。
func NewSigner(secret string, ttl time.Duration) *Signer {
	return &Signer{
		secret:        []byte(secret),
		ttl:           ttl,
		revokedAdmins: make(map[int64]struct{}),
	}
}

// RevokeAdmin 停用管理员：其既有 token 立即失效。
func (s *Signer) RevokeAdmin(adminID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revokedAdmins[adminID] = struct{}{}
}

// RestoreAdmin 重新激活管理员：解除踢下线标记（之后新签发的 token 正常可用）。
func (s *Signer) RestoreAdmin(adminID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.revokedAdmins, adminID)
}

// TTL 返回 token 有效期，用于登录响应中的 expires_in。
func (s *Signer) TTL() time.Duration { return s.ttl }

// IssueUser 为小程序用户签发 HS256 JWT。
func (s *Signer) IssueUser(userID int64, openID string) (string, error) {
	now := time.Now()
	claims := UserClaims{
		UserID: userID,
		OpenID: openID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   subjectUser,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign user token: %w", err)
	}
	return signed, nil
}

// ParseUser 校验并解析小程序用户 JWT。
func (s *Signer) ParseUser(raw string) (*UserClaims, error) {
	claims := &UserClaims{}
	_, err := jwt.ParseWithClaims(raw, claims,
		func(*jwt.Token) (any, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithSubject(subjectUser),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if claims.UserID <= 0 || claims.OpenID == "" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// IssueAdmin 为后台管理员签发 HS256 JWT。
func (s *Signer) IssueAdmin(adminID int64, role string) (string, error) {
	now := time.Now()
	claims := AdminClaims{
		AdminID: adminID,
		Role:    role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   subjectAdmin,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign admin token: %w", err)
	}
	return signed, nil
}

// ParseAdmin 校验并解析后台管理员 JWT。
//
// 与用户 token 共用签名器但锁定 subject=admin：小程序用户 token 不能
// 冒用后台接口，反之亦然。
func (s *Signer) ParseAdmin(raw string) (*AdminClaims, error) {
	claims := &AdminClaims{}
	_, err := jwt.ParseWithClaims(raw, claims,
		func(*jwt.Token) (any, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithSubject(subjectAdmin),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if claims.AdminID <= 0 || claims.Role == "" {
		return nil, ErrInvalidToken
	}
	s.mu.Lock()
	_, revoked := s.revokedAdmins[claims.AdminID]
	s.mu.Unlock()
	if revoked {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
