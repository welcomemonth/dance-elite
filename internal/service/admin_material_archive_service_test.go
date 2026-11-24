package service

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/storage"
)

// TestMaterialArchive 覆盖媒体归档：按选手分目录、流式 zip、筛选与审计留痕。
func TestMaterialArchive(t *testing.T) {
	f := newAdminQueryFixture(t)
	ctx := context.Background()
	u11 := mustGroupID(t, f.db, f.eventID, model.GroupLevelU11, model.DanceTypeClassical)
	adminID := mustAdminID(t, f.regFixture, "admin")

	active := f.seed(t, "A1", "ORD-ARC-1", model.OrderStatusPaid,
		model.OrderSourceMiniapp, model.DanceTypeClassical, u11, "张小雨")
	canceled := f.seed(t, "A2", "ORD-ARC-2", model.OrderStatusCanceled,
		model.OrderSourceMiniapp, model.DanceTypeClassical, u11, "李小舞")

	dir := t.TempDir()
	files, err := storage.New(dir)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	put := func(seed adminSeed, kind, ext, body string) string {
		rel := fmt.Sprintf("uploads/%d/%s/%s_x.%s", f.eventID, seed.PlayerNo, kind, ext)
		abs := filepath.Join(dir, filepath.FromSlash(rel[len("uploads/"):]))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		return rel
	}
	photo := put(active, "photo", "jpg", "PHOTO")
	audio := put(active, "audio", "mp3", "AUDIO")
	put(canceled, "photo", "jpg", "CANCELED")
	mustExecSQL(t, f.db, `UPDATE registrations SET photo_path = ?, audio_path = ?, program_name = '春闺梦' WHERE id = ?`,
		photo, audio, active.RegistrationID)

	svc := NewMaterialArchiveService(f.stores, files)
	var buf bytes.Buffer
	stats, err := svc.Archive(ctx, adminID, f.eventID, MaterialArchiveOptions{}, &buf)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if stats.Players != 1 || stats.Files != 2 {
		t.Fatalf("stats = %+v, want 1 player 2 files", stats)
	}
	names, contents := readZip(t, buf.Bytes())
	joined := strings.Join(names, "|")
	for _, want := range []string{"张小雨_" + active.PlayerNo + "/证件照.jpg",
		"张小雨_" + active.PlayerNo + "/春闺梦.mp3"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("zip missing %q: %v", want, names)
		}
	}
	if strings.Contains(joined, canceled.PlayerNo) {
		t.Fatalf("canceled player should be excluded: %v", names)
	}
	if contents["PHOTO"] != "PHOTO" || contents["AUDIO"] != "AUDIO" {
		t.Fatalf("zip contents not streamed correctly: %v", contents)
	}
	if n := countAdminLogAction(t, f.db, adminActionMaterialArchive); n != 1 {
		t.Fatalf("archive logs = %d, want 1", n)
	}

	// 按选手筛选：只导出指定选手；include_canceled 含已取消。
	buf.Reset()
	stats, err = svc.Archive(ctx, adminID, f.eventID, MaterialArchiveOptions{
		PlayerNos: []string{active.PlayerNo}, IncludeCanceled: true,
	}, &buf)
	if err != nil || stats.Players != 1 {
		t.Fatalf("filtered archive: stats=%+v err=%v", stats, err)
	}
}

// readZip 读取 zip 条目名与内容（测试用，小文件）。
func readZip(t *testing.T, data []byte) ([]string, map[string]string) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	names := make([]string, 0, len(zr.File))
	contents := make(map[string]string, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open zip entry %q: %v", f.Name, err)
		}
		var b bytes.Buffer
		if _, err := b.ReadFrom(rc); err != nil {
			_ = rc.Close()
			t.Fatalf("read zip entry %q: %v", f.Name, err)
		}
		_ = rc.Close()
		names = append(names, f.Name)
		contents[b.String()] = b.String()
	}
	return names, contents
}
