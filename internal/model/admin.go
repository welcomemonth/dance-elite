package model

// AdminRole 是 admins.role 的枚举值（API 文档 14.1）。
const (
	RoleSuperAdmin = "super_admin"
	RoleAdmin      = "admin"
	RoleCS         = "cs"
)

// AdminStatus 是 admins.status 的枚举值。
const (
	AdminStatusActive   = "active"
	AdminStatusDisabled = "disabled"
)

// Admin 是后台管理员（admins 表）。
//
// Username 是登录账号；Name 是真实姓名（后台展示用，可空）。
// PasswordHash 仅用于服务端 bcrypt 校验，绝不进入任何响应序列化。
type Admin struct {
	ID           int64
	Username     string
	Name         *string
	PasswordHash string
	Role         string
	Status       string
	LastLoginAt  *string
	CreatedAt    string
	UpdatedAt    string
}

// AdminInfo 是登录/账号响应中的管理员视图，不含敏感字段。
type AdminInfo struct {
	ID       int64   `json:"id"`
	Username string  `json:"username"`
	Role     string  `json:"role"`
	Name     *string `json:"name"`
	// Permissions 是敏感操作权限位；前端据此显隐「明文查看」等入口
	// （API 文档 11.3：手机号明文查看需独立权限，默认拒绝）。
	Permissions []string `json:"permissions"`
}

// PermissionRevealPhone 是手机号明文查看的权限位（API 文档 11.3）。
const PermissionRevealPhone = "reveal_phone"

// PermissionsForRole 返回角色的敏感权限位：管理员与超管可查看手机号明文，
// 客服只读且不授予（后台设计文档 0.2 权限矩阵）。
func PermissionsForRole(role string) []string {
	switch role {
	case RoleSuperAdmin, RoleAdmin:
		return []string{PermissionRevealPhone}
	default:
		return []string{}
	}
}

// AdminLoginRequest 是 POST /api/v1/admin/auth/login 的请求体（API 文档 8.1）。
type AdminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AdminLoginResponse 是管理员登录成功后的响应数据（API 文档 8.1）。
type AdminLoginResponse struct {
	Token     string    `json:"token"`
	ExpiresIn int       `json:"expires_in"`
	Admin     AdminInfo `json:"admin"`
}

// AdminChangePasswordRequest 是 PUT /api/v1/admin/auth/password 的请求体。
// 新密码要求 ≥8 位（与 API 文档 14.1 账号管理密码强度一致）。
type AdminChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}
