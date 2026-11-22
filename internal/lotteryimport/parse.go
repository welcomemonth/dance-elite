package lotteryimport

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/meetmonth/dance-elite/internal/pkg/sms"
	"github.com/meetmonth/dance-elite/internal/utils"
)

var (
	// lotteryIDPattern 校验 18 位身份证号（末位可为 X）。
	lotteryIDPattern = regexp.MustCompile(`^\d{17}[\dX]$`)
	// ageGroupPattern 从「组别及场次」中提取 U11/U13/U15。
	ageGroupPattern = regexp.MustCompile(`U1[135]`)
)

// row 是一条通过校验的报名记录（导入中间结构）。
type row struct {
	excelRow    int
	name        string
	idCard      string
	genre       string
	ageGroup    string
	institution string
	program     string
	phone       string
	// phoneWarning 记录手机号不合法等不阻断导入的问题。
	phoneWarning string
}

// parseRow 解析并校验一行报名数据；错误信息带具体原因，由调用方加行号汇总。
func parseRow(values []string, cols columnMap, excelRow int) (row, error) {
	name := strings.TrimSpace(cell(values, cols.Name))
	rawIDCard := cell(values, cols.IDCard)
	groupText := cell(values, cols.Group)
	genreText := groupText
	if cols.Genre >= 0 {
		genreText = cell(values, cols.Genre)
	}

	if name == "" {
		return row{}, fmt.Errorf("姓名为空")
	}
	idCard := utils.NormalizeIDNumber(utils.IDTypeIDCard, rawIDCard)
	if idCard == "" {
		return row{}, fmt.Errorf("身份证号为空")
	}
	if !lotteryIDPattern.MatchString(idCard) {
		return row{}, fmt.Errorf("身份证号应为 18 位（末位可为 X），实际为 %q", rawIDCard)
	}
	ageGroup, ok := parseAgeGroup(groupText)
	if !ok {
		return row{}, fmt.Errorf("组别无法识别出生年份组（应包含 U11/U13/U15），实际为 %q", groupText)
	}
	genre, ok := parseGenre(genreText)
	if !ok {
		return row{}, fmt.Errorf("舞种无法识别（应为古典舞或民族民间舞），实际为 %q", genreText)
	}

	phone, phoneWarning := parsePhone(cell(values, cols.Phone))
	return row{
		excelRow:     excelRow,
		name:         name,
		idCard:       idCard,
		genre:        genre,
		ageGroup:     ageGroup,
		institution:  cell(values, cols.Institution),
		program:      cell(values, cols.Program),
		phone:        phone,
		phoneWarning: phoneWarning,
	}, nil
}

// parsePhone 规范化手机号（去空白/连字符、去 +86 前缀）并校验；
// 不合法时返回空号与提示，不阻断导入（该选手只是收不到短信）。
func parsePhone(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	phone := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsSpace(r), r == '-', r == '(', r == ')':
			return -1
		}
		return r
	}, raw)
	phone = strings.TrimPrefix(phone, "+86")
	if len(phone) == 13 && strings.HasPrefix(phone, "86") {
		phone = phone[2:]
	}
	if !sms.ValidPhone(phone) {
		return "", fmt.Sprintf("手机号 %q 格式不合法，该选手不会收到短信", raw)
	}
	return phone, ""
}

// parseAgeGroup 从组别文本中提取年龄组标识。
func parseAgeGroup(text string) (string, bool) {
	match := ageGroupPattern.FindString(text)
	return match, match != ""
}

// parseGenre 从舞种文本中识别系统枚举值。
// 先判断「民族民间舞」再判断「古典舞」，避免关键词包含关系误判。
func parseGenre(text string) (string, bool) {
	compact := strings.Join(strings.Fields(text), "")
	switch {
	case strings.Contains(compact, "民族民间舞"), strings.Contains(compact, "民族舞"):
		return "folk", true
	case strings.Contains(compact, "古典舞"):
		return "classical", true
	}
	return "", false
}
