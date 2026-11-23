package service

import (
	"context"
	"errors"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
)

const testLotteryToken = "Token00001"

func newLotteryServiceForTest(t *testing.T) (*LotteryService, *store.Store) {
	t.Helper()
	stores := newLotteryTestStores(t)
	mustSeedLottery(t, stores, testLotterySeed{
		Token: testLotteryToken, EventName: "测试赛事抽签",
		Groups: []model.LotteryGroup{
			{Genre: "classical", AgeGroup: "U11", Letter: "A", Size: 2},
			{Genre: "folk", AgeGroup: "U11", Letter: "A", Size: 1},
		},
		Players: []testLotteryPlayer{
			{Name: "张三", IDCard: "310101201501011234", Genre: "classical", AgeGroup: "U11"},
			{Name: "张三", IDCard: "310101201501011234", Genre: "folk", AgeGroup: "U11"},
			{Name: "李四", IDCard: "310101201501015678", Genre: "classical", AgeGroup: "U11"},
			{Name: "王五", IDCard: "310101201501019012", Genre: "classical", AgeGroup: "U11"},
		},
	})
	return NewLotteryService(stores, testLotteryIDKey), stores
}

func lotteryDrawReq(name, idCard, genre string) model.LotteryDrawRequest {
	return model.LotteryDrawRequest{Name: name, IDCard: idCard, Genre: genre}
}

func assertLotteryBizCode(t *testing.T, err error, want int) {
	t.Helper()
	var biz *httpx.BizError
	if !errors.As(err, &biz) {
		t.Fatalf("err = %v, want *httpx.BizError", err)
	}
	if biz.Code != want {
		t.Fatalf("code = %d, want %d (message=%q)", biz.Code, want, biz.Message)
	}
}

func findEntry(t *testing.T, entries []model.LotteryEntry, genre string) model.LotteryEntry {
	t.Helper()
	for _, entry := range entries {
		if entry.Genre == genre {
			return entry
		}
	}
	t.Fatalf("entry genre %q not found in %+v", genre, entries)
	return model.LotteryEntry{}
}

// 同一姓名+证件号有两个待抽舞种时，先返回 choose_genre；指定舞种后只抽该舞种，
// 再次不带舞种提交会把剩下的一条抽完，之后返回 already_drawn。
func TestLotteryDrawChooseGenreThenCompletesAll(t *testing.T) {
	svc, _ := newLotteryServiceForTest(t)
	ctx := context.Background()
	req := lotteryDrawReq("张三", "310101201501011234", "")

	first, err := svc.Draw(ctx, testLotteryToken, req)
	if err != nil {
		t.Fatalf("first draw: %v", err)
	}
	if first.Status != model.LotteryOutcomeChooseGenre || len(first.Choices) != 2 {
		t.Fatalf("first = %+v, want choose_genre with 2 choices", first)
	}
	if len(first.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(first.Entries))
	}

	second, err := svc.Draw(ctx, testLotteryToken, lotteryDrawReq("张三", "310101201501011234", "classical"))
	if err != nil {
		t.Fatalf("draw classical: %v", err)
	}
	if second.Status != model.LotteryOutcomeDrawn {
		t.Fatalf("second status = %q", second.Status)
	}
	classical := findEntry(t, second.Entries, "classical")
	folk := findEntry(t, second.Entries, "folk")
	if !classical.Drawn || classical.Slot == nil || classical.GenreLabel != "古典舞" {
		t.Fatalf("classical = %+v", classical)
	}
	if folk.Drawn {
		t.Fatalf("folk should still be pending: %+v", folk)
	}
	if classical.DrawnAt == "" {
		t.Fatal("drawn_at is empty")
	}

	third, err := svc.Draw(ctx, testLotteryToken, req)
	if err != nil {
		t.Fatalf("draw remaining: %v", err)
	}
	if third.Status != model.LotteryOutcomeDrawn || !findEntry(t, third.Entries, "folk").Drawn {
		t.Fatalf("third = %+v, want folk drawn", third)
	}

	fourth, err := svc.Draw(ctx, testLotteryToken, req)
	if err != nil {
		t.Fatalf("draw again: %v", err)
	}
	if fourth.Status != model.LotteryOutcomeAlreadyDrawn {
		t.Fatalf("fourth status = %q", fourth.Status)
	}
	if *findEntry(t, fourth.Entries, "classical").Slot != *classical.Slot {
		t.Fatal("repeat draw changed slot")
	}
}

