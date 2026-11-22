package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/pkg/storage"
	"github.com/meetmonth/dance-elite/internal/store"
)

// actionAudioReviewToken 是生成音频审核链接的动作名（API 文档 10.6）。
const actionAudioReviewToken = "audio_review_token"

// audioReviewTokenAlphabet 是 token 字符集（大小写字母 + 数字，不可枚举）。
const audioReviewTokenAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// audioReviewTokenLength 是 token 长度（API 文档 7.1：8 位）。
const audioReviewTokenLength = 8

// AudioReviewService 承载音频审核链接生成与 H5 免登录数据读取（API 文档 7.1/10.6）。
type AudioReviewService struct {
	stores        *store.Store
	publicBaseURL string
}

// NewAudioReviewService 构造音频审核服务；publicBaseURL 为空时返回相对路径。
func NewAudioReviewService(stores *store.Store, publicBaseURL string) *AudioReviewService {
	return &AudioReviewService{stores: stores, publicBaseURL: strings.TrimRight(publicBaseURL, "/")}
}

// GenerateToken 生成或重新生成赛事音频审核 token（旧链接立即失效）并记操作日志。
func (s *AudioReviewService) GenerateToken(ctx context.Context, adminID, eventID int64) (*model.AudioReviewTokenResponse, error) {
	token, err := newAudioReviewToken()
	if err != nil {
		return nil, httpx.ErrInternal(err, "生成审核链接失败，请稍后重试")
	}
	err = s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		if err := s.stores.Events.SetAudioReviewToken(ctx, tx, eventID, token); err != nil {
			return err
		}
		targetType := "event"
		return adminLog(ctx, s.stores, tx, &adminID, actionAudioReviewToken, &targetType, &eventID,
			map[string]any{"token": token})
	})
	if errors.Is(err, store.ErrNotFound) {
		return nil, httpx.ErrNotFound("赛事不存在")
	}
	if err != nil {
		return nil, httpx.ErrInternal(err, "生成审核链接失败，请稍后重试")
	}
	return &model.AudioReviewTokenResponse{URL: s.audioReviewURL(token), Token: token}, nil
}

// Get 返回赛事音频审核数据（免登录，按 token 定位赛事）。
func (s *AudioReviewService) Get(ctx context.Context, token, keyword string) (*model.AudioReviewData, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, httpx.ErrNotFound("审核链接无效或已失效")
	}
	event, err := s.stores.Events.GetByAudioReviewToken(ctx, token)
	if errors.Is(err, store.ErrNotFound) {
		return nil, httpx.ErrNotFound("审核链接无效或已失效")
	}
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载审核数据失败，请稍后重试")
	}
	rows, err := s.stores.Events.ListAudioReview(ctx, event.ID, strings.TrimSpace(keyword))
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载审核数据失败，请稍后重试")
	}
	entries := make([]model.AudioReviewEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, model.AudioReviewEntry{
			PlayerName:  row.PlayerName,
			ProgramName: row.ProgramName,
			AudioURL:    storage.PublicURL(row.AudioPath),
			UpdatedAt:   row.UpdatedAt,
		})
	}
	return &model.AudioReviewData{EventName: event.Name, Entries: entries}, nil
}

// audioReviewURL 按配置拼接完整链接；未配置 server.public_base_url 时返回相对路径。
func (s *AudioReviewService) audioReviewURL(token string) string {
	return s.publicBaseURL + "/audio-review/" + token
}

// newAudioReviewToken 生成 8 位 crypto/rand 字母数字 token。
func newAudioReviewToken() (string, error) {
	buf := make([]byte, audioReviewTokenLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	out := make([]byte, audioReviewTokenLength)
	for i, b := range buf {
		out[i] = audioReviewTokenAlphabet[int(b)%len(audioReviewTokenAlphabet)]
	}
	return string(out), nil
}
