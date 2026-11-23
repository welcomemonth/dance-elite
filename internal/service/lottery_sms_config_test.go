package service

import (
	"testing"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// smsTestSession 构造一场抽签会话（仅时间窗口与短信模板）。
func smsTestSession(start, end time.Time, remindTpl, resultTpl string) *model.LotterySession {
	session := &model.LotterySession{ID: 1, EventName: "短信测试", Token: "SmsToken01"}
	if !start.IsZero() {
		session.StartAt = lotteryTimePtr(start)
	}
	if !end.IsZero() {
		session.EndAt = lotteryTimePtr(end)
	}
	if remindTpl != "" {
		session.SMSRemindTemplate = &remindTpl
	}
	if resultTpl != "" {
		session.SMSResultTemplate = &resultTpl
	}
	return session
}

// smsTestPlayers 构造两名待抽签 / 已自动分配的选手。
func smsTestPlayers(autoDrawn bool) []model.LotteryPlayer {
	phone1, phone2 := "13800000001", "13800000002"
	players := []model.LotteryPlayer{
		{ID: 1, Name: "张三", Genre: "classical", AgeGroup: "U11", Phone: &phone1},
		{ID: 2, Name: "李四", Genre: "classical", AgeGroup: "U11", Phone: &phone2},
	}
	if autoDrawn {
		letter := "A"
		for i := range players {
			no := i + 1
			players[i].GroupLetter = &letter
			players[i].SlotNo = &no
			players[i].AutoDrawn = true
		}
	}
	return players
}

func TestMissingLotterySMSConfig(t *testing.T) {
	now := time.Date(2026, 11, 27, 21, 0, 0, 0, utils.ChinaLoc)
	const baseURL = "https://dance.test"

	tests := []struct {
		name    string
		session *model.LotterySession
		players []model.LotteryPlayer
		baseURL string
		want    []string
	}{
		{
			name:    "无截止时间按设计不发短信",
			session: smsTestSession(now.Add(-time.Hour), time.Time{}, "", ""),
			players: smsTestPlayers(false),
			baseURL: baseURL,
			want:    nil,
		},
		{
			name:    "截止前缺提醒模板",
			session: smsTestSession(now.Add(-24*time.Hour), now.Add(time.Hour), "", ""),
			players: smsTestPlayers(false),
			baseURL: baseURL,
			want:    []string{"sms_remind_template"},
		},
		{
			name:    "截止前缺 public_base_url 拼链接",
			session: smsTestSession(now.Add(-24*time.Hour), now.Add(time.Hour), "SMS_REMIND", ""),
			players: smsTestPlayers(false),
			baseURL: "",
			want:    []string{"public_base_url"},
		},
		{
			name:    "截止前模板齐全",
			session: smsTestSession(now.Add(-24*time.Hour), now.Add(time.Hour), "SMS_REMIND", "SMS_RESULT"),
			players: smsTestPlayers(false),
			baseURL: baseURL,
			want:    nil,
		},
		{
			name:    "已全部抽签不再告警",
			session: smsTestSession(now.Add(-24*time.Hour), now.Add(time.Hour), "", ""),
			players: func() []model.LotteryPlayer {
				players := smsTestPlayers(true)
				return players
			}(),
			baseURL: baseURL,
			want:    nil,
		},
		{
			name:    "截止后缺结果模板",
			session: smsTestSession(now.Add(-48*time.Hour), now.Add(-time.Minute), "SMS_REMIND", ""),
			players: smsTestPlayers(true),
			baseURL: baseURL,
			want:    []string{"sms_result_template"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := missingLotterySMSConfig(tc.session, tc.players, tc.baseURL, now)
			if len(got) != len(tc.want) {
				t.Fatalf("missing = %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("missing = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestPendingLotterySMSPlayers(t *testing.T) {
	players := smsTestPlayers(false)
	if got := pendingRemindPlayers(players); got != 2 {
		t.Fatalf("pendingRemindPlayers = %d, want 2", got)
	}
	if got := pendingResultPlayers(players); got != 0 {
		t.Fatalf("pendingResultPlayers = %d, want 0", got)
	}

	sentAt := utils.NowISO()
	players[0].SMS.RemindSentAt = &sentAt
	if got := pendingRemindPlayers(players); got != 1 {
		t.Fatalf("after sent pendingRemindPlayers = %d, want 1", got)
	}

	auto := smsTestPlayers(true)
	if got := pendingResultPlayers(auto); got != 2 {
		t.Fatalf("auto pendingResultPlayers = %d, want 2", got)
	}
	auto[1].SMS.ResultAttempts = lotterySMSMaxAttempts
	if got := pendingResultPlayers(auto); got != 1 {
		t.Fatalf("max attempts pendingResultPlayers = %d, want 1", got)
	}
}
