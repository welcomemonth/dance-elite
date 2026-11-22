package lotteryimport

import "testing"

func TestParseAgeGroupAndGenre(t *testing.T) {
	cases := []struct {
		text     string
		wantAge  string
		wantKind string
	}{
		{"U11组（2016-2018年出生） - 12月5日：古典舞 980元", "U11", "classical"},
		{"U15组（2012-2013年出生） - 12月6日:民族民间舞 980元", "U15", "folk"},
		{"U13组 - 12月6日：民族舞 980元", "U13", "folk"},
	}
	for _, tc := range cases {
		age, ok := parseAgeGroup(tc.text)
		if !ok || age != tc.wantAge {
			t.Errorf("parseAgeGroup(%q) = %q,%v want %q", tc.text, age, ok, tc.wantAge)
		}
		genre, ok := parseGenre(tc.text)
		if !ok || genre != tc.wantKind {
			t.Errorf("parseGenre(%q) = %q,%v want %q", tc.text, genre, ok, tc.wantKind)
		}
	}
}

func TestParsePhone(t *testing.T) {
	cases := []struct {
		raw     string
		want    string
		warning bool
	}{
		{raw: "13800000001", want: "13800000001"},
		{raw: " 138-0000-0001 ", want: "13800000001"},
		{raw: "+8613800000001", want: "13800000001"},
		{raw: "8613800000001", want: "13800000001"},
		{raw: "", want: ""},
		{raw: "12345", want: "", warning: true},
		{raw: "23800000001", want: "", warning: true},
	}
	for _, tc := range cases {
		got, warning := parsePhone(tc.raw)
		if got != tc.want {
			t.Fatalf("parsePhone(%q) = %q, want %q", tc.raw, got, tc.want)
		}
		if (warning != "") != tc.warning {
			t.Fatalf("parsePhone(%q) warning = %q, want warning=%v", tc.raw, warning, tc.warning)
		}
	}
}

func TestParseUnknownValues(t *testing.T) {
	if _, ok := parseAgeGroup("U17组"); ok {
		t.Error("U17 should not match")
	}
	if _, ok := parseGenre("芭蕾"); ok {
		t.Error("芭蕾 should not match")
	}
}
