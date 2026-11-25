package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
)

func TestAdminLotteryDetailShowsDrawIdentityAndMatchedUser(t *testing.T) {
	stores := newLotteryTestStores(t)
	mustSeedLottery(t, stores, testLotterySeed{
		Token: testLotteryToken, EventName: "测试赛事抽签",
		Groups: []model.LotteryGroup{
			{Genre: "classical", AgeGroup: "U11", Letter: "A", Size: 1},
		},
		Players: []testLotteryPlayer{
			{Name: "张三", IDCard: "310101201501011234", Genre: "classical", AgeGroup: "U11",
				Slot: &model.LotterySlot{Letter: "A", No: 1}, DrawnAt: "2026-11-25T10:00:00+08:00",
				DrawnOpenID: "mp-openid-1", DrawnUnionID: "unionid-1"},
		},
	})
	var userID int64
	err := stores.WithTx(context.Background(), func(tx *sql.Tx) error {
		user, _, err := stores.Users.UpsertLogin(context.Background(), tx, store.LoginParams{
			OpenID: "mp-openid", UnionID: "unionid-1", Nickname: "家长", Now: utils.NowISO(),
		})
		if err != nil {
			return err
		}
		userID = user.ID
		return stores.Users.UpdatePhone(context.Background(), tx, userID, "13800138000")
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	adminSvc := NewAdminLotteryService(stores, "", testLotteryIDKey)
	detail, err := adminSvc.Get(context.Background(), testLotteryToken)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(detail.Draws) != 1 {
		t.Fatalf("draws = %d, want 1", len(detail.Draws))
	}
	draw := detail.Draws[0]
	if draw.OpenID != "mp-openid-1" || draw.UnionID != "unionid-1" || draw.AutoDrawn {
		t.Fatalf("draw = %+v", draw)
	}
	if draw.User == nil || draw.User.ID != userID || draw.User.Nickname != "家长" {
		t.Fatalf("matched user = %+v", draw.User)
	}
	if draw.User.Phone != "138****8000" {
		t.Fatalf("masked phone = %q", draw.User.Phone)
	}
	if draw.SameWechatCount != 1 {
		t.Fatalf("same_wechat_count = %d, want 1", draw.SameWechatCount)
	}
}

func TestAdminLotteryDetailAutoDrawHasNoIdentity(t *testing.T) {
	stores := newLotteryTestStores(t)
	mustSeedLottery(t, stores, testLotterySeed{
		Token: testLotteryToken, EventName: "测试赛事抽签",
		Groups: []model.LotteryGroup{
			{Genre: "classical", AgeGroup: "U11", Letter: "A", Size: 1},
		},
		Players: []testLotteryPlayer{
			{Name: "张三", IDCard: "310101201501011234", Genre: "classical", AgeGroup: "U11",
				Slot: &model.LotterySlot{Letter: "A", No: 1}, AutoDrawn: true},
		},
	})
	adminSvc := NewAdminLotteryService(stores, "", testLotteryIDKey)
	detail, err := adminSvc.Get(context.Background(), testLotteryToken)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(detail.Draws) != 1 || !detail.Draws[0].AutoDrawn || detail.Draws[0].OpenID != "" {
		t.Fatalf("draws = %+v", detail.Draws)
	}
	if detail.Draws[0].SameWechatCount != 0 {
		t.Fatalf("auto draw same_wechat_count = %d, want 0", detail.Draws[0].SameWechatCount)
	}
}

func TestAdminLotteryDetailSameWechatCount(t *testing.T) {
	stores := newLotteryTestStores(t)
	mustSeedLottery(t, stores, testLotterySeed{
		Token: testLotteryToken, EventName: "测试赛事抽签",
		Groups: []model.LotteryGroup{
			{Genre: "classical", AgeGroup: "U11", Letter: "A", Size: 2},
		},
		Players: []testLotteryPlayer{
			{Name: "张三", IDCard: "310101201501011234", Genre: "classical", AgeGroup: "U11",
				Slot: &model.LotterySlot{Letter: "A", No: 1}, DrawnAt: "2026-11-25T10:00:00+08:00",
				DrawnOpenID: "mp-openid-1", DrawnUnionID: "unionid-1"},
			{Name: "李四", IDCard: "310101201501015678", Genre: "classical", AgeGroup: "U11",
				Slot: &model.LotterySlot{Letter: "A", No: 2}, DrawnAt: "2026-11-25T10:01:00+08:00",
				DrawnOpenID: "mp-openid-1", DrawnUnionID: "unionid-1"},
		},
	})
	adminSvc := NewAdminLotteryService(stores, "", testLotteryIDKey)
	detail, err := adminSvc.Get(context.Background(), testLotteryToken)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(detail.Draws) != 2 {
		t.Fatalf("draws = %d, want 2", len(detail.Draws))
	}
	for _, draw := range detail.Draws {
		if draw.SameWechatCount != 2 {
			t.Fatalf("%s same_wechat_count = %d, want 2", draw.PlayerName, draw.SameWechatCount)
		}
	}
}
