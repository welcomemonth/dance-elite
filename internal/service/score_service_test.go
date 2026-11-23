package service

import (
	"context"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/store"
)

func newScoreFixture(t *testing.T) (*regFixture, *ScoreService, *RankingAdminService, *PublicRankingService, int64) {
	t.Helper()
	f := newRegFixture(t)
	players := NewPlayerService(f.stores, "test-hmac-key")
	scores := NewScoreService(f.stores, players)
	admin := NewRankingAdminService(f.stores)
	public := NewPublicRankingService(f.stores)
	var adminID int64
	if err := f.db.SQL().QueryRowContext(context.Background(),
		`SELECT id FROM admins WHERE username = 'admin'`).Scan(&adminID); err != nil {
		t.Fatalf("admin id: %v", err)
	}
	return f, scores, admin, public, adminID
}

// mustScoredEvent 创建报名中的赛事，并为指定选手建档+报名，返回赛事与玩家 ID。
func mustScoredEvent(t *testing.T, f *regFixture, tag string, quota int, names ...string) (eventID int64, players []int64) {
	t.Helper()
	eventID = mustCreateRegisteringEvent(t, f, "成绩赛事-"+tag, []model.AdminEventGroupInput{
		{Level: model.GroupLevelU13, DanceType: model.DanceTypeClassical, Quota: quota},
	})
	groupID := mustGroupID(t, f.db, eventID, model.GroupLevelU13, model.DanceTypeClassical)
	for i, name := range names {
		userID := mustInsertRegUser(t, f.db, tag+"-user-"+itoaTest(i))
		playerNo := padTest(tag, i)
		playerID := mustScorePlayer(t, f.db, playerNo, name)
		mustInsertRegistrationRow(t, f.db, eventID, groupID, playerID, userID, "机构A")
		players = append(players, playerID)
	}
	return eventID, players
}

func TestScorePublishComputesPointsAndSnapshot(t *testing.T) {
	f, scores, admin, public, adminID := newScoreFixture(t)
	eventID, players := mustScoredEvent(t, f, "Pub", 10, "甲选手", "乙选手", "丙选手")
	ctx := context.Background()

	rows := []model.ScorePreviewInputRow{
		{RowNo: 1, PlayerName: "甲选手", Award: model.AwardChampion},
		{RowNo: 2, PlayerName: "乙选手", Award: model.AwardRunnerUp},
		{RowNo: 3, PlayerName: "丙选手", Award: model.AwardThird},
	}
	preview, err := scores.Preview(ctx, eventID, "U13", "classical", rows)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Matched != 3 || preview.Unmatched != 0 {
		t.Fatalf("preview matched=%d unmatched=%d", preview.Matched, preview.Unmatched)
	}

	if _, err := scores.Publish(ctx, adminID, eventID, "U13", "classical", preview.Rows, "batch-1"); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// 3 人参赛 → 人数系数 0.8；冠军 100×1.0×0.8=80，亚军 85×0.8=68，季军 75×0.8=60。
	boards, err := admin.ListLeaderboards(ctx, mustSeasonOf(t, f, eventID))
	if err != nil {
		t.Fatalf("leaderboards: %v", err)
	}
	var board *model.AdminLeaderboard
	for i := range boards.List {
		if boards.List[i].Level == "U13" && boards.List[i].DanceType == "classical" {
			board = &boards.List[i]
		}
	}
	if board == nil || len(board.Rows) != 3 {
		t.Fatalf("board rows = %v", board)
	}
	if board.Rows[0].PlayerID != players[0] || board.Rows[0].TotalPoints != 80 || board.Rows[0].Rank != 1 {
		t.Fatalf("rank1 = %+v", board.Rows[0])
	}

	// 小程序榜单可见且冠军持有直通卡。
	pub, err := public.Ranking(ctx, "U13", "classical")
	if err != nil {
		t.Fatalf("public ranking: %v", err)
	}
	if len(pub.Ranking) != 3 || len(pub.Direct) != 1 {
		t.Fatalf("public ranking=%d direct=%d", len(pub.Ranking), len(pub.Direct))
	}
	if !pub.Ranking[0].IsDirect {
		t.Fatal("champion should hold direct qualification")
	}

	// 隐藏后不占位、不出现在小程序榜。
	hidden := true
	if _, err := admin.UpdateLeaderboardRowByKey(ctx, adminID, mustSeasonOf(t, f, eventID),
		"U13", "classical", players[0], model.UpdateLeaderboardRowRequest{Hidden: &hidden}); err != nil {
		t.Fatalf("hide: %v", err)
	}
	pub, err = public.Ranking(ctx, "U13", "classical")
	if err != nil {
		t.Fatalf("public ranking after hide: %v", err)
	}
	if len(pub.Ranking) != 2 || pub.Ranking[0].Rank != 1 || pub.Ranking[0].TotalPoints != 68 {
		t.Fatalf("after hide ranking = %+v", pub.Ranking)
	}
}

