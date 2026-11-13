package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/meetmonth/dance-elite/internal/model"
)

// AdminAccountFilter 是后台账号列表的筛选与分页条件（API 文档 14.1）。
type AdminAccountFilter struct {
	Keyword  string // 用户名 / 姓名模糊
	Page     int
	PageSize int
}

// ListAccounts 分页查询后台账号列表（不含密码哈希）。
func (s *AdminStore) ListAccounts(ctx context.Context, f AdminAccountFilter) ([]model.AdminAccountItem, int64, error) {
	where := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		where = append(where, "(username LIKE ? OR name LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like)
	}
	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := s.db.sql.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM admins`+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count admin accounts: %w", err)
	}

	rows, err := s.db.sql.QueryContext(ctx, `
SELECT id, username, COALESCE(name, ''), role, status, last_login_at, created_at
FROM admins`+whereSQL+` ORDER BY id LIMIT ? OFFSET ?`,
		append(args, f.PageSize, (f.Page-1)*f.PageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list admin accounts: %w", err)
	}
	defer rows.Close()

	list := make([]model.AdminAccountItem, 0, f.PageSize)
	for rows.Next() {
		var item model.AdminAccountItem
		if err := rows.Scan(&item.AdminID, &item.Username, &item.Name, &item.Role,
			&item.Status, &item.LastLoginAt, &item.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan admin account: %w", err)
		}
		list = append(list, item)
	}
	return list, total, rows.Err()
}

// CreateAdminParams 是创建后台账号的写入参数（API 文档 14.1）。
type CreateAdminParams struct {
	Username     string
	Name         string
	PasswordHash string
	Role         string
}

// CreateAdmin 在事务内插入后台账号，返回新账号 ID。
// 调用方须先保证 username 唯一（UsernameExists 预检）。
func (s *AdminStore) CreateAdmin(ctx context.Context, tx *sql.Tx, p CreateAdminParams) (int64, error) {
	res, err := tx.ExecContext(ctx, `
INSERT INTO admins (username, name, password_hash, role, status)
VALUES (?, ?, ?, ?, 'active')`, p.Username, p.Name, p.PasswordHash, p.Role)
	if err != nil {
		return 0, fmt.Errorf("insert admin %q: %w", p.Username, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id of admin %q: %w", p.Username, err)
	}
	return id, nil
}

// UsernameExists 检查用户名是否已存在（创建账号预检，冲突报 40001）。
func (s *AdminStore) UsernameExists(ctx context.Context, username string) (bool, error) {
	var n int
	if err := s.db.sql.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM admins WHERE username = ?`, username).Scan(&n); err != nil {
		return false, fmt.Errorf("check username %q: %w", username, err)
	}
	return n > 0, nil
}

// UpdateAdminParams 是更新后台账号的写入参数（API 文档 14.1 PUT）。
// 各字段 nil 表示不修改。
type UpdateAdminParams struct {
	Status       *string
	PasswordHash *string
	Role         *string
}

// UpdateAdmin 在事务内按非 nil 字段更新后台账号。
func (s *AdminStore) UpdateAdmin(ctx context.Context, tx *sql.Tx, adminID int64, p UpdateAdminParams) error {
	sets := make([]string, 0, 3)
	args := make([]any, 0, 4)
	if p.Status != nil {
		sets = append(sets, "status = ?")
		args = append(args, *p.Status)
	}
	if p.PasswordHash != nil {
		sets = append(sets, "password_hash = ?")
		args = append(args, *p.PasswordHash)
	}
	if p.Role != nil {
		sets = append(sets, "role = ?")
		args = append(args, *p.Role)
	}
	if len(sets) == 0 {
		return errors.New("update admin: no fields to update")
	}
	args = append(args, adminID)
	res, err := tx.ExecContext(ctx,
		`UPDATE admins SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		return fmt.Errorf("update admin %d: %w", adminID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected of admin %d update: %w", adminID, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// AdminLogFilter 是操作日志列表的筛选与分页条件（API 文档 14.2）。
type AdminLogFilter struct {
	AdminID     int64
	Action      string
	TargetType  string
	TargetID    int64
	CreatedFrom string
	CreatedTo   string
	Page        int
	PageSize    int
}

// AdminLogRow 是操作日志的一行（admin_logs + 操作人姓名）。
type AdminLogRow struct {
	LogID      int64
	AdminName  string
	Action     string
	TargetType *string
	TargetID   *int64
	Detail     *string // JSON 文本，nil 表示无详情
	CreatedAt  string
}

// ListLogs 分页查询操作日志（倒序），按操作人/动作/对象/时间范围筛选。
func (s *AdminStore) ListLogs(ctx context.Context, f AdminLogFilter) ([]AdminLogRow, int64, error) {
	where := make([]string, 0, 5)
	args := make([]any, 0, 5)
	if f.AdminID > 0 {
		where = append(where, "l.admin_id = ?")
		args = append(args, f.AdminID)
	}
	if f.Action != "" {
		where = append(where, "l.action = ?")
		args = append(args, f.Action)
	}
	if f.TargetType != "" {
		where = append(where, "l.target_type = ?")
		args = append(args, f.TargetType)
	}
	if f.TargetID > 0 {
		where = append(where, "l.target_id = ?")
		args = append(args, f.TargetID)
	}
	if f.CreatedFrom != "" {
		where = append(where, "l.created_at >= ?")
		args = append(args, f.CreatedFrom)
	}
	if f.CreatedTo != "" {
		where = append(where, "l.created_at <= ?")
		args = append(args, f.CreatedTo)
	}
	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := s.db.sql.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM admin_logs l`+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count admin logs: %w", err)
	}

	rows, err := s.db.sql.QueryContext(ctx, `
SELECT l.id, COALESCE(a.name, a.username), l.action, l.target_type, l.target_id,
       l.detail, l.created_at
FROM admin_logs l
LEFT JOIN admins a ON a.id = l.admin_id`+whereSQL+`
ORDER BY l.id DESC LIMIT ? OFFSET ?`,
		append(args, f.PageSize, (f.Page-1)*f.PageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list admin logs: %w", err)
	}
	defer rows.Close()

	list := make([]AdminLogRow, 0, f.PageSize)
	for rows.Next() {
		var r AdminLogRow
		if err := rows.Scan(&r.LogID, &r.AdminName, &r.Action, &r.TargetType,
			&r.TargetID, &r.Detail, &r.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan admin log: %w", err)
		}
		list = append(list, r)
	}
	return list, total, rows.Err()
}
