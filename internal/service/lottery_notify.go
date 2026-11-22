package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/logger"
	"github.com/meetmonth/dance-elite/internal/utils"
)

const (
	// lotterySMSKindRemind / lotterySMSKindResult 区分两类短信的发送记录字段。
	lotterySMSKindRemind = "remind"
	lotterySMSKindResult = "result"
	// lotterySMSMaxAttempts 是单个选手单类短信的最大尝试次数，避免模板错误时反复调用。
	lotterySMSMaxAttempts = 3
	// lotterySMSParallel 是并发发送数；同手机号连续发送可能触发频控，失败下次 tick 重试。
	lotterySMSParallel = 4
	// lotterySMSBatch 是每批发送后落盘一次的条数，保证超时被取消时已发结果不丢失。
	lotterySMSBatch = 20
)

// lotterySMSRecipient 指向某位选手（按证件号 + 舞种定位）。
type lotterySMSRecipient struct {
	idCard string
	genre  string
}

// lotterySMSJob 是一条待发送短信；recipients 为这条短信覆盖的选手
// （提醒短信按手机号合并后可能覆盖多人，结果短信恒为一人）。
type lotterySMSJob struct {
	phone      string
	template   string
	params     map[string]string
	recipients []lotterySMSRecipient
}

// remindJobs 生成截止前提醒短信：未抽签、有手机号、未发送且未达尝试上限的选手，
// 按手机号合并成一条，姓名为「张三、李四」。
func (s *LotterySweeper) remindJobs(data *model.LotteryData, token string) []lotterySMSJob {
	template := data.SMSRemindTemplate
	window := lotteryWindowOf(data)
	if template == "" || !window.hasEnd || lotteryLink(s.baseURL, token) == "" {
		return nil
	}

	type group struct {
		names      []string
		recipients []lotterySMSRecipient
	}
	groups := make(map[string]*group)
	var order []string
	for _, player := range data.Players {
		if player.Slot != nil || player.Phone == "" {
			continue
		}
		if player.SMS != nil && (player.SMS.RemindSentAt != "" ||
			player.SMS.RemindAttempts >= lotterySMSMaxAttempts) {
			continue
		}
		g, ok := groups[player.Phone]
		if !ok {
			g = &group{}
			groups[player.Phone] = g
			order = append(order, player.Phone)
		}
		g.names = append(g.names, player.Name)
		g.recipients = append(g.recipients, lotterySMSRecipient{player.IDCard, player.Genre})
	}

	deadline := lotteryTimeText(window.end)
	jobs := make([]lotterySMSJob, 0, len(order))
	for _, phone := range order {
		g := groups[phone]
		jobs = append(jobs, lotterySMSJob{
			phone:    phone,
			template: template,
			params: map[string]string{
				"name":     strings.Join(g.names, "、"),
				"event":    data.EventName,
				"deadline": deadline,
			},
			recipients: g.recipients,
		})
	}
	return jobs
}

// resultJobs 生成截止后自动分配的结果短信：只发给由系统自动分配的选手。
func (s *LotterySweeper) resultJobs(data *model.LotteryData, token string) []lotterySMSJob {
	template := data.SMSResultTemplate
	if template == "" || lotteryLink(s.baseURL, token) == "" {
		return nil
	}
	var jobs []lotterySMSJob
	for _, player := range data.Players {
		if !player.AutoDrawn || player.Slot == nil || player.Phone == "" {
			continue
		}
		if player.SMS != nil && (player.SMS.ResultSentAt != "" ||
			player.SMS.ResultAttempts >= lotterySMSMaxAttempts) {
			continue
		}
		jobs = append(jobs, lotterySMSJob{
			phone:    player.Phone,
			template: template,
			params: map[string]string{
				"name":  player.Name,
				"event": data.EventName,
				"genre": lotteryGenreLabel(data, player.Genre),
				"slot":  lotterySlotText(*player.Slot),
			},
			recipients: []lotterySMSRecipient{{player.IDCard, player.Genre}},
		})
	}
	return jobs
}

// pendingRemindPlayers 统计还会收到提醒短信的选手数
// （未抽签、有手机号、未发送且未达尝试上限）。
func pendingRemindPlayers(data *model.LotteryData) int {
	pending := 0
	for _, player := range data.Players {
		if player.Slot != nil || player.Phone == "" {
			continue
		}
		if player.SMS != nil && (player.SMS.RemindSentAt != "" ||
			player.SMS.RemindAttempts >= lotterySMSMaxAttempts) {
			continue
		}
		pending++
	}
	return pending
}