func TestScorePublishPrizeAndCorrection(t *testing.T) {
	f, scores, admin, _, adminID := newScoreFixture(t)
	// 40 人门槛：这里只有 1 人 → 甲级赛冠军按低于门槛发 1000 元。
	eventID, _ := mustScoredEvent(t, f, "Prize", 10, "冠军选手")
	ctx := context.Background()

	preview, err := scores.Preview(ctx, eventID, "U13", "classical",
		[]model.ScorePreviewInputRow{{RowNo: 1, PlayerName: "冠军选手", Award: model.AwardChampion}})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if _, err := scores.Publish(ctx, adminID, eventID, "U13", "classical", preview.Rows, ""); err != nil {
		t.Fatalf("publish: %v", err)
	}
	prizes, err := NewPrizeService(f.stores).List(ctx, mustSeasonOf(t, f, eventID), "")
	if err != nil {
		t.Fatalf("prizes: %v", err)
	}
	if len(prizes) != 1 || prizes[0].Amount != 100000 {
		t.Fatalf("prize = %+v", prizes)
	}

	// 纠错：冠军改为亚军，旧记录置 corrected，奖金随之调整。
	entries, err := scores.List(ctx, eventID, "U13", "classical")
	if err != nil || len(entries) != 1 {
		t.Fatalf("list entries: %v %v", entries, err)
	}
	newAward := model.AwardRunnerUp
	if _, err := scores.Correct(ctx, adminID, entries[0].EntryID, model.CorrectScoreRequest{Award: newAward}); err != nil {
		t.Fatalf("correct: %v", err)
	}
	var corrected int
	_ = f.db.SQL().QueryRowContext(ctx,
		`SELECT COUNT(1) FROM results WHERE status = 'corrected'`).Scan(&corrected)
	if corrected != 1 {
		t.Fatalf("corrected rows = %d, want 1", corrected)
	}
	boards, err := admin.ListLeaderboards(ctx, mustSeasonOf(t, f, eventID))
	if err != nil {
		t.Fatalf("leaderboards: %v", err)
	}
	for _, b := range boards.List {
		if b.Level == "U13" && b.DanceType == "classical" {
			if len(b.Rows) != 1 || b.Rows[0].TotalPoints != 68 {
				t.Fatalf("after correction rows = %+v", b.Rows)
			}
		}
	}
}

func TestScorePreviewUnmatchedAndValidation(t *testing.T) {
	f, scores, _, _, _ := newScoreFixture(t)
	eventID, _ := mustScoredEvent(t, f, "Val", 5, "已知选手")
	ctx := context.Background()

	preview, err := scores.Preview(ctx, eventID, "U13", "classical",
		[]model.ScorePreviewInputRow{{RowNo: 1, PlayerName: "不存在", Award: model.AwardChampion}})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Unmatched != 1 {
		t.Fatalf("unmatched = %d", preview.Unmatched)
	}

	if _, err := scores.Preview(ctx, eventID, "U99", "classical", nil); err == nil {
		t.Fatal("expected invalid level error")
	}
}

// ---------- 夹具 ----------

func mustSeasonOf(t *testing.T, f *regFixture, eventID int64) int64 {
	t.Helper()
	var seasonID int64
	if err := f.db.SQL().QueryRowContext(context.Background(),
		`SELECT season_id FROM events WHERE id = ?`, eventID).Scan(&seasonID); err != nil {
		t.Fatalf("season of event %d: %v", eventID, err)
	}
	return seasonID
}

func mustScorePlayer(t *testing.T, db *store.DB, playerNo, name string) int64 {
	t.Helper()
	res, err := db.SQL().ExecContext(context.Background(),
		`INSERT INTO players (player_no, name, id_card_hash) VALUES (?, ?, ?)`,
		playerNo, name, "hash-"+playerNo)
	if err != nil {
		t.Fatalf("insert player %s: %v", playerNo, err)
	}
	id, _ := res.LastInsertId()
	return id
}

func mustInsertRegistrationRow(t *testing.T, db *store.DB, eventID, groupID, playerID, userID int64, institution string) {
	t.Helper()
	if _, err := db.SQL().ExecContext(context.Background(),
		`INSERT INTO registrations (event_id, group_id, player_id, user_id, dance_type, institution, status)
		 VALUES (?, ?, ?, ?, 'classical', ?, 'active')`,
		eventID, groupID, playerID, userID, institution); err != nil {
		t.Fatalf("insert registration: %v", err)
	}
}

func itoaTest(i int) string { return string(rune('a' + i)) }

// padTest 生成 8 位字母数字的 player_no（base 最多 6 位 + 1 位序号 + 1 位字母）。
func padTest(tag string, i int) string {
	base := tag
	if len(base) > 6 {
		base = base[:6]
	}
	for len(base) < 6 {
		base += "0"
	}
	return base + string(rune('0'+i%10)) + "A"
}
