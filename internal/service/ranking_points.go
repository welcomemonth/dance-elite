package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"time"

	"github.com/meetmonth/dance-elite/internal/model"
	"github.com/meetmonth/dance-elite/internal/store"
	"github.com/meetmonth/dance-elite/internal/utils"
)

// advanceQuota 是每个榜单按积分晋级总决赛的名额（FAQ：前 20）。
const advanceQuota = 20

// recomputeBoardPoints 重算某榜单的选手积分快照并重建自动资格。
// 规则：每名选手取本年度积分最高的 3 场之和；隐藏选手不占排名。
func recomputeBoardPoints(ctx context.Context, stores *store.Store, tx *sql.Tx,
	seasonID, boardID int64, now time.Time) error {
	rows, err := stores.Results.ListPublishedBySeasonBoard(ctx, tx, seasonID, boardID)
	if err != nil {
		return err
	}

	type agg struct {
		total  int
		events []int64
	}
	byPlayer := map[int64]*agg{}
	for _, r := range rows {
		a := byPlayer[r.Result.PlayerID]
		if a == nil {
			a = &agg{}
			byPlayer[r.Result.PlayerID] = a
		}
		a.events = append(a.events, r.Result.EventID)
		a.total += r.Result.Points
	}

	// 按单场积分取最高 3 场：需要每场积分，按 event 聚合（同赛事同榜单同选手唯一）。
	perEvent := map[int64]map[int64]int{}
	for _, r := range rows {
		if perEvent[r.Result.PlayerID] == nil {
			perEvent[r.Result.PlayerID] = map[int64]int{}
		}
		perEvent[r.Result.PlayerID][r.Result.EventID] = r.Result.Points
	}
	points := make([]model.PlayerPoint, 0, len(byPlayer))
	for pid, m := range perEvent {
		type ev struct {
			id int64
			pt int
		}
		list := make([]ev, 0, len(m))
		for id, pt := range m {
			list = append(list, ev{id, pt})
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].pt != list[j].pt {
				return list[i].pt > list[j].pt
			}
			return list[i].id < list[j].id
		})
		total := 0
		used := make([]int64, 0, 3)
		for i, e := range list {
			if i >= 3 {
				break
			}
			total += e.pt
			used = append(used, e.id)
		}
		sort.Slice(used, func(i, j int) bool { return used[i] < used[j] })
		raw, _ := json.Marshal(used)
		points = append(points, model.PlayerPoint{
			PlayerID: pid, TotalPoints: total, UsedEventIDs: string(raw),
			ComputedAt: strPtr(utils.FormatISO(now)),
		})
	}

	// 隐藏选手不占排名：先取旧快照里的 hidden，再分配给非隐藏选手。
	hidden := map[int64]bool{}
	old, err := stores.Points.ListPlayerPointsByBoardTx(ctx, tx, boardID)
	if err != nil {
		return err
	}
	for _, o := range old {
		hidden[o.PlayerPoint.PlayerID] = o.PlayerPoint.Hidden
	}
	sort.Slice(points, func(i, j int) bool {
		if points[i].TotalPoints != points[j].TotalPoints {
			return points[i].TotalPoints > points[j].TotalPoints
		}
		return points[i].PlayerID < points[j].PlayerID
	})
	rank := 0
	for i := range points {
		if hidden[points[i].PlayerID] {
			continue
		}
		rank++
		r := rank
		points[i].Rank = &r
	}

	if err := stores.Points.ReplacePlayerPoints(ctx, tx, seasonID, boardID, points); err != nil {
		return err
	}
	return rebuildBoardQualifications(ctx, stores, tx, seasonID, boardID, points, rows)
}

