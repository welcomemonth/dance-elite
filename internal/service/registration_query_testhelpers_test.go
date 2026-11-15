package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/meetmonth/dance-elite/internal/model"
)

// regQueryFixture 在报名测试装配之上挂载小程序「我的报名」查询服务（T35）。
type regQueryFixture struct {
	*regFixture
	svc     *RegistrationQueryService
	eventID int64
	seq     int
}

// newRegQueryFixture 创建含 U11 古典舞/民族民间舞、U13 古典舞的报名中赛事。
func newRegQueryFixture(t *testing.T) *regQueryFixture {
	t.Helper()
	f := newRegFixture(t)
	eventID := mustCreateRegisteringEvent(t, f, "我的报名赛事", []model.AdminEventGroupInput{
		{Level: model.GroupLevelU11, DanceType: model.DanceTypeClassical, Quota: 20},
		{Level: model.GroupLevelU11, DanceType: model.DanceTypeFolk, Quota: 20},
		{Level: model.GroupLevelU13, DanceType: model.DanceTypeClassical, Quota: 20},
	})
	return &regQueryFixture{
		regFixture: f,
		svc:        NewRegistrationQueryService(f.stores),
		eventID:    eventID,
	}
}

// regQuerySeed 是一条「我的报名」种子（选手 + 报名单 + 订单）。
type regQuerySeed struct {
	RegistrationID int64
	OrderID        int64
	PlayerID       int64
	UserID         int64
	PlayerNo       string
	OrderNo        string
}

// seed 插入一条报名与订单；userID 为 0 时新建用户。
func (f *regQueryFixture) seed(t *testing.T, userID int64, orderNo, orderStatus,
	regStatus, danceType string, groupID int64, playerName string) regQuerySeed {
	t.Helper()
	ctx := context.Background()
	if userID == 0 {
		userID = mustInsertRegUser(t, f.db, "o-reg-query-"+orderNo)
	}

	f.seq++
	playerNo := fmt.Sprintf("RQ%06d", f.seq)
	res, err := f.db.SQL().ExecContext(ctx,
		`INSERT INTO players (player_no, name, id_card_hash) VALUES (?, ?, ?)`,
		playerNo, playerName, "hash-"+playerNo)
	if err != nil {
		t.Fatalf("insert player %s: %v", playerNo, err)
	}
	playerID, _ := res.LastInsertId()

	res, err = f.db.SQL().ExecContext(ctx,
		`INSERT INTO registrations (event_id, group_id, player_id, user_id, dance_type,
			institution, status) VALUES (?, ?, ?, ?, ?, '上海灵舞艺术中心', ?)`,
		f.eventID, groupID, playerID, userID, danceType, regStatus)
	if err != nil {
		t.Fatalf("insert registration %s: %v", orderNo, err)
	}
	regID, _ := res.LastInsertId()

	var paidAt any
	if orderStatus == model.OrderStatusPaid || orderStatus == model.OrderStatusRefunding ||
		orderStatus == model.OrderStatusRefunded {
		paidAt = "2027-01-01T10:00:00+08:00"
	}
	res, err = f.db.SQL().ExecContext(ctx,
		`INSERT INTO orders (order_no, registration_id, user_id, amount, status, paid_at)
		 VALUES (?, ?, ?, 98000, ?, ?)`, orderNo, regID, userID, orderStatus, paidAt)
	if err != nil {
		t.Fatalf("insert order %s: %v", orderNo, err)
	}
	orderID, _ := res.LastInsertId()
	return regQuerySeed{
		RegistrationID: regID, OrderID: orderID, PlayerID: playerID,
		UserID: userID, PlayerNo: playerNo, OrderNo: orderNo,
	}
}

// fillMaterials 覆盖报名单 8 项资料：fields 中列出的项写入测试值，其余置 NULL。
func (f *regQueryFixture) fillMaterials(t *testing.T, regID int64, fields ...string) {
	t.Helper()
	set := make(map[string]bool, len(fields))
	for _, field := range fields {
		set[field] = true
	}
	value := func(column string) any {
		if set[column] {
			return "v-" + column
		}
		return nil
	}
	mustExecSQL(t, f.db, `UPDATE registrations SET
		photo_path = ?, audio_path = ?, program_name = ?, teacher_name = ?,
		contact_phone = ?, express_address = ?, work_intro = ?, special_req = ?,
		material_updated_at = '2027-01-02T09:00:00+08:00'
		WHERE id = ?`,
		value("photo"), value("audio"), value("program_name"), value("teacher_name"),
		value("contact_phone"), value("express_address"), value("work_intro"), value("special_req"),
		regID)
}

// setTimelinePast 把赛事整条时间线移到过去（资料截止/只读测试用）。
func (f *regQueryFixture) setTimelinePast(t *testing.T) {
	t.Helper()
	mustExecSQL(t, f.db, `UPDATE events SET
		reg_start_at = '2018-12-01T00:00:00+08:00',
		reg_end_at = '2018-12-15T00:00:00+08:00',
		lottery_at = '2019-01-01T00:00:00+08:00',
		material_check_at = '2019-02-01T00:00:00+08:00',
		start_at = '2019-03-01T00:00:00+08:00' WHERE id = ?`, f.eventID)
}

// addRefundTicket 为订单插入一张退款工单。
func (f *regQueryFixture) addRefundTicket(t *testing.T, orderID int64, ticketNo, status, reason string) {
	t.Helper()
	mustExecSQL(t, f.db, `INSERT INTO refund_tickets (ticket_no, order_id, reason, status)
		VALUES (?, ?, ?, ?)`, ticketNo, orderID, reason, status)
}

// regQueryAllMaterialFields 是资料完成度 8 项（本测试文件独立维护）。
var regQueryAllMaterialFields = []string{
	model.MaterialFieldPhoto, model.MaterialFieldAudio, model.MaterialFieldProgramName,
	model.MaterialFieldTeacherName, model.MaterialFieldContactPhone,
	model.MaterialFieldExpressAddress, model.MaterialFieldWorkIntro, model.MaterialFieldSpecialReq,
}

// withoutRegQueryField 返回去掉指定字段后的副本。
func withoutRegQueryField(fields []string, drop string) []string {
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if field != drop {
			out = append(out, field)
		}
	}
	return out
}
