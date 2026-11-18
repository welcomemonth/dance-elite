package lotteryimport

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/xuri/excelize/v2"
)

// columnMap 记录必需/可选列在表头中的下标（-1 表示不存在）。
// 列按表头名称定位，因此报名表调整列顺序不影响导入。
type columnMap struct {
	Name        int
	IDCard      int
	Group       int
	Genre       int
	Program     int
	Institution int
}

// headerAliases 是各字段可接受的中文表头写法（按优先级）。
var headerAliases = map[string][]string{
	"name":        {"姓名", "选手姓名", "学生姓名", "学员姓名", "参赛选手"},
	"id_card":     {"身份证号", "身份证号码", "身份证", "证件号", "证件号码"},
	"group":       {"组别及场次", "组别", "参赛组别", "年龄组", "组别场次"},
	"genre":       {"舞种", "舞蹈种类", "参赛舞种"},
	"program":     {"剧目名称", "节目名称", "剧目", "节目"},
	"institution": {"选送机构", "机构", "选送单位", "单位"},
}

// readRows 读取第一个工作表的全部行。返回的切片下标 0 是表头，
// 即 Excel 第 1 行；后续行号按 index+1 计算。
func readRows(path string) ([][]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("打开 Excel 失败: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("Excel 没有工作表")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("读取工作表失败: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("Excel 为空")
	}
	return rows, nil
}

// mapColumns 按表头名称定位列；缺少必需列时返回可读的错误。
func mapColumns(header []string) (columnMap, error) {
	cols := columnMap{Name: -1, IDCard: -1, Group: -1, Genre: -1, Program: -1, Institution: -1}
	find := func(field string) int {
		for _, alias := range headerAliases[field] {
			want := normalizeHeader(alias)
			for i, h := range header {
				if normalizeHeader(h) == want {
					return i
				}
			}
		}
		return -1
	}

	cols.Name = find("name")
	cols.IDCard = find("id_card")
	cols.Group = find("group")
	cols.Genre = find("genre")
	cols.Program = find("program")
	cols.Institution = find("institution")

	var missing []string
	if cols.Name < 0 {
		missing = append(missing, "姓名（可用表头：姓名/选手姓名/学生姓名）")
	}
	if cols.IDCard < 0 {
		missing = append(missing, "身份证号（可用表头：身份证号/身份证号码/证件号）")
	}
	if cols.Group < 0 {
		missing = append(missing, "组别（可用表头：组别及场次/组别/参赛组别/年龄组）")
	}
	if len(missing) > 0 {
		return cols, fmt.Errorf("Excel 缺少必需列：%s", strings.Join(missing, "；"))
	}
	return cols, nil
}

// normalizeHeader 去掉表头中的 BOM、全部空白与首尾空格，
// 兼容 " 指导老师（仅限一人） " 这类带空格的表头。
func normalizeHeader(s string) string {
	s = strings.TrimPrefix(s, "\ufeff")
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

// cell 安全读取一行中的某一列。
func cell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

// isBlankRow 判断一行在业务列上是否全为空（用于跳过 Excel 末尾空行）。
func isBlankRow(row []string, cols columnMap) bool {
	return cell(row, cols.Name) == "" && cell(row, cols.IDCard) == "" && cell(row, cols.Group) == ""
}
