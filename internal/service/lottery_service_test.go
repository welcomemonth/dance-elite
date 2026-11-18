package service

import (
	"context"
	"errors"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/httpx"
	"github.com/meetmonth/dance-elite/internal/store"
)

const testLotteryToken = "TestToken1"

// newLotteryServiceForTest 构造一个含 3 名 U11 古典舞选手、仅 2 个槽位的数据集，
// 便于同时验证抽签、幂等与槽位耗尽。
func newLotteryServiceForTest(t *testing.T) *LotteryService {
	t.Helper()
	st := store.NewLotteryStore(t.TempDir())
	data := &model.LotteryData{
		Version:   1,
		EventName: "测试赛事抽签",
		Genres:    []model.LotteryGenre{{Key: "classical", Label: "古典舞"}},
		Groups:    []model.LotteryGroup{{Genre: "classical", AgeGroup: "U11", Letter: "A", Size: 2}},
		Players: []model.LotteryPlayer{
			{Name: "张三", IDCard: "310101201501011234", Genre: "classical", AgeGroup: "U11"},
			{Name: "李四", IDCard: "310101201501015678", Genre: "classical", AgeGroup: "U11"},
			{Name: "王五", IDCard: "310101201501019012", Genre: "classical", AgeGroup: "U11"},
		},
	}
	if err := st.Save(context.Background(), testLotteryToken, data); err != nil {
		t.Fatalf("save lottery data: %v", err)
	}
	return NewLotteryService(st)
}

func lotteryDrawReq(name, idCard string) model.LotteryDrawRequest {
	return model.LotteryDrawRequest{Name: name, IDCard: idCard, Genre: "classical"}
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

func TestLotteryDrawAssignsAndIsIdempotent(t *testing.T) {
	svc := newLotteryServiceForTest(t)
	ctx := context.Background()
	req := lotteryDrawReq("张三", "310101201501011234")

	first, err := svc.Draw(ctx, testLotteryToken, req)
	if err != nil {
		t.Fatalf("first draw: %v", err)
	}
	if first.Slot == nil || !first.JustDrawn {
		t.Fatalf("first draw = %+v, want slot and just_drawn", first)
	}
	if first.GenreLabel != "古典舞" {
		t.Fatalf("genre_label = %q", first.GenreLabel)
	}

	second, err := svc.Draw(ctx, testLotteryToken, req)
	if err != nil {
		t.Fatalf("second draw: %v", err)
	}
	if second.JustDrawn {
		t.Fatal("second draw should not draw again")
	}
	if *second.Slot != *first.Slot {
		t.Fatalf("slot changed on repeat draw: %+v -> %+v", first.Slot, second.Slot)
	}

	queried, err := svc.Query(ctx, testLotteryToken, model.LotteryQueryRequest{
		IDCard: "310101201501011234", Genre: "classical",
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if *queried.Slot != *first.Slot {
		t.Fatalf("query slot = %+v, want %+v", queried.Slot, first.Slot)
	}
}

func TestLotteryDrawNormalizesNameAndID(t *testing.T) {
	svc := newLotteryServiceForTest(t)
	// 姓名中的空格与证件号首尾空白都应被规范化后匹配。
	req := model.LotteryDrawRequest{Name: " 张 三 ", IDCard: " 310101201501011234 ", Genre: "classical"}
	if _, err := svc.Draw(context.Background(), testLotteryToken, req); err != nil {
		t.Fatalf("draw with normalized input: %v", err)
	}
}

func TestLotteryDrawRejectsMismatch(t *testing.T) {
	svc := newLotteryServiceForTest(t)
	ctx := context.Background()

	_, err := svc.Draw(ctx, testLotteryToken, lotteryDrawReq("李四", "310101201501011234"))
	assertLotteryBizCode(t, err, httpx.CodeNotFound)

	_, err = svc.Draw(ctx, testLotteryToken, lotteryDrawReq("张三", "310101201501019999"))
	assertLotteryBizCode(t, err, httpx.CodeNotFound)

	_, err = svc.Draw(ctx, testLotteryToken, model.LotteryDrawRequest{
		Name: "张三", IDCard: "310101201501011234", Genre: "folk",
	})
	assertLotteryBizCode(t, err, httpx.CodeNotFound)
}

func TestLotteryQueryBeforeDraw(t *testing.T) {
	svc := newLotteryServiceForTest(t)
	got, err := svc.Query(context.Background(), testLotteryToken, model.LotteryQueryRequest{
		IDCard: "310101201501015678", Genre: "classical",
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if got.Slot != nil {
		t.Fatalf("slot = %+v, want nil", got.Slot)
	}
	if got.Name != "李四" {
		t.Fatalf("name = %q", got.Name)
	}
}

func TestLotteryDrawExhaustedSlots(t *testing.T) {
	svc := newLotteryServiceForTest(t)
	ctx := context.Background()

	if _, err := svc.Draw(ctx, testLotteryToken, lotteryDrawReq("张三", "310101201501011234")); err != nil {
		t.Fatalf("draw 张三: %v", err)
	}
	if _, err := svc.Draw(ctx, testLotteryToken, lotteryDrawReq("李四", "310101201501015678")); err != nil {
		t.Fatalf("draw 李四: %v", err)
	}
	_, err := svc.Draw(ctx, testLotteryToken, lotteryDrawReq("王五", "310101201501019012"))
	assertLotteryBizCode(t, err, httpx.CodeConflict)
}

func TestLotteryMetaAndInvalidToken(t *testing.T) {
	svc := newLotteryServiceForTest(t)
	meta, err := svc.Meta(context.Background(), testLotteryToken)
	if err != nil {
		t.Fatalf("meta: %v", err)
	}
	if meta.EventName != "测试赛事抽签" || len(meta.Genres) != 1 {
		t.Fatalf("meta = %+v", meta)
	}

	_, err = svc.Meta(context.Background(), "BadToken1")
	assertLotteryBizCode(t, err, httpx.CodeNotFound)
}
