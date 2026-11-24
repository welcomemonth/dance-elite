package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/utils"
	"github.com/xuri/excelize/v2"
)

const (
	// adminActionScoreImport 是成绩 Excel 导入预览的审计动作。
	adminActionScoreImport = "admin.score.import"
	// scoreImportRowLimit 限制单次导入的数据行数，防止超大文件长时间占用写锁。
	scoreImportRowLimit = 5000
	// scoreImportSheetName 是导入模板的工作表名。
	scoreImportSheetName = "成绩导入"
)

// scoreImportColumns 是表头列在解析结果中的索引；-1 表示未提供该列。
type scoreImportColumns struct {
	name     int
	idNumber int
	award    int
	advanced int
	remark   int
}

// ParseScoreWorkbook 解析成绩导入 Excel（仅 .xlsx），按表头名称定位列。
//
// 必需列：姓名、证件号、奖项；可选列：晋级决赛、备注。证书号列请使用文本格式，
// 数字格式的 18 位身份证在 Excel 中会丢失精度（模板已做说明）。
func ParseScoreWorkbook(r io.Reader) ([]model.ScorePreviewInputRow, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, httpx.ErrBadRequest("Excel 解析失败：请上传 .xlsx 文件").WithCause(err)
	}
	defer func() { _ = f.Close() }()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, httpx.ErrBadRequest("Excel 中没有工作表")
	}
	all, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, httpx.ErrBadRequest("读取 Excel 内容失败").WithCause(err)
	}
	headerRow, cols, err := findScoreHeader(all)
	if err != nil {
		return nil, err
	}

	out := make([]model.ScorePreviewInputRow, 0, len(all)-headerRow-1)
	for i := headerRow + 1; i < len(all); i++ {
		cells := all[i]
		get := func(idx int) string {
			if idx < 0 || idx >= len(cells) {
				return ""
			}
			return strings.TrimSpace(cells[idx])
		}
		name, idNumber, award := get(cols.name), get(cols.idNumber), get(cols.award)
		advanced, remark := get(cols.advanced), get(cols.remark)
		if name == "" && idNumber == "" && award == "" && advanced == "" && remark == "" {
			continue
		}
		out = append(out, model.ScorePreviewInputRow{
			RowNo:           i + 1,
			PlayerName:      name,
			IDCardNo:        idNumber,
			Award:           normalizeImportAward(award),
			AdvancedToFinal: parseImportBool(advanced),
			Remark:          remark,
		})
	}
	if len(out) == 0 {
		return nil, httpx.ErrBadRequest("Excel 没有可导入的成绩行")
	}
	if len(out) > scoreImportRowLimit {
		return nil, httpx.ErrBadRequest(fmt.Sprintf("单次最多导入 %d 行，请拆分文件", scoreImportRowLimit))
	}
	return out, nil
}

// findScoreHeader 在前 20 行内定位表头，返回表头行下标与列映射。
func findScoreHeader(rows [][]string) (int, scoreImportColumns, error) {
	empty := scoreImportColumns{-1, -1, -1, -1, -1}
	for i := 0; i < len(rows) && i < 20; i++ {
		cols := empty
		for j, cell := range rows[i] {
			switch scoreHeaderKind(cell) {
			case "name":
				cols.name = j
			case "id":
				cols.idNumber = j
			case "award":
				cols.award = j
			case "advanced":
				cols.advanced = j
			case "remark":
				cols.remark = j
			}
		}
		if cols.name >= 0 && cols.idNumber >= 0 && cols.award >= 0 {
			return i, cols, nil
		}
	}
	return 0, empty, httpx.ErrBadRequest("未找到表头：Excel 需包含「姓名」「证件号」「奖项」列")
}

// scoreHeaderKind 把表头文本归一化为列类型（兼容常见别名）。
func scoreHeaderKind(raw string) string {
	h := strings.ToLower(strings.Join(strings.Fields(raw), ""))
	h = strings.TrimSuffix(h, "(必填)")
	switch h {
	case "姓名", "选手姓名", "选手", "name", "player_name":
		return "name"
	case "证件号", "身份证号", "身份证", "证件号码", "身份证号码", "id_card_no", "id_number":
		return "id"
	case "奖项", "名次", "获奖", "award":
		return "award"
	case "晋级决赛", "是否晋级", "晋级", "advanced_to_final":
		return "advanced"
	case "备注", "说明", "remark":
		return "remark"
	default:
		return ""
	}
}

