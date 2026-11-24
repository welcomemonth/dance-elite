package service

import (
	"context"
	"testing"

	"github.com/meetmonth/dance-elite/internal/store"
)

// countAdminLogAction 统计指定审计动作的条数（用于无 target_id 的导出/任务日志）。
func countAdminLogAction(t *testing.T, db *store.DB, action string) int {
	t.Helper()
	var n int
	if err := db.SQL().QueryRowContext(context.Background(),
		`SELECT COUNT(1) FROM admin_logs WHERE action = ?`, action).Scan(&n); err != nil {
		t.Fatalf("count admin log %q: %v", action, err)
	}
	return n
}
