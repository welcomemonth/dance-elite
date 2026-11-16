package service

import "github.com/meetmonth/dance-elite/internal/model"

// seasonStatusForDates 按当天日期推导新建赛季的初始状态：
// 今天落在 [start, end] 内（空串视为不限）→ active，否则 upcoming。
// 日期均为 YYYY-MM-DD，可直接按字符串比较。
func seasonStatusForDates(start, end, today string) string {
	if (start == "" || start <= today) && (end == "" || today <= end) {
		return model.SeasonStatusActive
	}
	return model.SeasonStatusUpcoming
}

// seasonRangesOverlap 判断两个日期区间 [aStart, aEnd] 与 [bStart, bEnd] 是否重叠；
// 空串视为不限（−∞ / +∞），端点相同也算重叠（两个赛季不能共用同一天）。
func seasonRangesOverlap(aStart, aEnd, bStart, bEnd string) bool {
	if aEnd != "" && bStart != "" && aEnd < bStart {
		return false
	}
	if bEnd != "" && aStart != "" && bEnd < aStart {
		return false
	}
	return true
}
