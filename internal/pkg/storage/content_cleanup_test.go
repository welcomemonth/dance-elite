package storage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStageContentImage 覆盖内容图片落盘路径与服务端随机文件名。
func TestStageContentImage(t *testing.T) {
	s := newTestStore(t)
	f, err := s.StageContentImage("official_account", ".png", strings.NewReader("qr"))
	if err != nil {
		t.Fatalf("StageContentImage: %v", err)
	}
	if !strings.HasPrefix(f.RelPath, "uploads/content/official_account_") || !strings.HasSuffix(f.RelPath, ".png") {
		t.Fatalf("rel path = %q", f.RelPath)
	}
	if err := s.Commit(f); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	rc, err := s.Open(f.RelPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer rc.Close()
	data, _ := io.ReadAll(rc)
	if string(data) != "qr" {
		t.Fatalf("content = %q", data)
	}
	if _, err := s.StageContentImage("../escape", ".png", strings.NewReader("x")); err == nil {
		t.Fatal("invalid content key should be rejected")
	}
}

// TestOpenRejectsOutsideUploads 覆盖 Open 的路径边界校验。
func TestOpenRejectsOutsideUploads(t *testing.T) {
	s := newTestStore(t)
	for _, rel := range []string{"", "uploads", "../etc/passwd", "/etc/passwd"} {
		if _, err := s.Open(rel); err == nil {
			t.Fatalf("Open(%q) should be rejected", rel)
		}
	}
	if _, err := s.Open("uploads/missing.jpg"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file should be ErrNotExist, got %v", err)
	}
}

// TestRemoveEventDir 覆盖赛后目录递归删除与幂等。
func TestRemoveEventDir(t *testing.T) {
	s := newTestStore(t)
	dir := filepath.Join(s.Root(), "12", "a8K2mP9x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range []string{"photo_a.jpg", "audio_a.mp3"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	removed, err := s.RemoveEventDir(12)
	if err != nil {
		t.Fatalf("RemoveEventDir: %v", err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	if _, err := os.Stat(filepath.Join(s.Root(), "12")); !os.IsNotExist(err) {
		t.Fatalf("event dir should be removed: %v", err)
	}
	if removed, err := s.RemoveEventDir(12); err != nil || removed != 0 {
		t.Fatalf("second remove = (%d, %v), want (0, nil)", removed, err)
	}
	if _, err := s.RemoveEventDir(0); err == nil {
		t.Fatal("invalid event id should be rejected")
	}
}