// rebuildBoardQualifications 重建自动资格（直通 + 积分前 20），保留手动来源。
func rebuildBoardQualifications(ctx context.Context, stores *store.Store, tx *sql.Tx,
	seasonID, boardID int64, points []model.PlayerPoint, rows []store.ResultRow) error {
	// 直通：甲级赛冠军、超级赛前两名（以该选手在该榜单的最好奖项判断）。
	type directInfo struct{ reason string }
	direct := map[int64]directInfo{}
	for _, r := range rows {
		rank := model.AwardRanks[r.Result.Award]
		if rank == 0 {
			continue
		}
		switch {
		case r.EventLevel == "grade_a" && rank == 1:
			direct[r.Result.PlayerID] = directInfo{reason: "甲级赛第一名直通"}
		case r.EventLevel == "super" && rank <= 2:
			direct[r.Result.PlayerID] = directInfo{reason: "超级赛前两名直通"}
		}
	}

	existing, err := stores.Qualifications.ListValidByBoardTx(ctx, tx, boardID)
	if err != nil {
		return err
	}
	manual := map[int64]bool{}
	for _, q := range existing {
		if q.Qualification.Source == model.QualificationManual ||
			q.Qualification.Source == model.QualificationManualDirect {
			manual[q.Qualification.PlayerID] = true
		}
	}

	out := make([]model.FinalQualification, 0, len(direct)+advanceQuota)
	for pid, info := range direct {
		out = append(out, model.FinalQualification{
			SeasonID: seasonID, BoardID: boardID, PlayerID: pid,
			Source: model.QualificationAutoDirect, Reason: strPtr(info.reason),
		})
	}
	// 积分前 20：按快照排名，跳过已直通/手动资格。
	added := 0
	for _, p := range points {
		if added >= advanceQuota {
			break
		}
		if p.Rank == nil || manual[p.PlayerID] {
			continue
		}
		if _, ok := direct[p.PlayerID]; ok {
			continue
		}
		out = append(out, model.FinalQualification{
			SeasonID: seasonID, BoardID: boardID, PlayerID: p.PlayerID,
			Source: model.QualificationPoints, Reason: strPtr("积分榜前 20 晋级"),
		})
		added++
	}
	return stores.Qualifications.ReplaceAuto(ctx, tx, seasonID, boardID, out)
}

// recomputeInstitutionPoints 重算赛季机构榜：每位选手取各榜单最高积分，按机构求和。
func recomputeInstitutionPoints(ctx context.Context, stores *store.Store, tx *sql.Tx,
	seasonID int64, boards []model.Board, now time.Time) error {
	// 选手在各榜单的积分 → 取最高。
	best := map[int64]int{}
	for _, b := range boards {
		rows, err := stores.Points.ListPlayerPointsByBoardTx(ctx, tx, b.ID)
		if err != nil {
			return err
		}
		for _, r := range rows {
			if r.PlayerPoint.TotalPoints > best[r.PlayerPoint.PlayerID] {
				best[r.PlayerPoint.PlayerID] = r.PlayerPoint.TotalPoints
			}
		}
	}

	// 机构归属：取选手最近一场有效成绩的机构。
	results, err := stores.Results.ListPublishedBySeason(ctx, tx, seasonID)
	if err != nil {
		return err
	}
	institution := map[int64]string{}
	for _, r := range results {
		if r.Result.Institution != nil && *r.Result.Institution != "" {
			institution[r.Result.PlayerID] = *r.Result.Institution
		}
	}

	type agg struct {
		total   int
		players int
	}
	byInst := map[string]*agg{}
	for pid, pts := range best {
		name := institution[pid]
		if name == "" {
			continue
		}
		a := byInst[name]
		if a == nil {
			a = &agg{}
			byInst[name] = a
		}
		a.total += pts
		a.players++
	}

	out := make([]model.InstitutionPoint, 0, len(byInst))
	for name, a := range byInst {
		out = append(out, model.InstitutionPoint{
			SeasonID: seasonID, InstitutionName: name,
			TotalPoints: a.total, PlayerCount: a.players,
			ComputedAt: strPtr(utils.FormatISO(now)),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalPoints != out[j].TotalPoints {
			return out[i].TotalPoints > out[j].TotalPoints
		}
		return out[i].InstitutionName < out[j].InstitutionName
	})
	for i := range out {
		r := i + 1
		out[i].Rank = &r
	}
	return stores.Points.ReplaceInstitutionPoints(ctx, tx, seasonID, out)
}
