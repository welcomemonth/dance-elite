package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"image"
	"io"
	"path"
	"strings"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
)

// 内容图片上传限制（S5-5，公众号二维码）：后台可上传 png/jpg，≤2MB。
const (
	MaxContentImageBytes        = 2 << 20
	MaxContentImageRequestBytes = MaxContentImageBytes + (1 << 20)
	maxContentImagePixels       = 20_000_000

	// contentImageKey 是支持图片上传的内容 key（目前仅公众号二维码）。
	contentImageKey = "official_account"
	// contentImageURLPrefix 是内容图片的对外访问前缀（映射 uploads/content/）。
	contentImageURLPrefix = "/static/content/"
)

// ContentImageFile 是一次内容图片上传的文件内容。
type ContentImageFile struct {
	Filename string
	Size     int64
	Content  io.ReadSeeker
}

// UploadImage 上传并保存内容图片（S5-5）：校验 png/jpg 与大小 → 落盘 →
// 事务内更新 settings.value.image_url 并写操作日志；提交后清理旧图片。
func (s *ContentService) UploadImage(ctx context.Context, actorID int64, key string, file *ContentImageFile) (*model.ContentEntry, error) {
	if _, ok := contentKeys[key]; !ok {
		return nil, httpx.ErrNotFound("内容配置不存在")
	}
	if key != contentImageKey {
		return nil, httpx.ErrBadRequest("该内容项不支持图片上传")
	}
	if s.files == nil {
		return nil, httpx.ErrInternal(errors.New("storage not configured"), "图片存储未配置")
	}
	ext, err := validateContentImage(file)
	if err != nil {
		return nil, err
	}

	if _, err := file.Content.Seek(0, io.SeekStart); err != nil {
		return nil, httpx.ErrInternal(err, "读取图片失败，请重试")
	}
	staged, err := s.files.StageContentImage(key, ext, file.Content)
	if err != nil {
		return nil, httpx.ErrInternal(err, "保存图片失败，请稍后重试")
	}
	defer s.files.Discard(staged)
	if err := s.files.Commit(staged); err != nil {
		return nil, httpx.ErrInternal(err, "保存图片失败，请稍后重试")
	}

	imageURL := contentImageURLPrefix + path.Base(staged.RelPath)
	old, err := s.currentImageURL(ctx, key)
	if err != nil {
		return nil, err
	}
	if err := s.persistImageURL(ctx, actorID, key, imageURL); err != nil {
		return nil, err
	}
	// 事务提交后再删除旧图片；失败不回滚（孤儿文件可由人工清理）。
	s.removeContentImage(old, imageURL)
	return s.Get(ctx, key)
}

// currentImageURL 读取当前配置里的 image_url（缺省为空）。
func (s *ContentService) currentImageURL(ctx context.Context, key string) (string, error) {
	entry, err := s.Get(ctx, key)
	if err != nil {
		return "", err
	}
	raw, _ := entry.Value["image_url"].(string)
	return raw, nil
}

// persistImageURL 在事务内把 value.image_url 写回 settings 并记操作日志。
func (s *ContentService) persistImageURL(ctx context.Context, actorID int64, key, imageURL string) error {
	err := s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		raw, err := s.stores.Settings.GetTx(ctx, tx, key)
		if errors.Is(err, store.ErrNotFound) {
			raw = "{}"
		} else if err != nil {
			return err
		}
		var value map[string]any
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return err
		}
		if value == nil {
			value = map[string]any{}
		}
		value["image_url"] = imageURL
		out, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if err := s.stores.Settings.Upsert(ctx, tx, key, string(out)); err != nil {
			return err
		}
		targetType := "content"
		return adminLog(ctx, s.stores, tx, &actorID, actionContentUpdate, &targetType, nil,
			map[string]any{"key": key, "image_url": imageURL})
	})
	if err != nil {
		var bizErr *httpx.BizError
		if errors.As(err, &bizErr) {
			return err
		}
		return httpx.ErrInternal(err, "保存图片失败，请稍后重试")
	}
	return nil
}

// removeContentImage 删除被替换的旧图片（只处理本系统内容图片 URL）。
func (s *ContentService) removeContentImage(oldURL, newURL string) {
	if oldURL == "" || oldURL == newURL {
		return
	}
	rel, ok := contentImageRelPath(oldURL)
	if !ok {
		return
	}
	_ = s.files.Remove(rel)
}

// contentImageRelPath 把 /static/content/x 还原为 uploads/content/x。
func contentImageRelPath(url string) (string, bool) {
	rest, ok := strings.CutPrefix(url, contentImageURLPrefix)
	if !ok || rest == "" || strings.Contains(rest, "/") {
		return "", false
	}
	return "uploads/content/" + rest, true
}

// validateContentImage 校验扩展名、大小与真实图片内容，返回规范扩展名。
func validateContentImage(f *ContentImageFile) (string, error) {
	if f == nil || f.Content == nil || f.Size <= 0 {
		return "", httpx.ErrBadRequest("请选择图片文件")
	}
	if f.Size > MaxContentImageBytes {
		return "", httpx.ErrBadRequest("图片不能超过 2MB")
	}
	ext := strings.ToLower(path.Ext(strings.ReplaceAll(f.Filename, "\\", "/")))
	var wantFormat, outExt string
	switch ext {
	case ".jpg", ".jpeg":
		wantFormat, outExt = "jpeg", "jpg"
	case ".png":
		wantFormat, outExt = "png", "png"
	default:
		return "", httpx.ErrBadRequest("图片仅支持 png/jpg 格式")
	}
	if _, err := f.Content.Seek(0, io.SeekStart); err != nil {
		return "", httpx.ErrInternal(err, "读取图片失败，请重试")
	}
	cfg, format, err := image.DecodeConfig(f.Content)
	if err != nil || format != wantFormat {
		return "", httpx.ErrBadRequest("图片文件格式不合法，请重新上传")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > maxContentImagePixels {
		return "", httpx.ErrBadRequest("图片尺寸不合法，请重新上传")
	}
	if _, err := f.Content.Seek(0, io.SeekStart); err != nil {
		return "", httpx.ErrInternal(err, "读取图片失败，请重试")
	}
	if _, decoded, err := image.Decode(f.Content); err != nil || decoded != wantFormat {
		return "", httpx.ErrBadRequest("图片文件格式不合法，请重新上传")
	}
	return outExt, nil
}
