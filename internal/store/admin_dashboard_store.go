package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/meetmonth/dance-elite/internal/model"
)

// DashboardStore 是数据看板的聚合查询（API 文档 9.1）。只读，全部走索引扫描。
type DashboardStore struct {
	db *DB
}

// NewDashboardStore 构造看板聚合查询。
func NewDashboardStore(db *DB) *DashboardStore { return &DashboardStore{db: db} }

// Metrics 查询核心指标卡。today 为 YYYY-MM-DD（本地时区），与 substr(created_at,1,10) 对齐。
func (s *DashboardStore) Metrics(ctx context.Context, today string) (model.DashboardMetrics, error) {
	var m model.DashboardMetrics
	row := s.db.SQL().QueryRowContext(ctx, `
SELECT
 (SELECT COUNT(*) FROM registrations WHERE status = 'active' AND substr(created_at,1,10) = @today),
 (SELECT COALESCE(SUM(amount),0) FROM orders
   WHERE status IN ('paid','refunding','refunded') AND substr(paid_at,1,10) = @today),
 (SELECT COUNT(*) FROM registrations WHERE status = 'active'),
 (SELECT COALESCE(SUM(amount),0) FROM orders WHERE status IN ('paid','refunding','refunded')),
 (SELECT COUNT(*) FROM refund_tickets WHERE status IN ('pending','processing')),
 (SELECT COUNT(*) FROM waitlists WHERE status IN ('waiting','notified')),
 (SELECT COUNT(*) FROM registrations WHERE status = 'active'
   AND (photo_path IS NULL OR photo_path = '' OR audio_path IS NULL OR audio_path = ''
        OR program_name IS NULL OR program_name = '')),
 (SELECT COUNT(*) FROM events WHERE status IN ('registering','ongoing'))`,
		sql.Named("today", today))
	if err := row.Scan(&m.TodayRegistrations, &m.TodayRevenue, &m.TotalRegistrations,
		&m.TotalRevenue, &m.PendingRefundTickets, &m.WaitlistTotal, &m.IncompleteMaterials,
		&m.ActiveEvents); err != nil {
		return m, fmt.Errorf("dashboard metrics: %w", err)
	}
	return m, nil
}

// ClosingEvents 查询临近报名截止的进行中赛事（reg_end_at 在 [now, now+horizon] 内）。
// now 为全库统一 ISO8601 时间格式（带 +08:00 后缀，julianday 可解析）。
func (s *DashboardStore) ClosingEvents(ctx context.Context, now string, horizonDays int) ([]model.ClosingEvent, error) {
	rows, err := s.db.SQL().QueryContext(ctx, `
SELECT e.id, e.name,
       CAST(CEIL(julianday(e.reg_end_at) - julianday(@now)) AS INTEGER)
FROM events e
WHERE e.status = 'registering' AND e.reg_end_at IS NOT NULL
  AND julianday(e.reg_end_at) >= julianday(@now)
  AND julianday(e.reg_end_at) <= julianday(@now, '+' || @horizon || ' days')
ORDER BY e.reg_end_at`,
		sql.Named("now", now), sql.Named("horizon", horizonDays))
	if err != nil {
		return nil, fmt.Errorf("dashboard closing events: %w", err)
	}
	defer rows.Close()

	list := make([]model.ClosingEvent, 0, 8)
	for rows.Next() {
		var ev model.ClosingEvent
		if err := rows.Scan(&ev.EventID, &ev.EventName, &ev.DaysLeft); err != nil {
			return nil, fmt.Errorf("dashboard closing events scan: %w", err)
		}
		list = append(list, ev)
	}
	return list, rows.Err()
}

// EventProgress 查询各赛事组别报名进度（未下线/未草稿的赛事）。
func (s *DashboardStore) EventProgress(ctx context.Context) ([]model.EventProgress, error) {
	rows, err := s.db.SQL().QueryContext(ctx, `
SELECT e.id, e.name, g.level, g.dance_type, g.taken, g.quota,
 (SELECT COUNT(*) FROM waitlists w
   WHERE w.group_id = g.id AND w.status IN ('waiting','notified'))
FROM event_groups g
JOIN events e ON e.id = g.event_id
WHERE e.status IN ('registering','reg_closed','ongoing')
ORDER BY e.id, g.level, g.dance_type`)
	if err != nil {
		return nil, fmt.Errorf("dashboard event progress: %w", err)
	}
	defer rows.Close()

	list := make([]model.EventProgress, 0, 32)
	for rows.Next() {
		var p model.EventProgress
		if err := rows.Scan(&p.EventID, &p.EventName, &p.Level, &p.DanceType,
			&p.Registered, &p.Quota, &p.WaitlistCount); err != nil {
			return nil, fmt.Errorf("dashboard event progress scan: %w", err)
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// PendingCSTickets 查询待处理（pending）的客服工单数。
func (s *DashboardStore) PendingCSTickets(ctx context.Context) (int64, error) {
	var n int64
	if err := s.db.SQL().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM cs_tickets WHERE status = 'pending'`).Scan(&n); err != nil {
		return 0, fmt.Errorf("dashboard pending cs tickets: %w", err)
	}
	return n, nil
}

// Trend 查询近 30 日（含今日，本地日期）的报名笔数与营收，按日期升序补齐无数据日。
// today 为 YYYY-MM-DD。
func (s *DashboardStore) Trend(ctx context.Context, today string) ([]model.TrendPoint, error) {
	rows, err := s.db.SQL().QueryContext(ctx, `
WITH RECURSIVE days(d) AS (
  SELECT date(@today)
  UNION ALL
  SELECT date(d, '-1 day') FROM days LIMIT 30
)
SELECT d,
 (SELECT COUNT(*) FROM registrations
   WHERE status = 'active' AND substr(created_at,1,10) = d),
 (SELECT COALESCE(SUM(amount),0) FROM orders
   WHERE status IN ('paid','refunding','refunded') AND substr(paid_at,1,10) = d)
FROM days ORDER BY d`,
		sql.Named("today", today))
	if err != nil {
		return nil, fmt.Errorf("dashboard trend: %w", err)
	}
	defer rows.Close()

	list := make([]model.TrendPoint, 0, 30)
	for rows.Next() {
		var p model.TrendPoint
		if err := rows.Scan(&p.Date, &p.Registrations, &p.Revenue); err != nil {
			return nil, fmt.Errorf("dashboard trend scan: %w", err)
		}
		list = append(list, p)
	}
	return list, rows.Err()
}
