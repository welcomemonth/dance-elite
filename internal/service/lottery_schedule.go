package service

import (
	"crypto/rand"
	"fmt"
	"math/big"
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

// lotteryWindowOf 解析抽签时间窗口；时间非法时按未配置处理，避免一个字段
// 写错让整场赛事不可用。
func lotteryWindowOf(session *model.LotterySession) lotteryWindow {
	var w lotteryWindow
	if session.StartAt != nil {
		if t, err := utils.ParseISO(*session.StartAt); err == nil {
			w.start, w.hasStart = t, true
		} else if t, err := utils.ParseFlexibleTime(*session.StartAt); err == nil {
			w.start, w.hasStart = t, true
		}
	}
	if session.EndAt != nil {
		if t, err := utils.ParseISO(*session.EndAt); err == nil {
			w.end, w.hasEnd = t, true
		} else if t, err := utils.ParseFlexibleTime(*session.EndAt); err == nil {
			w.end, w.hasEnd = t, true
		}
	}
	return w
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
func remindMinutes(session *model.LotterySession) int {
	if session.RemindBeforeMinutes > 0 {
		return session.RemindBeforeMinutes
	}
	return lotteryDefaultRemindMinutes
}

// lotteryTimeText 把时间格式化为短信与页面可读的本地时间。
func lotteryTimeText(t time.Time) string {
	return t.In(utils.ChinaLoc).Format("2006年01月02日 15:04")
}

// pendingLotteryPlayers 返回尚未抽签的选手人数。
func pendingLotteryPlayers(players []model.LotteryPlayer) int {
	pending := 0
	for _, player := range players {
		if player.SlotNo == nil {
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

// availableLotterySlots 返回同「组别 × 舞种」下尚未被占用的全部槽位。
func availableLotterySlots(groups []model.LotteryGroup, players []model.LotteryPlayer,
	genre, ageGroup string) []model.LotterySlot {
	taken := make(map[model.LotterySlot]bool, len(players))
	for _, player := range players {
		if player.Genre == genre && player.AgeGroup == ageGroup && player.SlotNo != nil {
			taken[model.LotterySlot{Letter: *player.GroupLetter, No: *player.SlotNo}] = true
		}
	}
	slots := make([]model.LotterySlot, 0)
	for _, group := range groups {
		if group.Genre != genre || group.AgeGroup != ageGroup {
			continue
		}
		for no := 1; no <= group.Size; no++ {
			slot := model.LotterySlot{Letter: group.Letter, No: no}
			if !taken[slot] {
				slots = append(slots, slot)
			}
		}
	}
	return slots
}

// pickLotterySlot 用 crypto/rand 从候选槽位中均匀随机取一个。
func pickLotterySlot(slots []model.LotterySlot) (model.LotterySlot, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(slots))))
	if err != nil {
		return model.LotterySlot{}, fmt.Errorf("pick random slot: %w", err)
	}
	return slots[n.Int64()], nil
}
