package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/pkg/secureid"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
)

const testLotteryIDKey = "test-id-key"

type testLotteryPlayer struct {
	Name        string
	IDCard      string
	Genre       string
	AgeGroup    string
	Institution string
	Program     string
	Phone       string
	Slot        *model.LotterySlot
	AutoDrawn   bool
	DrawnAt     string
}

type testLotterySeed struct {
	Token     string
	EventName string
	StartAt   *string
	EndAt     *string
	RemindTpl string
	ResultTpl string
	Groups    []model.LotteryGroup
	Players   []testLotteryPlayer
}

// newLotteryTestStores 打开内存库并完成迁移。
func newLotteryTestStores(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db.SQL()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store.New(db)
}

// mustSeedLottery 写入一场抽签，返回 session ID 与选手 ID。
func mustSeedLottery(t *testing.T, stores *store.Store, seed testLotterySeed) (int64, []int64) {
	t.Helper()
	ctx := context.Background()
	encKey := secureid.DeriveKey(testLotteryIDKey)
	params := make([]store.LotteryPlayerParams, 0, len(seed.Players))
	for _, player := range seed.Players {
		idCard := utils.NormalizeIDNumber(utils.IDTypeIDCard, player.IDCard)
		encrypted, err := secureid.Encrypt(encKey, idCard)
		if err != nil {
			t.Fatalf("encrypt id: %v", err)
		}
		params = append(params, store.LotteryPlayerParams{
			Name:         player.Name,
			IDCardHash:   utils.HashIDNumber([]byte(testLotteryIDKey), utils.IDTypeIDCard, idCard),
			IDCardMasked: strPtr(utils.MaskIDNumber(idCard)),
			IDCardEnc:    &encrypted,
			Genre:        player.Genre,
			AgeGroup:     player.AgeGroup,
			Institution:  trimmedPtr(player.Institution),
			Program:      trimmedPtr(player.Program),
			Phone:        trimmedPtr(player.Phone),
		})
	}
	var sessionID int64
	err := stores.WithTx(ctx, func(tx *sql.Tx) error {
		var err error
		sessionID, err = stores.Lottery.Create(ctx, tx, store.LotterySessionParams{
			EventName:           seed.EventName,
			Token:               seed.Token,
			Source:              model.LotterySourceExcel,
			StartAt:             seed.StartAt,
			EndAt:               seed.EndAt,
			RemindBeforeMinutes: 120,
			SMSRemindTemplate:   trimmedPtr(seed.RemindTpl),
			SMSResultTemplate:   trimmedPtr(seed.ResultTpl),
		}, seed.Groups, params)
		return err
	})
	if err != nil {
		t.Fatalf("seed lottery: %v", err)
	}
	_, _, players, err := stores.Lottery.GetByToken(ctx, seed.Token)
	if err != nil {
		t.Fatalf("load seeded lottery: %v", err)
	}
	ids := make([]int64, 0, len(players))
	for i, player := range players {
		ids = append(ids, player.ID)
		if i < len(seed.Players) && seed.Players[i].Slot != nil {
			drawAt := seed.Players[i].DrawnAt
			if drawAt == "" {
				drawAt = utils.NowISO()
			}
			err := stores.WithTx(ctx, func(tx *sql.Tx) error {
				_, err := stores.Lottery.UpdatePlayerSlot(ctx, tx, player.ID,
					seed.Players[i].Slot.Letter, seed.Players[i].Slot.No, drawAt, seed.Players[i].AutoDrawn)
				return err
			})
			if err != nil {
				t.Fatalf("draw seeded player: %v", err)
			}
		}
	}
	return sessionID, ids
}

func lotteryTimePtr(t time.Time) *string {
	value := utils.FormatISO(t)
	return &value
}
