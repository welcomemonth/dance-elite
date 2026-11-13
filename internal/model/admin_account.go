package model

// AdminAccountItem 是后台账号列表单条记录（API 文档 14.1）。
// 不含密码哈希等敏感字段。
type AdminAccountItem struct {
	AdminID     int64   `json:"admin_id"`
	Username    string  `json:"username"`
	Name        string  `json:"name"`
	Role        string  `json:"role"`
	Status      string  `json:"status"`
	LastLoginAt *string `json:"last_login_at"`
	CreatedAt   string  `json:"created_at"`
}

// AdminAccountListResponse 是后台账号列表响应（统一分页结构）。
type AdminAccountListResponse struct {
	List     []AdminAccountItem `json:"list"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

// CreateAdminRequest 是 POST /api/v1/admin/accounts 的请求体（API 文档 14.1）。
type CreateAdminRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role" binding:"required"`
	Name     string `json:"name" binding:"required"`
}

// CreateAdminResponse 是创建账号响应。
type CreateAdminResponse struct {
	AdminID  int64  `json:"admin_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// UpdateAdminRequest 是 PUT /api/v1/admin/accounts/{admin_id} 的请求体（API 文档 14.1）。
// status/password/role 至少提供一个；status 取 active/disabled。
type UpdateAdminRequest struct {
	Status   string `json:"status"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// UpdateAdminResponse 是更新账号响应。
type UpdateAdminResponse struct {
	AdminID int64 `json:"admin_id"`
}

// AdminLogListQuery 是操作日志查询参数（API 文档 14.2）。
type AdminLogListQuery struct {
	AdminID     int64
	Action      string
	TargetType  string
	CreatedFrom string // ISO8601，全库统一时间格式
	CreatedTo   string
	Page        int
	PageSize    int
}

// AdminLogItem 是操作日志单条记录（API 文档 14.2）。
// Detail 为操作详情对象（admin_logs.detail JSON 文本解析结果）。
type AdminLogItem struct {
	LogID      int64          `json:"log_id"`
	AdminName  string         `json:"admin_name"`
	Action     string         `json:"action"`
	TargetType *string        `json:"target_type"`
	TargetID   *int64         `json:"target_id"`
	Detail     map[string]any `json:"detail"`
	CreatedAt  string         `json:"created_at"`
}

// AdminLogListResponse 是操作日志列表响应（统一分页结构）。
type AdminLogListResponse struct {
	List     []AdminLogItem `json:"list"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}
