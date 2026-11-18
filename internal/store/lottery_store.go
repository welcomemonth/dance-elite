package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/meetmonth/dance-elite/internal/model"
)

// lotteryTokenPattern 限定 token 只能是字母数字，既保证链接不可枚举，
// 也避免把用户输入拼进文件路径造成目录穿越。
var lotteryTokenPattern = regexp.MustCompile(`^[A-Za-z0-9]{8,64}$`)

// ValidLotteryToken 判断 token 是否形如抽签文件名（8-64 位字母数字）。
func ValidLotteryToken(token string) bool {
	return lotteryTokenPattern.MatchString(token)
}

// LotteryStore 以「一个 token 一个 JSON 文件」的方式读写抽签数据。
//
// 抽签数据与数据库无关，全量放在 data/lottery/<token>.json。单机单进程，
// 写入用一把互斥锁串行化并用临时文件 + rename 原子替换，避免并发抽签
// 拿到同一个槽位或留下半截文件；每次覆盖前保留一份 <token>.json.bak。
type LotteryStore struct {
	dir string
	mu  sync.Mutex
}

// NewLotteryStore 构造文件存储；dir 为抽签 JSON 所在目录（如 data/lottery）。
func NewLotteryStore(dir string) *LotteryStore {
	return &LotteryStore{dir: dir}
}

// Load 读取一个赛事的抽签数据；token 非法或文件不存在均返回 ErrNotFound。
func (s *LotteryStore) Load(ctx context.Context, token string) (*model.LotteryData, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := s.path(token)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read lottery file: %w", err)
	}
	var data model.LotteryData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("decode lottery file: %w", err)
	}
	return &data, nil
}

// Mutate 在互斥锁内完成「读取 → 修改 → 原子写回」的整个流程。
//
// fn 返回错误时不写文件；返回 nil 时无论是否实际修改都会写回（数据量小，
// 重复写入的代价可以忽略）。fn 内返回的 *httpx.BizError 会原样传出。
func (s *LotteryStore) Mutate(ctx context.Context, token string, fn func(*model.LotteryData) error) (*model.LotteryData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.Load(ctx, token)
	if err != nil {
		return nil, err
	}
	if err := fn(data); err != nil {
		return nil, err
	}
	if err := s.write(token, data); err != nil {
		return nil, err
	}
	return data, nil
}

// Save 覆盖写入一个赛事的抽签数据（导入工具使用），目录不存在时自动创建。
func (s *LotteryStore) Save(ctx context.Context, token string, data *model.LotteryData) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.write(token, data)
}

// Exists 判断 token 对应的抽签文件是否已存在。
func (s *LotteryStore) Exists(token string) bool {
	path, err := s.path(token)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// Dir 返回抽签数据目录，供调用方拼接提示信息。
func (s *LotteryStore) Dir() string { return s.dir }

func (s *LotteryStore) path(token string) (string, error) {
	if !ValidLotteryToken(token) {
		return "", ErrNotFound
	}
	return filepath.Join(s.dir, token+".json"), nil
}

// write 调用方须已持有 s.mu。
func (s *LotteryStore) write(token string, data *model.LotteryData) error {
	path, err := s.path(token)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("create lottery dir: %w", err)
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode lottery data: %w", err)
	}
	// 覆盖前保留上一版，便于误操作后恢复。
	if old, err := os.ReadFile(path); err == nil {
		_ = os.WriteFile(path+".bak", old, 0o600)
	}
	tmp := path + ".tmp"
	if err := writeFileSync(tmp, raw); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace lottery file: %w", err)
	}
	return nil
}

// writeFileSync 写入并落盘，供原子替换前使用。
func writeFileSync(path string, raw []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	if _, err := f.Write(raw); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("close temp file: %w", err)
	}
	return nil
}
