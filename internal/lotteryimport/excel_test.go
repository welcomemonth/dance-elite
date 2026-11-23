package lotteryimport

import (
	"strings"
	"testing"
)

func TestMapColumnsByHeaderName(t *testing.T) {
	header := []string{
		"序号", " 身份证号 ", "组别及场次", "姓名",
		"剧目名称", "选送机构", " 指导老师（仅限一人） ", "手机",
	}
	cols, err := mapColumns(header)
	if err != nil {
		t.Fatalf("mapColumns: %v", err)
	}
	if cols.Name != 3 || cols.IDCard != 1 || cols.Group != 2 {
		t.Fatalf("required columns = %+v", cols)
	}
	if cols.Program != 4 || cols.Institution != 5 || cols.Genre != -1 {
		t.Fatalf("optional columns = %+v", cols)
	}
	if cols.Phone != 7 {
		t.Fatalf("phone column = %d, want 7 (表头「手机」)", cols.Phone)
	}
}

func TestMapColumnsMissingRequired(t *testing.T) {
	_, err := mapColumns([]string{"序号", "身份证号", "组别及场次"})
	if err == nil || !strings.Contains(err.Error(), "姓名") {
		t.Fatalf("err = %v, want missing name error", err)
	}
}
