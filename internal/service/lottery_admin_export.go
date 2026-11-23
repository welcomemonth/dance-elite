package service

import (
	"context"
	"fmt"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/pkg/logger"
	"github.com/meetmonth/dance-elite/internal/pkg/secureid"
	"github.com/meetmonth/dance-elite/internal/utils"
	"github.com/xuri/excelize/v2"
)

// lotteryExportSheet 是抽签结果导出工作表名。
const lotteryExportSheet = "抽签结果"

// Export 导出单场赛事的抽签结果（姓名、身份证号、抽签信息）。
//
// Excel 导入的选手存有加密证件号，导出时解密回明文；从赛事报名生成的
// 选手报名库只有哈希，身份证号列回退为掩码。
func (s *AdminLotteryService) Export(ctx context.Context, adminID int64, token string) (*ExportFile, error) {
	session, _, players, err := s.load(ctx, token)
	if err != nil {
		return nil, err
	}
	content, err := s.buildLotteryWorkbook(ctx, players)
	if err != nil {
		return nil, httpx.ErrInternal(err, "生成导出文件失败，请稍后重试")
	}
	if err := s.audit(ctx, adminID, adminActionLotteryExport, session.EventID, map[string]any{
		"players": len(players),
	}); err != nil {
		return nil, err
	}
	return &ExportFile{
		Name:    lotteryExportName(session, s.now()),
		Content: content,
		Rows:    len(players),
	}, nil
}

// buildLotteryWorkbook 生成抽签结果 Excel。
func (s *AdminLotteryService) buildLotteryWorkbook(ctx context.Context, players []model.LotteryPlayer) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if err := f.SetSheetName("Sheet1", lotteryExportSheet); err != nil {
		return nil, fmt.Errorf("rename lottery sheet: %w", err)
	}

	headers := []any{"序号", "姓名", "身份证号", "年龄组", "舞种",
		"分组", "组内序号", "出场顺序", "抽签时间", "抽签方式"}
	if err := f.SetSheetRow(lotteryExportSheet, "A1", &headers); err != nil {
		return nil, fmt.Errorf("write lottery header: %w", err)
	}
	sorted := lotterySortPlayers(players)
	for i, player := range sorted {
		row := i + 2
		idCard := s.exportIDCard(ctx, player)
		values := []any{
			i + 1,
			player.Name,
			idCard,
			player.AgeGroup,
			lotteryGenreLabel(player.Genre),
			lotteryGroupText(player),
			lotterySlotNo(player),
			lotteryOrderText(player),
			lotteryDrawnAtText(player),
			lotteryDrawMode(player),
		}
		cell := fmt.Sprintf("A%d", row)
		if err := f.SetSheetRow(lotteryExportSheet, cell, &values); err != nil {
			return nil, fmt.Errorf("write lottery row %d: %w", i+1, err)
		}
		// 身份证号写成文本，避免 Excel 自动转数字/科学计数法。
		if err := f.SetCellStr(lotteryExportSheet, fmt.Sprintf("C%d", row), idCard); err != nil {
			return nil, fmt.Errorf("write lottery id card row %d: %w", i+1, err)
		}
	}
	if err := styleLotterySheet(f); err != nil {
		return nil, err
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write lottery workbook: %w", err)
	}
	return buf.Bytes(), nil
}

// exportIDCard 优先解密入库的证件号；无密文（赛事报名来源）回退掩码。
func (s *AdminLotteryService) exportIDCard(ctx context.Context, player model.LotteryPlayer) string {
	if player.IDCardEnc != nil && *player.IDCardEnc != "" {
		plain, err := secureid.Decrypt(s.encKey, *player.IDCardEnc)
		if err == nil {
			return plain
		}
		logger.WarnEvent(ctx, "lottery.id_decrypt_failed", "抽签导出解密证件号失败",
			"player_id", player.ID, "error", err)
	}
	if player.IDCardMasked != nil {
		return *player.IDCardMasked
	}
	return "（报名库无明文）"
}

// styleLotterySheet 设置表头样式、列宽与首行冻结。
func styleLotterySheet(f *excelize.File) error {
	style, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"DDEBF7"}},
	})
	if err != nil {
		return fmt.Errorf("create lottery export style: %w", err)
	}
	if err := f.SetCellStyle(lotteryExportSheet, "A1", "J1", style); err != nil {
		return fmt.Errorf("apply lottery header style: %w", err)
	}
	widths := map[string]float64{
		"A": 6, "B": 12, "C": 22, "D": 8, "E": 14,
		"F": 8, "G": 10, "H": 10, "I": 20, "J": 14,
	}
	for column, width := range widths {
		if err := f.SetColWidth(lotteryExportSheet, column, column, width); err != nil {
			return fmt.Errorf("set lottery column %s width: %w", column, err)
		}
	}
	if err := f.SetPanes(lotteryExportSheet, &excelize.Panes{
		Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft",
	}); err != nil {
		return fmt.Errorf("freeze lottery header: %w", err)
	}
	return nil
}

// lotteryExportName 生成导出文件名（不含赛事名，避免文件名非法字符）。
func lotteryExportName(session *model.LotterySession, now time.Time) string {
	key := fmt.Sprintf("lottery_%d", session.ID)
	if session.EventID != nil {
		key = fmt.Sprintf("lottery_%d", *session.EventID)
	}
	return fmt.Sprintf("%s_%s.xlsx", key, now.Format("20060102"))
}

// lotteryGroupText 返回「A组」；未抽签返回空。
func lotteryGroupText(player model.LotteryPlayer) string {
	if player.GroupLetter == nil {
		return ""
	}
	return *player.GroupLetter + "组"
}

// lotterySlotNo 返回组内序号；未抽签返回空。
func lotterySlotNo(player model.LotteryPlayer) any {
	if player.SlotNo == nil {
		return ""
	}
	return *player.SlotNo
}

// lotteryOrderText 返回出场顺序 A3；未抽签返回「未抽签」。
func lotteryOrderText(player model.LotteryPlayer) string {
	if player.SlotNo == nil || player.GroupLetter == nil {
		return "未抽签"
	}
	return lotterySlotText(model.LotterySlot{Letter: *player.GroupLetter, No: *player.SlotNo})
}

// lotteryDrawnAtText 把抽签时间转成 Excel 友好的本地时间。
func lotteryDrawnAtText(player model.LotteryPlayer) string {
	if player.DrawnAt == nil || *player.DrawnAt == "" {
		return ""
	}
	t, err := utils.ParseISO(*player.DrawnAt)
	if err != nil {
		return *player.DrawnAt
	}
	return t.In(utils.ChinaLoc).Format("2006-01-02 15:04")
}

// lotteryDrawMode 返回抽签方式展示文案。
func lotteryDrawMode(player model.LotteryPlayer) string {
	switch {
	case player.SlotNo == nil:
		return ""
	case player.AutoDrawn:
		return "系统自动分配"
	default:
		return "本人抽取"
	}
}