// normalizeImportAward 把模板中的中文奖项名转换为 API 枚举；英文枚举原样返回。
func normalizeImportAward(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	lower := strings.ToLower(strings.Join(strings.Fields(s), ""))
	if model.IsValidAward(lower) {
		return lower
	}
	switch strings.TrimSpace(s) {
	case "冠军", "第一名", "冠军奖":
		return model.AwardChampion
	case "亚军", "第二名", "亚军奖":
		return model.AwardRunnerUp
	case "季军", "第三名", "季军奖":
		return model.AwardThird
	case "菁英舞者", "精英舞者":
		return model.AwardEliteDancer
	case "特金奖", "特别金奖":
		return model.AwardSpecialGold
	case "金奖":
		return model.AwardGold
	case "银奖":
		return model.AwardSilver
	default:
		return s
	}
}

func parseImportBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "是", "1", "true", "yes", "y", "晋级":
		return true
	default:
		return false
	}
}

// BuildScoreTemplateWorkbook 生成成绩导入模板（表头 + 填写说明）。
func BuildScoreTemplateWorkbook() ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if err := f.SetSheetName("Sheet1", scoreImportSheetName); err != nil {
		return nil, fmt.Errorf("rename score template sheet: %w", err)
	}
	headers := []any{"姓名", "证件号", "奖项", "晋级决赛", "备注"}
	if err := f.SetSheetRow(scoreImportSheetName, "A1", &headers); err != nil {
		return nil, fmt.Errorf("write score template header: %w", err)
	}
	widths := map[string]float64{"A": 18, "B": 28, "C": 14, "D": 12, "E": 32}
	if err := styleExportSheetRange(f, scoreImportSheetName, "E", widths); err != nil {
		return nil, err
	}

	notes := "填写说明"
	if _, err := f.NewSheet(notes); err != nil {
		return nil, fmt.Errorf("add score template notes: %w", err)
	}
	lines := []string{
		"1. 姓名、证件号、奖项为必填列；晋级决赛填「是」或留空；备注可选。",
		"2. 奖项可填：冠军 / 亚军 / 季军 / 菁英舞者 / 特金奖 / 金奖 / 银奖，或对应英文枚举。",
		"3. 证件号列请设置为「文本」格式，避免 18 位身份证被 Excel 转为科学计数法。",
		"4. 大陆身份证可在导入时自动建档；护照等其他证件需先在「人工报名」建档后再导入。",
		"5. 每次导入对应一个赛事的一个榜单；同一赛事 6 个榜单需分别导入。",
	}
	for i, line := range lines {
		cell := fmt.Sprintf("A%d", i+1)
		if err := f.SetCellValue(notes, cell, line); err != nil {
			return nil, fmt.Errorf("write score template note: %w", err)
		}
	}
	if err := f.SetColWidth(notes, "A", "A", 90); err != nil {
		return nil, fmt.Errorf("set score template note width: %w", err)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write score template workbook: %w", err)
	}
	return buf.Bytes(), nil
}

