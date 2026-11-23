package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
)

func newLotteryTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Migrate(context.Background(), db.SQL()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return New(db)
}

func testLotteryParams(eventName, token string) LotterySessionParams {
	start := "2026-11-25T10:00:00+08:00"
	end := "2026-11-27T22:00:00+08:00"
	return LotterySessionParams{
		EventName:           eventName,
		Token:               token,
		Source:              "excel",
		StartAt:             &start,
		EndAt:               &end,
		RemindBeforeMinutes: 120,
	}
}

func lotteryTestGroups() []model.LotteryGroup {
	return []model.LotteryGroup{{Genre: "classical", AgeGroup: "U11", Letter: "A", Size: 2}}
}

func testLotteryPlayers() []LotteryPlayerParams {
	return []LotteryPlayerParams{
		{Name: "张三", IDCardHash: "hash-a", Genre: "classical", AgeGroup: "U11"},
		{Name: "李四", IDCardHash: "hash-b", Genre: "classical", AgeGroup: "U11"},
	}
}

func createTestLottery(t *testing.T, s *Store, token string) (int64, []LotteryPlayerParams) {
	t.Helper()
	players := testLotteryPlayers()
	var sessionID int64
	err := s.WithTx(context.Background(), func(tx *sql.Tx) error {
		var err error
		sessionID, err = s.Lottery.Create(context.Background(), tx, testLotteryParams("测试赛事", token),
			lotteryTestGroups(), players)
		return err
	})
	if err != nil {
		t.Fatalf("create lottery: %v", err)
	}
	return sessionID, players
}

func TestLotteryStoreCreateGetAndSlot(t *testing.T) {
	s := newLotteryTestStore(t)
	ctx := context.Background()
	sessionID, _ := createTestLottery(t, s, "Token00001")

	exists, err := s.Lottery.ExistsToken(ctx, "Token00001")
	if err != nil || !exists {
		t.Fatalf("ExistsToken = %v/%v", exists, err)
	}

	session, groups, players, err := s.Lottery.GetByToken(ctx, "Token00001")
	if err != nil {
		t.Fatalf("GetByToken: %v", err)
	}
	if session.ID != sessionID || session.PlayerCount != 2 || session.DrawnCount != 0 {
		t.Fatalf("session = %+v", session)
	}
	if len(groups) != 1 || groups[0].Size != 2 {
		t.Fatalf("groups = %+v", groups)
	}
	if len(players) != 2 {
		t.Fatalf("players = %d", len(players))
	}

	var drawn, again bool
	err = s.WithTx(ctx, func(tx *sql.Tx) error {
		var err error
		drawn, err = s.Lottery.UpdatePlayerSlot(ctx, tx, players[0].ID, "A", 1, "2026-11-25T10:01:00+08:00", false)
		if err != nil {
			return err
		}
		again, err = s.Lottery.UpdatePlayerSlot(ctx, tx, players[0].ID, "A", 2, "2026-11-25T10:02:00+08:00", false)
		return err
	})
	if err != nil || !drawn || again {
		t.Fatalf("UpdatePlayerSlot = drawn:%v again:%v err:%v", drawn, again, err)
	}

	_, _, players, err = s.Lottery.GetByToken(ctx, "Token00001")
	if err != nil {
		t.Fatalf("GetByToken after draw: %v", err)
	}
	if players[0].SlotNo == nil || *players[0].SlotNo != 1 || players[0].Slot() == nil {
		t.Fatalf("player[0] = %+v", players[0])
	}

	list, err := s.Lottery.List(ctx)
	if err != nil || len(list) != 1 || list[0].DrawnCount != 1 {
		t.Fatalf("List = %+v/%v", list, err)
	}

	if _, _, _, err := s.Lottery.GetByToken(ctx, "Missing01"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing token err = %v", err)
	}
}