// 只有一个舞种时无需选择，直接抽签；重复提交幂等。
func TestLotteryDrawSingleGenreIsIdempotent(t *testing.T) {
	svc, _ := newLotteryServiceForTest(t)
	ctx := context.Background()
	req := lotteryDrawReq("李四", "310101201501015678", "")

	first, err := svc.Draw(ctx, testLotteryToken, req)
	if err != nil {
		t.Fatalf("first draw: %v", err)
	}
	if first.Status != model.LotteryOutcomeDrawn || len(first.Choices) != 0 {
		t.Fatalf("first = %+v", first)
	}
	entry := first.Entries[0]
	if entry.Slot == nil || entry.Slot.Letter != "A" {
		t.Fatalf("entry = %+v", entry)
	}

	second, err := svc.Draw(ctx, testLotteryToken, req)
	if err != nil {
		t.Fatalf("repeat draw: %v", err)
	}
	if second.Status != model.LotteryOutcomeAlreadyDrawn || *second.Entries[0].Slot != *entry.Slot {
		t.Fatalf("second = %+v", second)
	}
}

// 查询只需要身份证号，一次返回全部已抽签记录（含未抽记录）。
func TestLotteryQueryByIdCardOnlyReturnsAllEntries(t *testing.T) {
	svc, _ := newLotteryServiceForTest(t)
	ctx := context.Background()
	if _, err := svc.Draw(ctx, testLotteryToken, lotteryDrawReq("张三", "310101201501011234", "classical")); err != nil {
		t.Fatalf("draw: %v", err)
	}

	got, err := svc.Query(ctx, testLotteryToken, model.LotteryLookupRequest{IDCard: "310101201501011234"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(got.Entries))
	}
	if !findEntry(t, got.Entries, "classical").Drawn {
		t.Fatal("classical should be drawn")
	}
	if findEntry(t, got.Entries, "folk").Drawn {
		t.Fatal("folk should be pending")
	}
}

func TestLotteryDrawNormalizesNameAndID(t *testing.T) {
	svc, _ := newLotteryServiceForTest(t)
	// 姓名中的空格与证件号首尾空白都应被规范化后匹配。
	req := model.LotteryDrawRequest{Name: " 张 三 ", IDCard: " 310101201501011234 ", Genre: "classical"}
	if _, err := svc.Draw(context.Background(), testLotteryToken, req); err != nil {
		t.Fatalf("draw with normalized input: %v", err)
	}
}

func TestLotteryDrawRejectsMismatch(t *testing.T) {
	svc, _ := newLotteryServiceForTest(t)
	ctx := context.Background()

	_, err := svc.Draw(ctx, testLotteryToken, lotteryDrawReq("李四", "310101201501011234", ""))
	assertLotteryBizCode(t, err, httpx.CodeNotFound)

	_, err = svc.Draw(ctx, testLotteryToken, lotteryDrawReq("张三", "310101201501019999", ""))
	assertLotteryBizCode(t, err, httpx.CodeNotFound)

	_, err = svc.Draw(ctx, testLotteryToken, lotteryDrawReq("张三", "310101201501011234", "unknown"))
	assertLotteryBizCode(t, err, httpx.CodeNotFound)
}

func TestLotteryQueryUnknownIdCardGivesActionableMessage(t *testing.T) {
	svc, _ := newLotteryServiceForTest(t)
	_, err := svc.Query(context.Background(), testLotteryToken, model.LotteryLookupRequest{IDCard: "310101201501019999"})
	assertLotteryBizCode(t, err, httpx.CodeNotFound)
	var biz *httpx.BizError
	_ = errors.As(err, &biz)
	if biz == nil || biz.Message == "" {
		t.Fatal("want a user-facing message")
	}
}

func TestLotteryDrawExhaustedSlots(t *testing.T) {
	svc, _ := newLotteryServiceForTest(t)
	ctx := context.Background()

	if _, err := svc.Draw(ctx, testLotteryToken, lotteryDrawReq("张三", "310101201501011234", "classical")); err != nil {
		t.Fatalf("draw 张三: %v", err)
	}
	if _, err := svc.Draw(ctx, testLotteryToken, lotteryDrawReq("李四", "310101201501015678", "")); err != nil {
		t.Fatalf("draw 李四: %v", err)
	}
	_, err := svc.Draw(ctx, testLotteryToken, lotteryDrawReq("王五", "310101201501019012", ""))
	assertLotteryBizCode(t, err, httpx.CodeConflict)
}

func TestLotteryMetaAndInvalidToken(t *testing.T) {
	svc, _ := newLotteryServiceForTest(t)
	meta, err := svc.Meta(context.Background(), testLotteryToken)
	if err != nil {
		t.Fatalf("meta: %v", err)
	}
	if meta.EventName != "测试赛事抽签" || len(meta.Genres) != 2 {
		t.Fatalf("meta = %+v", meta)
	}

	_, err = svc.Meta(context.Background(), "BadToken1")
	assertLotteryBizCode(t, err, httpx.CodeNotFound)
}
