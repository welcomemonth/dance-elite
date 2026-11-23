package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// fakeSMSClient 记录发送内容并可模拟失败，用于验证提醒/结果短信逻辑。
type fakeSMSClient struct {
	mu    sync.Mutex
	sent  []fakeSMSMessage
	calls int
	fail  bool
}

type fakeSMSMessage struct {
	phone    string
	template string
	params   map[string]string
}

func (f *fakeSMSClient) Send(_ context.Context, phone, template string, params map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.fail {
		return errors.New("mock sms failure")
	}
	f.sent = append(f.sent, fakeSMSMessage{phone: phone, template: template, params: params})
	return nil
}

func (f *fakeSMSClient) messages() []fakeSMSMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeSMSMessage(nil), f.sent...)
}

func (f *fakeSMSClient) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func newLotterySweeperForTest(t *testing.T, seed testLotterySeed, now time.Time, client *fakeSMSClient) (*LotterySweeper, *store.Store) {
	t.Helper()
	stores := newLotteryTestStores(t)
	mustSeedLottery(t, stores, seed)
	sweeper := NewLotterySweeper(stores, client, "https://dance.test")
	sweeper.now = func() time.Time { return now }
	return sweeper, stores
}

func loadSeededPlayers(t *testing.T, stores *store.Store) []model.LotteryPlayer {
	t.Helper()
	_, _, players, err := stores.Lottery.GetByToken(context.Background(), windowTestToken)
	if err != nil {
		t.Fatalf("load players: %v", err)
	}
	return players
}

func TestLotterySweeperSendsMergedReminderOnce(t *testing.T) {
	now := time.Date(2026, 11, 27, 21, 0, 0, 0, utils.ChinaLoc)
	seed := lotteryWindowSeed(now.Add(-24*time.Hour), now.Add(time.Hour))
	seed.RemindTpl = "SMS_REMIND"
	seed.ResultTpl = "SMS_RESULT"
	seed.Players[0].Phone = "13800000009"
	seed.Players[1].Phone = "13800000009" // 同手机号合并为一条
	client := &fakeSMSClient{}
	sweeper, stores := newLotterySweeperForTest(t, seed, now, client)

	if err := sweeper.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}
	messages := client.messages()
	if len(messages) != 1 {
		t.Fatalf("sent %d messages, want 1 merged reminder", len(messages))
	}
	if messages[0].phone != "13800000009" || messages[0].template != "SMS_REMIND" {
		t.Fatalf("message = %+v", messages[0])
	}
	if got := messages[0].params["name"]; got != "张三、李四" {
		t.Fatalf("merged name = %q", got)
	}
	if got := messages[0].params["deadline"]; got != lotteryTimeText(now.Add(time.Hour)) {
		t.Fatalf("deadline = %q", got)
	}

	// 幂等：下一轮不重复发送。
	if err := sweeper.RunOnce(context.Background()); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if got := len(client.messages()); got != 1 {
		t.Fatalf("sent %d messages after second run, want 1", got)
	}

	for _, player := range loadSeededPlayers(t, stores) {
		if player.SMS.RemindSentAt == nil || *player.SMS.RemindSentAt == "" {
			t.Fatalf("player %s reminder not recorded", player.Name)
		}
	}
}

func TestLotterySweeperAutoDrawsAndNotifiesOnlyAutoDrawn(t *testing.T) {
	now := time.Date(2026, 11, 28, 0, 0, 0, 0, utils.ChinaLoc)
	seed := lotteryWindowSeed(now.Add(-48*time.Hour), now.Add(-time.Minute))
	seed.RemindTpl = "SMS_REMIND"
	seed.ResultTpl = "SMS_RESULT"
	// 张三已自行抽签，只应给自动分配的李四发结果短信。
	seed.Players[0].Slot = &model.LotterySlot{Letter: "A", No: 1}
	seed.Players[0].DrawnAt = utils.FormatISO(now.Add(-time.Hour))
	seed.Players[1].Phone = "13800000002"
	client := &fakeSMSClient{}
	sweeper, stores := newLotterySweeperForTest(t, seed, now, client)

	if err := sweeper.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}

	players := loadSeededPlayers(t, stores)
	autoDrawn, manual := players[1], players[0]
	if autoDrawn.SlotNo == nil || !autoDrawn.AutoDrawn {
		t.Fatalf("auto player = %+v, want auto-assigned slot", autoDrawn)
	}
	if manual.AutoDrawn || manual.SlotNo == nil || *manual.SlotNo != 1 {
		t.Fatalf("manual player = %+v, should keep original slot", manual)
	}

	messages := client.messages()
	if len(messages) != 1 {
		t.Fatalf("sent %d messages, want 1 result message", len(messages))
	}
	if messages[0].template != "SMS_RESULT" || messages[0].phone != "13800000002" {
		t.Fatalf("message = %+v", messages[0])
	}
	if got := messages[0].params["slot"]; got != lotterySlotText(*autoDrawn.Slot()) {
		t.Fatalf("slot param = %q", got)
	}

	if err := sweeper.RunOnce(context.Background()); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if got := len(client.messages()); got != 1 {
		t.Fatalf("sent %d messages after second run, want 1", got)
	}
}

func TestLotterySweeperStopsAfterMaxAttempts(t *testing.T) {
	now := time.Date(2026, 11, 27, 21, 0, 0, 0, utils.ChinaLoc)
	seed := lotteryWindowSeed(now.Add(-24*time.Hour), now.Add(time.Hour))
	seed.RemindTpl = "SMS_REMIND"
	seed.Players[0].Phone = "13800000001"
	seed.Players[1].Phone = ""
	client := &fakeSMSClient{fail: true}
	sweeper, stores := newLotterySweeperForTest(t, seed, now, client)

	for i := 0; i < lotterySMSMaxAttempts+2; i++ {
		if err := sweeper.RunOnce(context.Background()); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	if got := client.callCount(); got != lotterySMSMaxAttempts {
		t.Fatalf("send calls = %d, want %d", got, lotterySMSMaxAttempts)
	}

	players := loadSeededPlayers(t, stores)
	if players[0].SMS.RemindSentAt != nil {
		t.Fatalf("failed send should not be marked sent: %+v", players[0].SMS)
	}
	if players[0].SMS.RemindAttempts != lotterySMSMaxAttempts {
		t.Fatalf("attempts = %d, want %d", players[0].SMS.RemindAttempts, lotterySMSMaxAttempts)
	}
	if players[1].SMS.RemindAttempts != 0 {
		t.Fatalf("player without phone should not be attempted: %+v", players[1].SMS)
	}
}

func TestLotterySweeperSkipsWithoutTemplate(t *testing.T) {
	now := time.Date(2026, 11, 27, 21, 0, 0, 0, utils.ChinaLoc)
	seed := lotteryWindowSeed(now.Add(-24*time.Hour), now.Add(time.Hour))
	seed.Players[0].Phone = "13800000001"
	client := &fakeSMSClient{}
	sweeper, _ := newLotterySweeperForTest(t, seed, now, client)

	if err := sweeper.RunOnce(context.Background()); err != nil {
		t.Fatalf("run once: %v", err)
	}
	if got := client.callCount(); got != 0 {
		t.Fatalf("send calls = %d, want 0 without template", got)
	}
}
