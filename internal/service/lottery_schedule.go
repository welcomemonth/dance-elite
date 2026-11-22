package service

import (
	"strings"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// lotteryDefaultRemindMinutes 是未配置 remind_before_minutes 时的提醒提前量。
const lotteryDefaultRemindMinutes = 120

// lotteryWindow 是抽签时间窗口；未配置的一端不做限制。
type lotteryWindow struct {
	start, end       time.Time
	hasStart, hasEnd bool
}

// lotteryWindowOf 解析赛事的抽签时间窗口；手工写入的时间非法时按未配置处理，
// 页面退回「不限制」的旧行为，不因一个字段写错让整场赛事不可用。
func lotteryWindowOf(data *model.LotteryData) lotteryWindow {
	var w lotteryWindow
	if t, err := parseLotteryTime(data.StartAt); err == nil {
		w.start, w.hasStart = t, true
	}
	if t, err := parseLotteryTime(data.EndAt); err == nil {
		w.end, w.hasEnd = t, true
	}
	return w
}

// parseLotteryTime 兼容手工编辑 JSON 时常见的几种写法；
// 不带时区的时间按部署时区（Asia/Shanghai）解释。
func parseLotteryTime(s string) (time.Time, error) {
	return utils.ParseFlexibleTime(s)
}

// status 返回页面状态；未开始优先于已截止（时间窗口写反时以开始为准）。
func (w lotteryWindow) status(now time.Time) string {
	if w.hasStart && now.Before(w.start) {
		return model.LotteryStatusNotStarted
	}
	if w.hasEnd && !now.Before(w.end) {
		return model.LotteryStatusEnded
	}
	return model.LotteryStatusOngoing
}

func (w lotteryWindow) started(now time.Time) bool {
	return !w.hasStart || !now.Before(w.start)
}

func (w lotteryWindow) ended(now time.Time) bool {
	return w.hasEnd && !now.Before(w.end)
}

// inRemindWindow 判断当前是否处于「截止前 remindMinutes 分钟」的提醒窗口。
func (w lotteryWindow) inRemindWindow(now time.Time, remindMinutes int) bool {
	if !w.hasEnd || w.ended(now) || !w.started(now) {
		return false
	}
	return !now.Before(w.end.Add(-remindDuration(remindMinutes)))
}

// remindDuration 把配置的提前量转成时长；非法值回落到默认 120 分钟。
func remindDuration(minutes int) time.Duration {
	if minutes <= 0 {
		minutes = lotteryDefaultRemindMinutes
	}
	return time.Duration(minutes) * time.Minute
}

// remindMinutes 读取赛事配置的提醒提前量（分钟）。
func remindMinutes(data *model.LotteryData) int {
	if data.RemindBeforeMinutes > 0 {
		return data.RemindBeforeMinutes
	}
	return lotteryDefaultRemindMinutes
}

// lotteryTimeText 把时间格式化为短信与页面可读的本地时间。
func lotteryTimeText(t time.Time) string {
	return t.In(utils.ChinaLoc).Format("2006年01月02日 15:04")
}

// autoDrawPending 给所有尚未抽签的选手分配剩余槽位（与人工抽签同一套规则）。
// 返回本次分配人数与因槽位不足而跳过的人数；正常数据下跳过恒为 0。
func autoDrawPending(data *model.LotteryData, now time.Time) (drawn, skipped int) {
	for i := range data.Players {
		player := &data.Players[i]
		if player.Slot != nil {
			continue
		}
		slots := availableLotterySlots(data, player.Genre, player.AgeGroup)
		if len(slots) == 0 {
			skipped++
			continue
		}
		slot, err := pickLotterySlot(slots)
		if err != nil {
			skipped++
			continue
		}
		player.Slot = &slot
		player.DrawnAt = utils.FormatISO(now)
		player.AutoDrawn = true
		drawn++
	}
	return drawn, skipped
}

// pendingLotteryPlayers 返回尚未抽签的选手人数。
func pendingLotteryPlayers(data *model.LotteryData) int {
	pending := 0
	for _, player := range data.Players {
		if player.Slot == nil {
			pending++
		}
	}
	return pending
}

// lotteryLink 拼接某个赛事的抽签页面链接（短信正文使用）。
func lotteryLink(baseURL, token string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return ""
	}
	return baseURL + "/lottery/" + token
}

// lotteryGenreLabel 返回舞种的展示名；找不到时退回枚举值。
func lotteryGenreLabel(data *model.LotteryData, genre string) string {
	for _, item := range data.Genres {
		if item.Key == genre {
			return item.Label
		}
	}
	return genre
}
