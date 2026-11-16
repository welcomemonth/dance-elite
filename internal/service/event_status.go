package service

import (
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// 赛事存储状态的自动流转次序（只前进、不回退）。
const (
	eventRankRegistering = 1
	eventRankRegClosed   = 2
	eventRankOngoing     = 3
	eventRankFinished    = 4
)

// dueEventStatus 返回赛事在 now 时刻应当处于的存储状态（T34）。
//
// 以存储状态为下界：手动关闭报名（reg_closed）、提前进入 ongoing 等动作不会被
// 时间线推导回退；draft / unpublished / offline 不参与自动流转，原样返回。
// 时间线规则：now ≥ reg_end_at → reg_closed；now ≥ start_at → ongoing；
// now ≥ max(event_date, start_at) 次日 00:00（+08:00）→ finished。
func dueEventStatus(e *model.Event, now time.Time) string {
	switch e.Status {
	case model.EventStatusDraft, model.EventStatusUnpublished, model.EventStatusOffline:
		return e.Status
	}

	storedRank := eventStatusRank(e.Status)
	target, targetRank := e.Status, storedRank

	if finishedAt, ok := eventFinishAt(e); ok && !now.Before(finishedAt) {
		target, targetRank = model.EventStatusFinished, eventRankFinished
	} else if startAt, ok := parseISOPtr(e.StartAt); ok && !now.Before(startAt) {
		target, targetRank = model.EventStatusOngoing, eventRankOngoing
	} else if regEnd, ok := parseISOPtr(e.RegEndAt); ok && !now.Before(regEnd) {
		target, targetRank = model.EventStatusRegClosed, eventRankRegClosed
	} else if regStart, ok := parseISOPtr(e.RegStartAt); ok && !now.Before(regStart) {
		target, targetRank = model.EventStatusRegistering, eventRankRegistering
	}

	if targetRank > storedRank {
		return target
	}
	return e.Status
}

// deriveMiniappStatus 推导小程序端展示状态（4 态，详见 MiniappEventItem 注释）。
// 存储状态作为下界参与推导：后台手动关闭报名后，即使未到 reg_end_at 也不会
// 再显示为报名中。
func deriveMiniappStatus(e *model.Event, now time.Time) string {
	switch dueEventStatus(e, now) {
	case model.EventStatusFinished:
		return model.MiniappStatusFinished
	case model.EventStatusRegClosed, model.EventStatusOngoing:
		// ongoing 对外映射为「报名已截止」，小程序契约保持 4 态。
		return model.MiniappStatusRegClosed
	case model.EventStatusRegistering:
		if regStart, ok := parseISOPtr(e.RegStartAt); ok && now.Before(regStart) {
			return model.MiniappStatusUpcoming
		}
		return model.MiniappStatusRegistering
	default:
		return model.MiniappStatusUpcoming
	}
}

// eventStatusRank 返回存储状态的流转位次；未知/未发布状态按 0 处理。
func eventStatusRank(status string) int {
	switch status {
	case model.EventStatusRegistering:
		return eventRankRegistering
	case model.EventStatusRegClosed:
		return eventRankRegClosed
	case model.EventStatusOngoing:
		return eventRankOngoing
	case model.EventStatusFinished:
		return eventRankFinished
	default:
		return 0
	}
}

// eventFinishAt 返回赛事结束时刻：比赛日期与开始比赛时间中较晚一天的次日 00:00。
func eventFinishAt(e *model.Event) (time.Time, bool) {
	var day time.Time
	hasDay := false
	if startAt, ok := parseISOPtr(e.StartAt); ok {
		day, hasDay = chinaDay(startAt), true
	}
	if e.EventDate != nil {
		if d, err := utils.ParseDate(*e.EventDate); err == nil {
			if !hasDay || chinaDay(d).After(day) {
				day = chinaDay(d)
			}
			hasDay = true
		}
	}
	if !hasDay {
		return time.Time{}, false
	}
	return day.AddDate(0, 0, 1), true
}

// parseISOPtr 解析可空的 ISO8601 时间字段。
func parseISOPtr(p *string) (time.Time, bool) {
	if p == nil || *p == "" {
		return time.Time{}, false
	}
	t, err := utils.ParseISO(*p)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// chinaDay 取时间在中国时区的当天 00:00。
func chinaDay(t time.Time) time.Time {
	y, m, d := t.In(utils.ChinaLoc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, utils.ChinaLoc)
}
