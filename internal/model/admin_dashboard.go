package model

// DashboardMetrics 是数据看板核心指标卡（API 文档 9.1）。
type DashboardMetrics struct {
	TodayRegistrations   int64 `json:"today_registrations"`
	TodayRevenue         int64 `json:"today_revenue"`
	TotalRegistrations   int64 `json:"total_registrations"`
	TotalRevenue         int64 `json:"total_revenue"`
	PendingRefundTickets int64 `json:"pending_refund_tickets"`
	WaitlistTotal        int64 `json:"waitlist_total"`
	IncompleteMaterials  int64 `json:"incomplete_materials"`
	ActiveEvents         int64 `json:"active_events"`
}

// ClosingEvent 是临近报名截止的赛事（API 文档 9.1 todos.closing_events）。
type ClosingEvent struct {
	EventID   int64  `json:"event_id"`
	EventName string `json:"event_name"`
	DaysLeft  int    `json:"days_left"`
}

// DashboardTodos 是数据看板待办事项（API 文档 9.1）。
type DashboardTodos struct {
	PendingRefunds int64          `json:"pending_refunds"`
	NewCSTickets   int64          `json:"new_cs_tickets"`
	ClosingEvents  []ClosingEvent `json:"closing_events"`
}

// EventProgress 是单个赛事组别的报名进度（API 文档 9.1 event_progress）。
type EventProgress struct {
	EventID       int64  `json:"event_id"`
	EventName     string `json:"event_name"`
	Level         string `json:"level"`
	DanceType     string `json:"dance_type"`
	Registered    int    `json:"registered"`
	Quota         int    `json:"quota"`
	WaitlistCount int64  `json:"waitlist_count"`
}

// TrendPoint 是近 30 日报名/营收趋势的一个日期点（API 文档 9.1 trend）。
type TrendPoint struct {
	Date          string `json:"date"`
	Registrations int64  `json:"registrations"`
	Revenue       int64  `json:"revenue"`
}

// Dashboard 是数据看板总览响应（API 文档 9.1）。
// 客服角色仅填充 Todos.NewCSTickets，其余字段按文档裁剪为零值。
type Dashboard struct {
	Metrics       DashboardMetrics `json:"metrics"`
	Todos         DashboardTodos   `json:"todos"`
	EventProgress []EventProgress  `json:"event_progress"`
	Trend         []TrendPoint     `json:"trend"`
}
