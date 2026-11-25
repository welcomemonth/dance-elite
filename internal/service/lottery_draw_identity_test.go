package service

import (
	"context"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
)

func TestLotteryDrawRecordsWechatIdentity(t *testing.T) {
	stores := newLotteryTestStores(t)
	mustSeedLottery(t, stores, testLotterySeed{
		Token: testLotteryToken, EventName: "测试赛事抽签",
		Groups: []model.LotteryGroup{
			{Genre: "classical", AgeGroup: "U11", Letter: "A", Size: 1},
		},
		Players: []testLotteryPlayer{
			{Name: "张三", IDCard: "310101201501011234", Genre: "classical", AgeGroup: "U11"},
		},
	})
	lotterySvc := NewLotteryService(stores, testLotteryIDKey)

	actor := LotteryActor{OpenID: "mp-openid-1", UnionID: "unionid-1"}
	if _, err := lotterySvc.Draw(context.Background(), testLotteryToken,
		lotteryDrawReq("张三", "310101201501011234", "classical"), actor); err != nil {
		t.Fatalf("Draw: %v", err)
	}

	_, _, players, err := stores.Lottery.GetByToken(context.Background(), testLotteryToken)
	if err != nil {
		t.Fatalf("GetByToken: %v", err)
	}
	if len(players) != 1 || players[0].DrawnOpenID == nil || *players[0].DrawnOpenID != "mp-openid-1" {
		t.Fatalf("drawn openid = %+v", players[0].DrawnOpenID)
	}
	if players[0].DrawnUnionID == nil || *players[0].DrawnUnionID != "unionid-1" {
		t.Fatalf("drawn unionid = %+v", players[0].DrawnUnionID)
	}
}