// pendingResultPlayers 统计还会收到结果短信的选手数
// （由系统自动分配、未发送且未达尝试上限）。
func pendingResultPlayers(data *model.LotteryData) int {
	pending := 0
	for _, player := range data.Players {
		if !player.AutoDrawn || player.Slot == nil || player.Phone == "" {
			continue
		}
		if player.SMS != nil && (player.SMS.ResultSentAt != "" ||
			player.SMS.ResultAttempts >= lotterySMSMaxAttempts) {
			continue
		}
		pending++
	}
	return pending
}

// missingLotterySMSConfig 返回当前阶段会导致短信整体跳过的缺失配置。
// 截止前只关心提醒模板、截止后只关心结果模板（都需要 PUBLIC_BASE_URL 拼链接）；
// 没有截止时间的赛事按设计不发短信，返回 nil。
func missingLotterySMSConfig(data *model.LotteryData, baseURL string, now time.Time) []string {
	if data.EndAt == "" {
		return nil
	}
	window := lotteryWindowOf(data)
	if !window.hasEnd {
		return nil
	}
	var missing []string
	if !window.ended(now) {
		if pendingRemindPlayers(data) == 0 {
			return nil
		}
		if data.SMSRemindTemplate == "" {
			missing = append(missing, "sms_remind_template")
		}
	} else {
		if pendingResultPlayers(data) == 0 {
			return nil
		}
		if data.SMSResultTemplate == "" {
			missing = append(missing, "sms_result_template")
		}
	}
	// 模板齐全但链接前缀缺失时同样发不出去，单独告警。
	if strings.TrimSpace(baseURL) == "" {
		missing = append(missing, "PUBLIC_BASE_URL")
	}
	return missing
}

// sendJobs 分批发送并在每批后落盘发送结果。
//
// 落盘用独立 context：即使本轮定时任务因超时被取消，已成功的记录也必须写回，
// 否则下一轮会重复发送。
func (s *LotterySweeper) sendJobs(ctx context.Context, token, kind string, jobs []lotterySMSJob) error {
	if s.sms == nil || len(jobs) == 0 {
		return nil
	}
	for start := 0; start < len(jobs); start += lotterySMSBatch {
		end := min(start+lotterySMSBatch, len(jobs))
		results := s.deliver(ctx, jobs[start:end])

		recordCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := s.recordSMS(recordCtx, token, kind, results)
		cancel()
		if err != nil {
			return fmt.Errorf("record lottery sms: %w", err)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return nil
}

// deliver 以有界并发发送一批短信，返回每个选手是否发送成功。
func (s *LotterySweeper) deliver(ctx context.Context, jobs []lotterySMSJob) map[lotterySMSRecipient]bool {
	results := make(map[lotterySMSRecipient]bool, len(jobs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, lotterySMSParallel)

	for _, job := range jobs {
		wg.Add(1)
		go func(job lotterySMSJob) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				mu.Lock()
				for _, recipient := range job.recipients {
					results[recipient] = false
				}
				mu.Unlock()
				return
			}
			err := s.sms.Send(ctx, job.phone, job.template, job.params)
			mu.Lock()
			for _, recipient := range job.recipients {
				results[recipient] = err == nil
			}
			mu.Unlock()
			if err != nil {
				logger.WarnEvent(ctx, "lottery.sms_failed", "短信发送失败",
					"phone", utils.MaskPhone(job.phone), "error", err)
			}
		}(job)
	}
	wg.Wait()
	return results
}

// recordSMS 把发送结果写回抽签文件：成功记 SentAt，所有尝试都累加 Attempts。
func (s *LotterySweeper) recordSMS(ctx context.Context, token, kind string, results map[lotterySMSRecipient]bool) error {
	if len(results) == 0 {
		return nil
	}
	sentAt := utils.NowISO()
	_, err := s.store.Mutate(ctx, token, func(data *model.LotteryData) error {
		for recipient, ok := range results {
			player := findLotteryPlayer(data, recipient.idCard, recipient.genre)
			if player == nil {
				continue
			}
			if player.SMS == nil {
				player.SMS = &model.LotterySMSState{}
			}
			switch kind {
			case lotterySMSKindRemind:
				player.SMS.RemindAttempts++
				if ok {
					player.SMS.RemindSentAt = sentAt
				}
			case lotterySMSKindResult:
				player.SMS.ResultAttempts++
				if ok {
					player.SMS.ResultSentAt = sentAt
				}
			}
		}
		return nil
	})
	return err
}
