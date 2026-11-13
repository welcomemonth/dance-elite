package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SettingsStore 是系统键值配置（settings 表）的数据访问层。
// 值均为 JSON 文本（模板/规则/文案），写入只发生在迁移种子与后台内容管理（后续任务）。
type SettingsStore struct{ db *DB }

// NewSettingsStore 构造系统配置 store。
func NewSettingsStore(db *DB) *SettingsStore { return &SettingsStore{db: db} }

// Get 返回指定 key 的配置值原文（JSON 文本）。不存在返回 ErrNotFound。
func (s *SettingsStore) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.sql.QueryRowContext(ctx, `SELECT value FROM settings WHERE "key" = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("query setting %q: %w", key, err)
	}
	return value, nil
}

// GetWithMeta 返回配置值原文与更新时间（后台内容管理响应用）。
func (s *SettingsStore) GetWithMeta(ctx context.Context, key string) (value, updatedAt string, err error) {
	err = s.db.sql.QueryRowContext(ctx,
		`SELECT value, updated_at FROM settings WHERE "key" = ?`, key).Scan(&value, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("query setting %q: %w", key, err)
	}
	return value, updatedAt, nil
}

// Upsert 在事务内写入配置值（JSON 文本）：key 不存在时插入，已存在时更新。
// key 的注册校验由 service 层负责（API 文档 13.4：未注册 key → 40401），
// 这里允许插入是为了兼容种子行缺失的库（迁移 0015 被应用后又被裁剪的情况）。
func (s *SettingsStore) Upsert(ctx context.Context, tx *sql.Tx, key, value string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO settings ("key", value) VALUES (?, ?)
		 ON CONFLICT ("key") DO UPDATE SET value = excluded.value`, key, value)
	if err != nil {
		return fmt.Errorf("upsert setting %q: %w", key, err)
	}
	return nil
}
