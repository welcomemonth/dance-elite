package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/storage"
)

// TestMaterialCleanupJob 覆盖赛后媒体清理：删目录、路径置 NULL、文本保留、
// 标记已清理与幂等（重复执行不重复留痕）。
func TestMaterialCleanupJob(t *testing.T) {
	f := newAdminQueryFixture(t)
	ctx := context.Background()
	u11 := mustGroupID(t, f.db, f.eventID, model.GroupLevelU11, model.DanceTypeClassical)
	seed := f.seed(t, "C1", "ORD-CLEAN-1", model.OrderStatusPaid,
		model.OrderSourceMiniapp, model.DanceTypeClassical, u11, "清理选手")
	f.fillMaterials(t, seed.RegistrationID, allMaterialFields...)

	dir := t.TempDir()
	files, err := storage.New(dir)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	photoRel := fmt.Sprintf("uploads/%d/%s/photo_x.jpg", f.eventID, seed.PlayerNo)
	audioRel := fmt.Sprintf("uploads/%d/%s/audio_x.mp3", f.eventID, seed.PlayerNo)
	for _, rel := range []string{photoRel, audioRel} {
		abs := filepath.Join(dir, filepath.FromSlash(rel[len("uploads/"):]))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(abs, []byte("media"), 0o644); err != nil {
			t.Fatalf("write media: %v", err)
		}
	}
	mustExecSQL(t, f.db, `UPDATE registrations SET photo_path = ?, audio_path = ? WHERE id = ?`,
		photoRel, audioRel, seed.RegistrationID)
	mustExecSQL(t, f.db, `UPDATE events SET status = 'finished',
		event_date = '2020-01-01', start_at = '2020-01-01T09:00:00+08:00',
		material_check_at = NULL WHERE id = ?`, f.eventID)

	job := NewMaterialCleanupJob(f.stores, files, 0)
	if err := job.RunOnce(ctx); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	var photo, audio, program *string
	var cleanedAt *string
	if err := f.db.SQL().QueryRowContext(ctx,
		`SELECT photo_path, audio_path, program_name FROM registrations WHERE id = ?`,
		seed.RegistrationID).Scan(&photo, &audio, &program); err != nil {
		t.Fatalf("query registration: %v", err)
	}
	if photo != nil || audio != nil {
		t.Fatalf("media paths should be NULL, got photo=%v audio=%v", photo, audio)
	}
	if program == nil || *program == "" {
		t.Fatal("text materials must be preserved")
	}
	if err := f.db.SQL().QueryRowContext(ctx,
		`SELECT materials_cleaned_at FROM events WHERE id = ?`, f.eventID).Scan(&cleanedAt); err != nil {
		t.Fatalf("query event: %v", err)
	}
	if cleanedAt == nil || *cleanedAt == "" {
		t.Fatal("materials_cleaned_at should be set")
	}
	if _, err := os.Stat(filepath.Join(dir, fmt.Sprint(f.eventID))); !os.IsNotExist(err) {
		t.Fatalf("event media dir should be removed, stat err = %v", err)
	}
	if n := countAdminLogAction(t, f.db, adminActionMaterialsCleaned); n != 1 {
		t.Fatalf("cleanup logs = %d, want 1", n)
	}

	// 幂等：再次执行不再处理已标记赛事。
	if err := job.RunOnce(ctx); err != nil {
		t.Fatalf("cleanup second run: %v", err)
	}
	if n := countAdminLogAction(t, f.db, adminActionMaterialsCleaned); n != 1 {
		t.Fatalf("cleanup logs after rerun = %d, want 1", n)
	}
}

// TestMaterialCleanupJobRespectsGrace 覆盖宽限期：结束时刻未到（或刚结束）不清。
func TestMaterialCleanupJobRespectsGrace(t *testing.T) {
	f := newAdminQueryFixture(t)
	ctx := context.Background()
	mustExecSQL(t, f.db, `UPDATE events SET status = 'finished',
		event_date = '2999-01-01', start_at = '2999-01-01T09:00:00+08:00' WHERE id = ?`, f.eventID)

	dir := t.TempDir()
	files, err := storage.New(dir)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	job := NewMaterialCleanupJob(f.stores, files, 72)
	if err := job.RunOnce(ctx); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	var cleanedAt *string
	if err := f.db.SQL().QueryRowContext(ctx,
		`SELECT materials_cleaned_at FROM events WHERE id = ?`, f.eventID).Scan(&cleanedAt); err != nil {
		t.Fatalf("query event: %v", err)
	}
	if cleanedAt != nil {
		t.Fatalf("event within grace must not be cleaned, got %v", *cleanedAt)
	}
}
