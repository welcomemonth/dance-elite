package service

import (
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
)

func TestSeasonStatusForDates(t *testing.T) {
	const today = "2026-11-16"
	cases := []struct {
		name       string
		start, end string
		want       string
	}{
		{"区间覆盖今天", "2026-01-01", "2026-12-31", model.SeasonStatusActive},
		{"今天等于开始日期", "2026-11-16", "2026-12-31", model.SeasonStatusActive},
		{"今天等于结束日期", "2026-01-01", "2026-11-16", model.SeasonStatusActive},
		{"尚未开始", "2026-11-17", "2026-12-31", model.SeasonStatusUpcoming},
		{"已经结束", "2025-01-01", "2026-11-15", model.SeasonStatusUpcoming},
		{"日期均留空视为不限", "", "", model.SeasonStatusActive},
		{"只有开始日期且已到", "2026-01-01", "", model.SeasonStatusActive},
		{"只有开始日期且未到", "2026-11-17", "", model.SeasonStatusUpcoming},
		{"只有结束日期且未过", "", "2026-12-31", model.SeasonStatusActive},
		{"只有结束日期且已过", "", "2026-11-15", model.SeasonStatusUpcoming},
	}
	for _, tc := range cases {
		if got := seasonStatusForDates(tc.start, tc.end, today); got != tc.want {
			t.Errorf("%s: seasonStatusForDates(%q, %q) = %q, want %q", tc.name, tc.start, tc.end, got, tc.want)
		}
	}
}

func TestSeasonRangesOverlap(t *testing.T) {
	cases := []struct {
		name                       string
		aStart, aEnd, bStart, bEnd string
		want                       bool
	}{
		{"完全相同", "2027-01-01", "2027-12-31", "2027-01-01", "2027-12-31", true},
		{"部分相交", "2027-01-01", "2027-06-30", "2027-06-01", "2027-12-31", true},
		{"包含", "2027-01-01", "2027-12-31", "2027-03-01", "2027-04-01", true},
		{"端点相接", "2027-01-01", "2027-06-30", "2027-06-30", "2027-12-31", true},
		{"完全在之前", "2027-01-01", "2027-12-31", "2028-01-01", "2028-12-31", false},
		{"完全在之后", "2028-01-01", "2028-12-31", "2027-01-01", "2027-12-31", false},
		{"甲方不限", "", "", "2030-01-01", "2030-12-31", true},
		{"甲方只限开始", "2027-01-01", "", "2026-01-01", "2026-12-31", false},
		{"甲方只限结束", "", "2027-01-01", "2027-02-01", "2027-12-31", false},
	}
	for _, tc := range cases {
		if got := seasonRangesOverlap(tc.aStart, tc.aEnd, tc.bStart, tc.bEnd); got != tc.want {
			t.Errorf("%s: seasonRangesOverlap = %v, want %v", tc.name, got, tc.want)
		}
	}
}
