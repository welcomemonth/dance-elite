package service

import (
	"fmt"
	"unicode/utf8"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"golang.org/x/crypto/bcrypt"
)

// minAdminPasswordLen 是后台账号初始/重置密码的最小长度（API 文档 14.1）。
const minAdminPasswordLen = 8

// maxAdminNameLen 是后台账号姓名的最大长度。
const maxAdminNameLen = 50

// validateCreateAdmin 校验创建账号请求（API 文档 14.1：用户名重复/密码过弱 → 40001）。
func validateCreateAdmin(req model.CreateAdminRequest) error {
	if err := validateAdminUsername(req.Username); err != nil {
		return err
	}
	if err := validateAdminPassword(req.Password); err != nil {
		return err
	}
	if !isValidAdminRole(req.Role) {
		return httpx.ErrBadRequest("角色不合法，仅支持 super_admin / admin / cs")
	}
	if err := validateAdminName(req.Name); err != nil {
		return err
	}
	return nil
}

// validateUpdateAdmin 校验更新账号请求：至少一个字段，status/role 合法。
func validateUpdateAdmin(req model.UpdateAdminRequest) error {
	if req.Status == "" && req.Password == "" && req.Role == "" {
		return httpx.ErrBadRequest("至少提供一个待修改字段：status / password / role")
	}
	if req.Status != "" && req.Status != model.AdminStatusActive && req.Status != model.AdminStatusDisabled {
		return httpx.ErrBadRequest("status 仅支持 active / disabled")
	}
	if req.Password != "" {
		if err := validateAdminPassword(req.Password); err != nil {
			return err
		}
	}
	if req.Role != "" && !isValidAdminRole(req.Role) {
		return httpx.ErrBadRequest("角色不合法，仅支持 super_admin / admin / cs")
	}
	return nil
}

// validateAdminUsername 校验登录账号：3~32 位字母数字下划线。
func validateAdminUsername(username string) error {
	if len(username) < 3 || len(username) > 32 {
		return httpx.ErrBadRequest("用户名须为 3~32 个字符")
	}
	for _, r := range username {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
		default:
			return httpx.ErrBadRequest("用户名仅支持字母、数字与下划线")
		}
	}
	return nil
}

// validateAdminPassword 校验密码长度（bcrypt 上限 72 字节）。
func validateAdminPassword(password string) error {
	if len(password) < minAdminPasswordLen {
		return httpx.ErrBadRequest(fmt.Sprintf("密码长度不能少于 %d 位", minAdminPasswordLen))
	}
	if len(password) > 72 {
		return httpx.ErrBadRequest("密码长度不能超过 72 字节")
	}
	return nil
}

// validateAdminName 校验姓名非空且不超长。
func validateAdminName(name string) error {
	if name == "" || utf8.RuneCountInString(name) > maxAdminNameLen {
		return httpx.ErrBadRequest("姓名不能为空且不能超过 50 字")
	}
	return nil
}

// isValidAdminRole 判断角色枚举值。
func isValidAdminRole(role string) bool {
	switch role {
	case model.RoleSuperAdmin, model.RoleAdmin, model.RoleCS:
		return true
	}
	return false
}

// hashAdminPassword 生成 bcrypt 密码哈希。
func hashAdminPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash admin password: %w", err)
	}
	return string(hash), nil
}
