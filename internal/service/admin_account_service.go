package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/pkg/token"
	"github.com/meetmonth/dance-elite/internal/store"
)

// accountLogActions 是账号管理写入 admin_logs 的动作名（API 文档 14.1 注意事项）。
const (
	actionAccountCreate = "account_create"
	actionAccountUpdate = "account_update"
)

// targetTypeAdmin 是账号管理日志的操作对象类型。
const targetTypeAdmin = "admin"

// AdminAccountService 是后台账号管理服务（联调 B4，API 文档 14.1）。
// 仅 super_admin 可用（路由层 RequireRole 保证）；所有变更记操作日志。
type AdminAccountService struct {
	stores *store.Store
	signer *token.Signer
}

// NewAdminAccountService 构造后台账号管理服务。
func NewAdminAccountService(stores *store.Store, signer *token.Signer) *AdminAccountService {
	return &AdminAccountService{stores: stores, signer: signer}
}

// List 返回后台账号分页列表（API 文档 14.1）。
func (s *AdminAccountService) List(ctx context.Context, keyword string, page, pageSize int) (*model.AdminAccountListResponse, error) {
	page, pageSize = normalizePage(page, pageSize)
	list, total, err := s.stores.Admins.ListAccounts(ctx, store.AdminAccountFilter{
		Keyword: keyword, Page: page, PageSize: pageSize,
	})
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载账号列表失败，请稍后重试")
	}
	if list == nil {
		list = make([]model.AdminAccountItem, 0)
	}
	return &model.AdminAccountListResponse{List: list, Total: total, Page: page, PageSize: pageSize}, nil
}

// Create 创建后台账号（API 文档 14.1）：用户名唯一，初始密码 ≥8 位，记操作日志。
func (s *AdminAccountService) Create(ctx context.Context, actorID int64, req model.CreateAdminRequest) (*model.CreateAdminResponse, error) {
	if err := validateCreateAdmin(req); err != nil {
		return nil, err
	}
	exists, err := s.stores.Admins.UsernameExists(ctx, req.Username)
	if err != nil {
		return nil, httpx.ErrInternal(err, "创建账号失败，请稍后重试")
	}
	if exists {
		return nil, httpx.ErrBadRequest("用户名已存在")
	}
	hash, err := hashAdminPassword(req.Password)
	if err != nil {
		return nil, httpx.ErrInternal(err, "创建账号失败，请稍后重试")
	}

	var adminID int64
	err = s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		id, err := s.stores.Admins.CreateAdmin(ctx, tx, store.CreateAdminParams{
			Username: req.Username, Name: req.Name, PasswordHash: hash, Role: req.Role,
		})
		if err != nil {
			return err
		}
		adminID = id
		targetType := targetTypeAdmin
		targetID := id
		return adminLog(ctx, s.stores, tx, &actorID, actionAccountCreate, &targetType, &targetID,
			map[string]any{"username": req.Username, "role": req.Role, "name": req.Name})
	})
	if err != nil {
		return nil, httpx.ErrInternal(err, "创建账号失败，请稍后重试")
	}
	return &model.CreateAdminResponse{AdminID: adminID, Username: req.Username, Role: req.Role}, nil
}

// Update 更新账号状态/密码/角色（API 文档 14.1）：停用立即踢下线，变更记操作日志。
// 不允许停用或降级自己（防止最后一个 super_admin 失去管理入口）。
func (s *AdminAccountService) Update(ctx context.Context, actorID int64, adminID int64, req model.UpdateAdminRequest) (*model.UpdateAdminResponse, error) {
	if err := validateUpdateAdmin(req); err != nil {
		return nil, err
	}
	target, err := s.stores.Admins.Get(ctx, adminID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, httpx.ErrNotFound("账号不存在")
	}
	if err != nil {
		return nil, httpx.ErrInternal(err, "更新账号失败，请稍后重试")
	}
	if adminID == actorID && (req.Status == model.AdminStatusDisabled || (req.Role != "" && req.Role != model.RoleSuperAdmin)) {
		return nil, httpx.ErrConflict("不能停用或降级当前登录账号")
	}

	params := store.UpdateAdminParams{}
	if req.Status != "" {
		params.Status = &req.Status
	}
	if req.Role != "" {
		params.Role = &req.Role
	}
	if req.Password != "" {
		hash, err := hashAdminPassword(req.Password)
		if err != nil {
			return nil, httpx.ErrInternal(err, "更新账号失败，请稍后重试")
		}
		params.PasswordHash = &hash
	}

	err = s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		if err := s.stores.Admins.UpdateAdmin(ctx, tx, adminID, params); err != nil {
			return err
		}
		targetType := targetTypeAdmin
		targetID := adminID
		detail := map[string]any{"username": target.Username}
		if req.Status != "" {
			detail["before_status"] = target.Status
			detail["after_status"] = req.Status
		}
		if req.Role != "" {
			detail["before_role"] = target.Role
			detail["after_role"] = req.Role
		}
		if req.Password != "" {
			detail["password_reset"] = true
		}
		return adminLog(ctx, s.stores, tx, &actorID, actionAccountUpdate, &targetType, &targetID, detail)
	})
	if err != nil {
		return nil, httpx.ErrInternal(err, "更新账号失败，请稍后重试")
	}

	// 停用立即踢下线（API 文档 14.1 注意事项）；重新激活解除标记。
	switch req.Status {
	case model.AdminStatusDisabled:
		s.signer.RevokeAdmin(adminID)
	case model.AdminStatusActive:
		s.signer.RestoreAdmin(adminID)
	}
	return &model.UpdateAdminResponse{AdminID: adminID}, nil
}
