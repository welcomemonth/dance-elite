package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
)

// actionContentUpdate 是内容管理写入 admin_logs 的动作名（API 文档 13.4 注意事项）。
const actionContentUpdate = "content_update"

// contentKeys 是内容管理已注册的 key 集合（API 文档 13.4；迁移 0015 种子）。
// 未注册的 key 一律 40401，不允许随意新增。
var contentKeys = map[string]struct{}{
	"points_rules_desc": {},
	"event_faq":         {},
	"official_account":  {},
	"announcement":      {},
}

// ContentService 是内容配置读写服务（联调 B5，API 文档 13.4）。
// 小程序端展示接口与后台读写同一份 settings 表。
type ContentService struct {
	stores *store.Store
}

// NewContentService 构造内容配置服务。
func NewContentService(stores *store.Store) *ContentService {
	return &ContentService{stores: stores}
}

// Get 读取指定 key 的内容配置（API 文档 13.4）。key 未注册 → 40401。
func (s *ContentService) Get(ctx context.Context, key string) (*model.ContentEntry, error) {
	if _, ok := contentKeys[key]; !ok {
		return nil, httpx.ErrNotFound("内容配置不存在")
	}
	value, updatedAt, err := s.stores.Settings.GetWithMeta(ctx, key)
	if errors.Is(err, store.ErrNotFound) {
		// 已注册 key 但种子行缺失（历史库）：返回空内容，首次保存时由 Upsert 补行。
		return &model.ContentEntry{Key: key, Value: map[string]any{}, UpdatedAt: ""}, nil
	}
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载内容配置失败，请稍后重试")
	}
	parsed, err := parseContentValue(value)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载内容配置失败，请稍后重试")
	}
	return &model.ContentEntry{Key: key, Value: parsed, UpdatedAt: updatedAt}, nil
}

// Put 写入指定 key 的内容配置（API 文档 13.4）：仅已注册 key，修改记操作日志。
// 调用方（路由层）保证 admin 角色；cs → 40301。
func (s *ContentService) Put(ctx context.Context, actorID int64, key string, value map[string]any) (*model.ContentEntry, error) {
	if _, ok := contentKeys[key]; !ok {
		return nil, httpx.ErrNotFound("内容配置不存在")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, httpx.ErrBadRequest("value 必须是合法 JSON 对象")
	}

	err = s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		if err := s.stores.Settings.Upsert(ctx, tx, key, string(raw)); err != nil {
			return err
		}
		targetType := "content"
		return adminLog(ctx, s.stores, tx, &actorID, actionContentUpdate, &targetType, nil,
			map[string]any{"key": key})
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, httpx.ErrNotFound("内容配置不存在")
		}
		return nil, httpx.ErrInternal(err, "保存内容配置失败，请稍后重试")
	}
	return s.Get(ctx, key)
}

// parseContentValue 把 settings.value 的 JSON 文本解析为对象。
func parseContentValue(raw string) (map[string]any, error) {
	var value map[string]any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, err
	}
	if value == nil {
		value = map[string]any{}
	}
	return value, nil
}
