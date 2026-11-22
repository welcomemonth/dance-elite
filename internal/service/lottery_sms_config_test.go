package service

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// autoDrawnLotteryData 构造截止后已由系统自动分配、但结果短信未发送的数据。
func autoDrawnLotteryData(start, end time.Time) *model.LotteryData {
	data := lotteryWindowData(start, end)
	for i := range data.Players {
		data.Players[i].Slot = &model.LotterySlot{Letter: "A", No: i + 1}
		data.Players[i].AutoDrawn = true
	}
	return data
}

func TestMissingLotterySMSConfig(t *testing.T) {
	now := time.Date(2026, 11, 27, 21, 0, 0, 0, utils.ChinaLoc)
	const baseURL = "https://dance.test"

	tests := []struct {
		name    string
		data    *model.LotteryData
		baseURL string
		want    []string
	}{
		{
			name:    "无截止时间按设计不发短信",
			data:    lotteryWindowData(now.Add(-time.Hour), time.Time{}),
			baseURL: baseURL,
			want:    nil,
		},
		{
			name:    "截止前缺提醒模板",
			data:    lotteryWindowData(now.Add(-24*time.Hour), now.Add(time.Hour)),
			baseURL: baseURL,
			want:    []string{"sms_remind_template"},
		},
		{
			name: "截止前缺 PUBLIC_BASE_URL 拼链接",
			data: func() *model.LotteryData {
				data := lotteryWindowData(now.Add(-24*time.Hour), now.Add(time.Hour))
				data.SMSRemindTemplate = "SMS_REMIND"
				return data
			}(),
			baseURL: "",
			want:    []string{"PUBLIC_BASE_URL"},
		},
		{
			name: "截止前模板齐全",
			data: func() *model.LotteryData {
				data := lotteryWindowData(now.Add(-24*time.Hour), now.Add(time.Hour))
				data.SMSRemindTemplate = "SMS_REMIND"
				data.SMSResultTemplate = "SMS_RESULT"
				return data
			}(),
			baseURL: baseURL,
			want:    nil,
		},
		{
			name: "截止后缺结果模板",
			data: autoDrawnLotteryData(now.Add(-24*time.Hour), now.Add(-time.Minute)),
			// 结果短信只需要结果模板，提醒模板缺失不再告警。
			baseURL: baseURL,
			want:    []string{"sms_result_template"},
		},
		{
			name: "截止后结果短信已发完",
			data: func() *model.LotteryData {
				data := autoDrawnLotteryData(now.Add(-24*time.Hour), now.Add(-time.Minute))
				for i := range data.Players {
					data.Players[i].SMS = &model.LotterySMSState{ResultSentAt: utils.FormatISO(now)}
				}
				return data
			}(),
			baseURL: baseURL,
			want:    nil,
		},
		{
			name: "截止后手动抽签的选手不发结果短信",
			data: func() *model.LotteryData {
				data := autoDrawnLotteryData(now.Add(-24*time.Hour), now.Add(-time.Minute))
				for i := range data.Players {
					data.Players[i].AutoDrawn = false
				}
				return data
			}(),
			baseURL: baseURL,
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := missingLotterySMSConfig(tt.data, tt.baseURL, now)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("missing config = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestLotterySweeperSkipsSilentlyWithoutTemplate 校验缺模板时不发送、也不报错，
// 告警只落在日志里，不影响其他赛事与本轮调度。
func TestLotterySweeperSkipsSilentlyWithoutTemplate(t *testing.T) {
	now := time.Date(2026, 11, 27, 21, 0, 0, 0, utils.ChinaLoc)
	data := lotteryWindowData(now.Add(-24*time.Hour), now.Add(time.Hour))
	client := &fakeSMSClient{}
	sweeper, _ := newLotterySweeperForTest(t, data, now, client)

	if err := sweeper.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}
	if calls := client.callCount(); calls != 0 {
		t.Fatalf("sms sent %d times, want 0 without template", calls)
	}
}
