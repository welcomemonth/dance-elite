package lotteryimport

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// testLotteryID 生成合法的 18 位身份证号（仅数字），用于构造测试数据。
func testLotteryID(n int) string {
	return fmt.Sprintf("31010120150101%04d", n)
}

// writeLotteryTestExcel 把表头与数据行写成一个临时 xlsx，返回文件路径。
func writeLotteryTestExcel(t *testing.T, headers []string, rows [][]string) string {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(0)

	headerRow := make([]interface{}, len(headers))
	for i, h := range headers {
		headerRow[i] = h
	}
	if err := f.SetSheetRow(sheet, "A1", &headerRow); err != nil {
		t.Fatalf("write header: %v", err)
	}
	for i, row := range rows {
		values := make([]interface{}, len(row))
		for j, v := range row {
			values[j] = v
		}
		cell := fmt.Sprintf("A%d", i+2)
		if err := f.SetSheetRow(sheet, cell, &values); err != nil {
			t.Fatalf("write row %d: %v", i+2, err)
		}
	}
	path := filepath.Join(t.TempDir(), "报名资料.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("save xlsx: %v", err)
	}
	return path
}

func TestBuildWithReorderedColumns(t *testing.T) {
	headers := []string{"序号", "组别及场次", "姓名", "身份证号", "剧目名称", "选送机构", "手机"}
	var rows [][]string
	for i := 0; i < 12; i++ {
		rows = append(rows, []string{
			fmt.Sprint(i + 1),
			"U11组（2016-2018年出生） - 12月5日：古典舞 980元",
			fmt.Sprintf("古典%02d", i),
			testLotteryID(i),
			"《剧目》",
			"测试机构",
			"13800000000",
		})
	}
	for i := 0; i < 8; i++ {
		rows = append(rows, []string{
			fmt.Sprint(i + 1),
			"U11组（2016-2018年出生） - 12月6日:民族民间舞 980元",
			fmt.Sprintf("民族%02d", i),
			testLotteryID(100 + i),
			"《剧目》",
			"测试机构",
			"",
		})
	}

	data, warnings, err := Build(writeLotteryTestExcel(t, headers, rows), "测试赛事")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(data.Players) != 20 {
		t.Fatalf("players = %d, want 20", len(data.Players))
	}
	if len(data.Genres) != 2 || data.Genres[0].Key != "classical" || data.Genres[1].Key != "folk" {
		t.Fatalf("genres = %+v", data.Genres)
	}
	if len(data.Groups) != 2 {
		t.Fatalf("groups = %+v", data.Groups)
	}
	wantGroups := []struct {
		genre, age, letter string
		size               int
	}{
		{"classical", "U11", "A", 12},
		{"folk", "U11", "A", 8},
	}
	for i, want := range wantGroups {
		got := data.Groups[i]
		if got.Genre != want.genre || got.AgeGroup != want.age || got.Letter != want.letter || got.Size != want.size {
			t.Fatalf("group[%d] = %+v, want %+v", i, got, want)
		}
	}
	if data.Players[0].Name != "古典00" || data.Players[0].Program != "《剧目》" ||
		data.Players[0].Institution != "测试机构" {
		t.Fatalf("player[0] = %+v", data.Players[0])
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestBuildWithSeparateGenreColumn(t *testing.T) {
	headers := []string{"姓名", "身份证号", "组别", "舞种"}
	rows := [][]string{{"张三", testLotteryID(1), "U13组（2014-2015年出生）", "民族民间舞"}}
	data, _, err := Build(writeLotteryTestExcel(t, headers, rows), "")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if data.Players[0].Genre != "folk" || data.Players[0].AgeGroup != "U13" {
		t.Fatalf("player = %+v", data.Players[0])
	}
}

func TestBuildReportsDuplicateAndBadID(t *testing.T) {
	headers := []string{"姓名", "身份证号", "组别及场次"}
	rows := [][]string{
		{"张三", testLotteryID(1), "U11组 古典舞"},
		{"李四", testLotteryID(1), "U11组 古典舞"},
		{"王五", "31010120150101123", "U11组 古典舞"},
	}
	_, _, err := Build(writeLotteryTestExcel(t, headers, rows), "")
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	problems := strings.Join(validation.Problems, "\n")
	if !strings.Contains(problems, "重复出现") {
		t.Errorf("missing duplicate problem: %s", problems)
	}
	if !strings.Contains(problems, "18 位") {
		t.Errorf("missing invalid id problem: %s", problems)
	}
}

func TestBuildRejectsUnknownGenre(t *testing.T) {
	headers := []string{"姓名", "身份证号", "组别及场次"}
	rows := [][]string{{"张三", testLotteryID(1), "U11组 芭蕾"}}
	_, _, err := Build(writeLotteryTestExcel(t, headers, rows), "")
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	if !strings.Contains(strings.Join(validation.Problems, "\n"), "舞种无法识别") {
		t.Fatalf("problems = %v", validation.Problems)
	}
}
