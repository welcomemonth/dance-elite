package token

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestIssueAndParseLottery(t *testing.T) {
	s := NewSigner("test-secret", time.Hour)
	tokenHash := LotteryTokenHash("LotteryToken1")

	raw, err := s.IssueLottery("openid-1", "unionid-1", tokenHash, 0)
	if err != nil {
		t.Fatalf("IssueLottery: %v", err)
	}
	claims, err := s.ParseLottery(raw)
	if err != nil {
		t.Fatalf("ParseLottery: %v", err)
	}
	if claims.OpenID != "openid-1" || claims.UnionID != "unionid-1" || claims.TokenHash != tokenHash {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestParseLotteryRejectsTamperAndWrongSubject(t *testing.T) {
	s := NewSigner("test-secret", time.Hour)
	tokenHash := LotteryTokenHash("LotteryToken1")
	raw, err := s.IssueLottery("openid-1", "", tokenHash, time.Hour)
	if err != nil {
		t.Fatalf("IssueLottery: %v", err)
	}
	if _, err := s.ParseLottery(raw + "x"); err == nil {
		t.Fatal("tampered lottery token should fail")
	}

	userToken, err := s.IssueUser(1, "openid-1")
	if err != nil {
		t.Fatalf("IssueUser: %v", err)
	}
	if _, err := s.ParseLottery(userToken); err == nil {
		t.Fatal("user token must not parse as lottery token")
	}
	if _, err := s.ParseUser(raw); err == nil {
		t.Fatal("lottery token must not parse as user token")
	}
}

func TestLotteryStateRoundTrip(t *testing.T) {
	s := NewSigner("test-secret", time.Hour)
	tokenHash := LotteryTokenHash("LotteryToken1")

	state, err := s.IssueLotteryState(tokenHash, time.Minute)
	if err != nil {
		t.Fatalf("IssueLotteryState: %v", err)
	}
	if len(state) != lotteryStateLen {
		t.Fatalf("state length = %d, want %d", len(state), lotteryStateLen)
	}
	for _, r := range state {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Fatalf("state %q contains non-hex char %q (WeChat requires a-zA-Z0-9)", state, r)
		}
	}
	if err := s.ParseLotteryState(state, tokenHash); err != nil {
		t.Fatalf("ParseLotteryState: %v", err)
	}
}

func TestLotteryStateRejectsTamperExpiryAndOtherToken(t *testing.T) {
	s := NewSigner("test-secret", time.Hour)
	tokenHash := LotteryTokenHash("LotteryToken1")
	state, err := s.IssueLotteryState(tokenHash, time.Minute)
	if err != nil {
		t.Fatalf("IssueLotteryState: %v", err)
	}

	if err := s.ParseLotteryState(state, LotteryTokenHash("OtherToken1")); !errors.Is(err, ErrInvalidLotteryState) {
		t.Fatalf("wrong token hash error = %v", err)
	}

	tampered := []byte(state)
	tampered[len(tampered)-1] ^= 0x01
	if err := s.ParseLotteryState(string(tampered), tokenHash); !errors.Is(err, ErrInvalidLotteryState) {
		t.Fatalf("tampered state error = %v", err)
	}

	// 手工构造一个已过期的 state（测试同包可直接用签名函数）。
	nonceHex := strings.Repeat("a", lotteryStateNonceBytes*2)
	expHex := "0000000000000001"
	expired := nonceHex + expHex + s.lotteryStateSig(tokenHash, nonceHex, expHex)
	if err := s.ParseLotteryState(expired, tokenHash); !errors.Is(err, ErrInvalidLotteryState) {
		t.Fatalf("expired state error = %v", err)
	}
}
