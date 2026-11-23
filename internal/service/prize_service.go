package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// PrizeService 是奖金明细单的管理服务（T31）。
type PrizeService struct {
	stores *store.Store
}

// NewPrizeService 构造奖金服务。
func NewPrizeService(stores *store.Store) *PrizeService {
	return &PrizeService{stores: stores}
}

// List 返回赛季奖金明细（可按状态筛选）。
func (s *PrizeService) List(ctx context.Context, seasonID int64, status string) ([]model.AdminPrizeStatementItem, error) {
	if status != "" && status != "pending" && status != "granted" && status != "skipped" {
		return nil, httpx.ErrBadRequest("status 取值不合法（pending / granted / skipped）")
	}
	rows, err := s.stores.Prizes.ListBySeason(ctx, seasonID, status)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载奖金明细失败，请稍后重试")
	}
	events, _, err := s.stores.Events.List(ctx, store.EventFilter{Page: 1, PageSize: 1000})
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载赛事失败，请稍后重试")
	}
	exclude := make(map[int64]bool, len(events))
	for _, e := range events {
		exclude[e.ID] = e.ExcludeFromPrize
	}

	out := make([]model.AdminPrizeStatementItem, 0, len(rows))
	for _, r := range rows {
		item := model.AdminPrizeStatementItem{
			ItemID: r.Prize.ID, EventID: r.Prize.EventID, PlayerID: r.Prize.PlayerID,
			PlayerName: r.PlayerName, Institution: r.Prize.InstitutionName,
			Award: r.Prize.Award, Amount: r.Prize.Amount, AdjustedAmount: r.Prize.Amount,
			Paid: r.Prize.Status == "granted", PaidAt: r.Prize.GrantedAt,
			Payee: r.Prize.GranteeNote,
		}
		if r.EventName != nil {
			item.EventName = *r.EventName
		}
		if r.Prize.EventID != nil {
			item.ExcludeFromPrize = exclude[*r.Prize.EventID]
		}
		out = append(out, item)
	}
	return out, nil
}

// Adjust 调整未发放奖金金额（留痕）。
func (s *PrizeService) Adjust(ctx context.Context, adminID, itemID int64, req model.AdjustPrizeRequest) error {
	if req.Amount < 0 {
		return httpx.ErrBadRequest("奖金金额不能为负")
	}
	return s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		prize, err := s.stores.Prizes.Get(ctx, itemID)
		if errors.Is(err, store.ErrNotFound) {
			return httpx.ErrNotFound("奖金明细不存在")
		}
		if err != nil {
			return err
		}
		if prize.Status != "pending" {
			return httpx.ErrStateDenied("仅未发放奖金可调整")
		}
		if err := s.stores.Prizes.UpdateAmount(ctx, tx, itemID, req.Amount); err != nil {
			if errors.Is(err, store.ErrConflict) {
				return httpx.ErrStateDenied("奖金状态已变化，请刷新后重试")
			}
			return err
		}
		targetType := "prize"
		return adminLog(ctx, s.stores, tx, &adminID, adminActionPrizeAdjust,
			&targetType, &itemID, map[string]any{
				"from": prize.Amount, "to": req.Amount, "reason": req.Reason,
			})
	})
}

// Pay 登记发放（pending → granted）。
func (s *PrizeService) Pay(ctx context.Context, adminID, itemID int64, req model.PayPrizeRequest) error {
	now := utils.NowISO()
	return s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		note := req.Payee
		if req.Note != "" {
			note = fmt.Sprintf("%s｜%s", req.Payee, req.Note)
		}
		if err := s.stores.Prizes.MarkGranted(ctx, tx, itemID, now, note); err != nil {
			if errors.Is(err, store.ErrConflict) {
				return httpx.ErrStateDenied("奖金不存在或已发放")
			}
			return err
		}
		targetType := "prize"
		return adminLog(ctx, s.stores, tx, &adminID, adminActionPrizePaid,
			&targetType, &itemID, map[string]any{"payee": req.Payee, "note": req.Note})
	})
}