// ImportPreview 解析导入行：匹配已有选手；全部行无错误时，为有效身份证的新选手建档。
//
// 有错误行时不写任何选手档案，仅返回报告，由管理员修正文件后重新导入。
// 建档发生在预览阶段（结果写入仍需确认发布），玩家档案是全局复用数据，不影响成绩。
func (s *ScoreService) ImportPreview(ctx context.Context, adminID, eventID int64, level, dance string,
	rows []model.ScorePreviewInputRow) (*model.ScorePreviewResponse, error) {
	_, board, err := s.boardContext(ctx, eventID, level, dance)
	if err != nil {
		return nil, err
	}
	players, err := s.stores.Registrations.ListEventPlayers(ctx, eventID, level, dance)
	if err != nil {
		return nil, httpx.ErrInternal(err, "加载报名选手失败，请稍后重试")
	}
	batch, err := newScoreImportBatch(s.now())
	if err != nil {
		return nil, httpx.ErrInternal(err, "生成导入批次号失败，请稍后重试")
	}

	resp := &model.ScorePreviewResponse{
		TotalRows: len(rows), ImportBatch: batch,
		Rows: make([]model.ScorePreviewRowResult, 0, len(rows)),
	}
	type pendingCreate struct {
		index int
		row   model.ScorePreviewInputRow
	}
	var pending []pendingCreate
	seen := map[string]bool{}

	for i, row := range rows {
		item := model.ScorePreviewRowResult{
			RowNo: row.RowNo, PlayerName: strings.TrimSpace(row.PlayerName),
			AdvancedToFinal: row.AdvancedToFinal, MatchStatus: "unmatched",
		}
		if item.RowNo <= 0 {
			item.RowNo = i + 1
		}
		award := normalizeImportAward(row.Award)
		if award != "" {
			item.Award = strPtr(award)
		}
		if row.Remark != "" {
			item.Remark = strPtr(row.Remark)
		}
		if msg := scoreImportRowError(item.PlayerName, award); msg != "" {
			item.Error = strPtr(msg)
			resp.Errors++
			resp.Rows = append(resp.Rows, item)
			continue
		}

		rawID := strings.TrimSpace(row.IDCardNo)
		if rawID != "" {
			item.IDCardMasked = strPtr(utils.MaskIDNumber(rawID))
			if pid := s.matchByIDNumber(ctx, nil, rawID); pid > 0 {
				key := fmt.Sprintf("p:%d", pid)
				if seen[key] {
					item.Error = strPtr("同一选手在文件中重复出现")
					resp.Errors++
				} else {
					seen[key] = true
					item.PlayerID = &pid
					item.MatchStatus = "matched"
				}
			} else if _, _, _, err := utils.ParseIDCard(rawID); err == nil {
				key := fmt.Sprintf("id:%s", utils.NormalizeIDNumber(utils.IDTypeIDCard, rawID))
				if seen[key] {
					item.Error = strPtr("同一证件号在文件中重复出现")
					resp.Errors++
				} else {
					seen[key] = true
					item.MatchStatus = "new"
					pending = append(pending, pendingCreate{index: len(resp.Rows), row: row})
				}
			} else {
				item.Error = strPtr("证件号无法识别且未匹配到已有选手（特殊证件请先在人工报名建档）")
				resp.Errors++
			}
		} else if pid := matchByName(players, item.PlayerName); pid > 0 {
			key := fmt.Sprintf("p:%d", pid)
			if seen[key] {
				item.Error = strPtr("同一选手在文件中重复出现")
				resp.Errors++
			} else {
				seen[key] = true
				item.PlayerID = &pid
				item.MatchStatus = "matched"
			}
		} else {
			item.Error = strPtr("缺少证件号且姓名未唯一匹配")
			resp.Errors++
		}
		resp.Rows = append(resp.Rows, item)
	}

	// 全部行可导入时才建档，避免半成品文件留下无成绩的孤儿档案。
	if resp.Errors == 0 && len(pending) > 0 {
		err = s.stores.WithTx(ctx, func(tx *sql.Tx) error {
			for _, p := range pending {
				player, created, err := s.players.MatchOrCreate(ctx, tx,
					IdentityInput{Name: p.row.PlayerName, IDNumber: p.row.IDCardNo})
				if err != nil {
					return err
				}
				resp.Rows[p.index].PlayerID = &player.ID
				if created {
					resp.Rows[p.index].MatchStatus = "created"
				} else {
					resp.Rows[p.index].MatchStatus = "matched"
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	resp.Matched, resp.Created, resp.Unmatched = 0, 0, resp.Errors
	for _, row := range resp.Rows {
		switch row.MatchStatus {
		case "matched":
			resp.Matched++
		case "created":
			resp.Created++
		}
	}

	targetType := "board"
	if err := s.stores.WithTx(ctx, func(tx *sql.Tx) error {
		return adminLog(ctx, s.stores, tx, &adminID, adminActionScoreImport,
			&targetType, &board.ID, map[string]any{
				"event_id": eventID, "level": level, "dance_type": dance,
				"rows": len(rows), "created": resp.Created, "errors": resp.Errors,
				"import_batch": batch,
			})
	}); err != nil {
		return nil, httpx.ErrInternal(err, "记录操作日志失败，请稍后重试")
	}
	return resp, nil
}

// scoreImportRowError 校验导入行必填项，返回用户可见错误（不含证件号明文）。
func scoreImportRowError(name, award string) string {
	switch {
	case name == "":
		return "选手姓名不能为空"
	case !model.IsValidAward(award):
		return "奖项不合法"
	default:
		return ""
	}
}

// newScoreImportBatch 生成导入批次号（时间 + 随机后缀），用于追溯成绩来源文件。
func newScoreImportBatch(now time.Time) (string, error) {
	buf := make([]byte, 3)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random: %w", err)
	}
	return fmt.Sprintf("score-%s-%s", now.Format("20060102150405"), hex.EncodeToString(buf)), nil
}
