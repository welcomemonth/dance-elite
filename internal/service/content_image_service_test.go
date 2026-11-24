package service

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/meetmonth/dance-elite/internal/pkg/storage"
)

// TestContentImageUpload 覆盖公众号二维码上传：落盘、写 image_url、
// 旧图清理、非官方账号 key 拒绝与审计留痕。
func TestContentImageUpload(t *testing.T) {
	f := newRegFixture(t)
	ctx := context.Background()
	adminID := mustAdminID(t, f, "admin")
	dir := t.TempDir()
	files, err := storage.New(dir)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	svc := NewContentService(f.stores, files)

	entry, err := svc.UploadImage(ctx, adminID, "official_account", contentPNG(t))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	first, _ := entry.Value["image_url"].(string)
	if !strings.HasPrefix(first, contentImageURLPrefix) {
		t.Fatalf("image_url = %q, want prefix %q", first, contentImageURLPrefix)
	}
	firstAbs := filepath.Join(dir, "content", strings.TrimPrefix(first, contentImageURLPrefix))
	if _, err := os.Stat(firstAbs); err != nil {
		t.Fatalf("uploaded image missing: %v", err)
	}

	entry, err = svc.UploadImage(ctx, adminID, "official_account", contentPNG(t))
	if err != nil {
		t.Fatalf("re-upload: %v", err)
	}
	second, _ := entry.Value["image_url"].(string)
	if second == first {
		t.Fatal("re-upload should replace image_url")
	}
	if _, err := os.Stat(firstAbs); !os.IsNotExist(err) {
		t.Fatalf("old image should be removed, stat err = %v", err)
	}

	if _, err := svc.UploadImage(ctx, adminID, "announcement", contentPNG(t)); mustBizCode(t, err) != 40001 {
		t.Fatalf("non-image key should be 40001, got %v", err)
	}
	if n := countAdminLogAction(t, f.db, actionContentUpdate); n != 2 {
		t.Fatalf("content update logs = %d, want 2", n)
	}
}

// contentPNG 生成一张合法的 1x1 png 上传文件。
func contentPNG(t *testing.T) *ContentImageFile {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	data := buf.Bytes()
	return &ContentImageFile{
		Filename: "qr.png",
		Size:     int64(len(data)),
		Content:  bytes.NewReader(data),
	}
}
