#!/usr/bin/env bash
# 后台运营链路端到端冒烟（S3-2 自动化部分）。
#
# 覆盖：后台登录 → 建赛季并激活 → 建赛事并发布 → 人工报名（出生日期）
#       → 报名名单/导出 → 人工退赛与名额释放 → 客服工单 → 看板 → 退款受理
#       → 鉴权/路径穿越/回调验签 负向校验。
#
# 用法（服务需已启动；APP_ENV=dev 时支付/退款走 mock）：
#   BASE_URL=http://127.0.0.1:18099 bash scripts/smoke/e2e.sh
#
# 依赖：curl、jq；后台账号密码取自仓库根 config.yaml 与 .env。
# 注意：会向目标库写入测试数据，请勿直连生产库。
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BASE="${BASE_URL:-http://localhost:8080}"
CONFIG_FILE="${CONFIG_FILE:-$ROOT/config.yaml}"

ADMIN_USERNAME=$(sed -n 's/^[[:space:]]*init_username:[[:space:]]*//p' "$CONFIG_FILE" | head -n1 | tr -d ' "')
ADMIN_PASSWORD=$(grep -E '^ADMIN_INIT_PASSWORD=' "$ROOT/.env" | cut -d= -f2-)
ADMIN_USERNAME=${ADMIN_USERNAME:-admin}

PASS=0
FAIL=0
ok() { PASS=$((PASS + 1)); printf '  \033[32mPASS\033[0m %s\n' "$1"; }
bad() { FAIL=$((FAIL + 1)); printf '  \033[31mFAIL\033[0m %s\n' "$1"; }
step() { printf '\n\033[1m%s\033[0m\n' "$1"; }

# call METHOD PATH [BODY] -> prints body, uses $TOKEN when set
call() {
  local method="$1" path="$2" body="${3:-}"
  local args=(-s -X "$method" "$BASE$path" -H 'Content-Type: application/json')
  [ -n "${TOKEN:-}" ] && args+=(-H "Authorization: Bearer $TOKEN")
  [ -n "$body" ] && args+=(--data-binary "$body")
  curl "${args[@]}"
}

code_of() { printf '%s' "$1" | jq -r '.code // "empty"'; }
eq() { [ "$1" = "$2" ]; }

SUFFIX=$(date +%s)

step "0. 健康检查"
RESP=$(call GET /api/v1/ping)
[ "$(code_of "$RESP")" = "0" ] && ok "ping" || bad "ping: $RESP"

step "1. 后台登录"
RESP=$(call POST /api/v1/admin/auth/login "{\"username\":\"$ADMIN_USERNAME\",\"password\":\"$ADMIN_PASSWORD\"}")
TOKEN=$(printf '%s' "$RESP" | jq -r '.data.token // empty')
[ -n "$TOKEN" ] && ok "login" || { bad "login: $RESP"; exit 1; }

step "2. 创建并激活赛季"
RESP=$(call POST /api/v1/admin/seasons "{\"name\":\"E2E 赛季 $SUFFIX\",\"start_date\":\"2026-01-01\",\"end_date\":\"2027-12-31\"}")
SEASON_ID=$(printf '%s' "$RESP" | jq -r '.data.id // empty')
[ -n "$SEASON_ID" ] && ok "create season ($SEASON_ID)" || bad "create season: $RESP"
RESP=$(call PUT "/api/v1/admin/seasons/$SEASON_ID" "{\"name\":\"E2E 赛季 $SUFFIX\",\"start_date\":\"2026-01-01\",\"end_date\":\"2027-12-31\",\"status\":\"active\"}")
[ "$(printf '%s' "$RESP" | jq -r '.data.status // empty')" = "active" ] && ok "activate season" || bad "activate season: $RESP"

step "3. 创建并发布赛事"
EVENT_BODY=$(cat <<JSON
{"name":"E2E 赛事 $SUFFIX","season_id":$SEASON_ID,"level":"grade_a","city":"上海","venue":"E2E 场馆",
 "event_date":"2027-03-15","fee":98000,
 "timeline":{"reg_start_at":"2026-01-01T00:00:00+08:00","reg_end_at":"2027-02-01T18:00:00+08:00",
   "lottery_at":"2027-02-10T10:00:00+08:00","material_check_at":"2027-03-01T18:00:00+08:00",
   "start_at":"2027-03-15T09:00:00+08:00"},
 "groups":[{"level":"U13","dance_type":"classical","quota":2},{"level":"U13","dance_type":"folk","quota":2}],
 "publish":true}
JSON
)
RESP=$(call POST /api/v1/admin/events "$EVENT_BODY")
EVENT_ID=$(printf '%s' "$RESP" | jq -r '.data.event_id // empty')
[ -n "$EVENT_ID" ] && ok "create+ publish event ($EVENT_ID)" || bad "create event: $RESP"
RESP=$(call GET "/api/v1/admin/events/$EVENT_ID")
GROUP_ID=$(printf '%s' "$RESP" | jq -r '.data.groups[] | select(.dance_type=="classical") | .group_id' | head -n1)
[ -n "$GROUP_ID" ] && ok "load event groups ($GROUP_ID)" || bad "load event groups: $RESP"

step "4. 人工报名（含出生日期）"
MANUAL_BODY=$(cat <<JSON
{"event_id":$EVENT_ID,"group_id":$GROUP_ID,"player_name":"E2E 选手","id_type":"passport",
 "id_card_no":"E2E-$SUFFIX","birth_date":"2014-05-01","contact_phone":"13800000001",
 "charge_type":"offline_paid","amount":98000}
JSON
)
RESP=$(call POST /api/v1/admin/manual-registrations "$MANUAL_BODY")
ORDER_NO=$(printf '%s' "$RESP" | jq -r '.data.order_no // empty')
REG_ID=$(printf '%s' "$RESP" | jq -r '.data.registration_id // empty')
[ -n "$ORDER_NO" ] && ok "manual registration ($ORDER_NO)" || bad "manual registration: $RESP"

RESP=$(call POST /api/v1/admin/manual-registrations \
  "{\"event_id\":$EVENT_ID,\"group_id\":$GROUP_ID,\"player_name\":\"缺生日\",\"id_type\":\"passport\",\"id_card_no\":\"NOBD-$SUFFIX\",\"contact_phone\":\"13800000002\",\"charge_type\":\"free\"}")
[ "$(code_of "$RESP")" = "40001" ] && ok "missing birth_date rejected (40001)" || bad "missing birth_date: $RESP"

step "5. 报名名单与导出"
RESP=$(call GET "/api/v1/admin/registrations?event_id=$EVENT_ID")
TOTAL=$(printf '%s' "$RESP" | jq -r '.data.total // -1')
[ "$TOTAL" = "1" ] && ok "registration list total=1" || bad "registration list total=$TOTAL"
[ "$(printf '%s' "$RESP" | jq -r '.data.list[0].material_total // 0')" = "8" ] && ok "material_total=8" || bad "material_total"

STATUS=$(curl -s -o /tmp/e2e-export.xlsx -w '%{http_code}' "$BASE/api/v1/admin/events/$EVENT_ID/registrations/export" \
  -H "Authorization: Bearer $TOKEN")
if [ "$STATUS" = "200" ] && [ "$(head -c2 /tmp/e2e-export.xlsx)" = "PK" ]; then
  ok "export xlsx"
else
  bad "export xlsx (status=$STATUS)"
fi

step "6. 人工退赛与名额释放"
RESP=$(call POST "/api/v1/admin/registrations/$REG_ID/withdraw" '{"reason":"E2E 退赛"}')
[ "$(printf '%s' "$RESP" | jq -r '.data.released // false')" = "true" ] && ok "withdraw released slot" || bad "withdraw: $RESP"
RESP=$(call GET "/api/v1/admin/events/$EVENT_ID")
TAKEN=$(printf '%s' "$RESP" | jq -r ".data.groups[] | select(.group_id==$GROUP_ID) | .registered")
[ "$TAKEN" = "0" ] && ok "group registered back to 0" || bad "group registered=$TAKEN"
RESP=$(call GET "/api/v1/admin/registrations?event_id=$EVENT_ID")
[ "$(printf '%s' "$RESP" | jq -r '.data.total')" = "0" ] && ok "active list empty after withdraw" || bad "active list after withdraw"

step "7. 客服工单与看板"
RESP=$(call POST /api/v1/admin/cs-tickets \
  "{\"type\":\"other\",\"user_name\":\"E2E\",\"phone\":\"13800000009\",\"content\":\"E2E 工单\"}")
[ -n "$(printf '%s' "$RESP" | jq -r '.data.ticket_id // empty')" ] && ok "create cs ticket" || bad "create cs ticket: $RESP"
RESP=$(call GET "/api/v1/admin/cs-tickets?keyword=E2E")
[ "$(printf '%s' "$RESP" | jq -r '.data.total')" -ge 1 ] && ok "cs ticket keyword filter" || bad "cs ticket keyword: $RESP"
RESP=$(call GET /api/v1/admin/dashboard)
[ -n "$(printf '%s' "$RESP" | jq -r '.data.metrics.today_registrations // empty')" ] && ok "dashboard" || bad "dashboard: $RESP"

step "8. 退款受理（mock 通道）"
RESP=$(call POST /api/v1/admin/refund-tickets "{\"order_no\":\"$ORDER_NO\",\"reason\":\"E2E 退费\",\"applicant\":\"phone\"}")
TICKET_ID=$(printf '%s' "$RESP" | jq -r '.data.ticket_id // empty')
if [ -n "$TICKET_ID" ]; then
  ok "create refund ticket ($TICKET_ID)"
  TICKET_NO=$(printf '%s' "$RESP" | jq -r '.data.ticket_no // empty')
  AMOUNT=$(call GET "/api/v1/admin/refund-tickets/$TICKET_ID" | jq -r '.data.suggest_amount // empty')
  RESP=$(call POST "/api/v1/admin/refund-tickets/$TICKET_ID/approve" "{\"amount\":$AMOUNT}")
  [ "$(printf '%s' "$RESP" | jq -r '.data.status // empty')" = "approved" ] && ok "approve refund (mock)" || bad "approve refund: $RESP"

  # mock 通道的退款回调接受明文 JSON（生产为验签 + 解密）。
  NOTIFY=$(cat <<JSON
{"mchid":"1900000109","out_trade_no":"$ORDER_NO","transaction_id":"E2E-TX-$SUFFIX",
 "out_refund_no":"$TICKET_NO","refund_id":"E2E-RF-$SUFFIX","refund_status":"SUCCESS",
 "success_time":"2027-01-01T11:00:00+08:00","amount":{"total":98000,"refund":98000,"payer_total":98000,"payer_refund":98000}}
JSON
)
  RESP=$(curl -s -X POST "$BASE/api/v1/payments/wechat/refund-notify" \
    -H 'Content-Type: application/json' --data-binary "$NOTIFY")
  RESP_ORDER=$(call GET "/api/v1/admin/orders/$ORDER_NO")
  ORDER_STATUS=$(printf '%s' "$RESP_ORDER" | jq -r '.data.order.status // empty')
  [ "$ORDER_STATUS" = "refunded" ] && ok "refund notify closes order (refunded)" || bad "refund notify: order=$ORDER_STATUS body=$RESP"
else
  # 订单已退赛但仍是 paid，正常应可登记；失败时打印原因便于定位。
  bad "create refund ticket: $RESP"
fi

step "9. 负向校验"
RESP=$(curl -s "$BASE/api/v1/admin/orders")
[ "$(code_of "$RESP")" = "40101" ] && ok "admin api requires token (40101)" || bad "admin api without token: $RESP"
STATUS=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/static/materials/../../config.yaml")
[ "$STATUS" != "200" ] && ok "static path traversal blocked ($STATUS)" || bad "static path traversal"
STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/v1/payments/wechat/notify" \
  -H 'Content-Type: application/json' -d '{"foo":1}')
[ "$STATUS" = "401" ] && ok "wechat notify rejects bad signature (401)" || bad "wechat notify bad signature (status=$STATUS)"

step "10. 榜单：成绩发布与小程序只读"
RANK_EVENT=$(cat <<JSON
{"name":"E2E 榜单赛事 $SUFFIX","season_id":$SEASON_ID,"level":"grade_a","city":"上海","venue":"E2E 榜单馆",
 "event_date":"2027-04-15","fee":98000,
 "timeline":{"reg_start_at":"2026-01-01T00:00:00+08:00","reg_end_at":"2027-03-01T18:00:00+08:00",
   "lottery_at":"2027-03-10T10:00:00+08:00","material_check_at":"2027-03-20T18:00:00+08:00",
   "start_at":"2027-04-15T09:00:00+08:00"},
 "groups":[{"level":"U13","dance_type":"classical","quota":5}],"publish":true}
JSON
)
RESP=$(call POST /api/v1/admin/events "$RANK_EVENT")
RANK_EVENT_ID=$(printf '%s' "$RESP" | jq -r '.data.event_id // empty')
RESP=$(call GET "/api/v1/admin/events/$RANK_EVENT_ID")
RANK_GROUP_ID=$(printf '%s' "$RESP" | jq -r '.data.groups[0].group_id')
for n in "榜单一号" "榜单二号"; do
  call POST /api/v1/admin/manual-registrations "{\"event_id\":$RANK_EVENT_ID,\"group_id\":$RANK_GROUP_ID,\"player_name\":\"$n\",\"id_type\":\"passport\",\"id_card_no\":\"$n-$SUFFIX\",\"birth_date\":\"2014-05-01\",\"contact_phone\":\"13800000011\",\"charge_type\":\"free\"}" >/dev/null
done

PREVIEW=$(call POST "/api/v1/admin/events/$RANK_EVENT_ID/scores/preview?level=U13&dance_type=classical" \
  '{"rows":[{"row_no":1,"player_name":"榜单一号","award":"champion"},{"row_no":2,"player_name":"榜单二号","award":"runner_up"}]}')
[ "$(printf '%s' "$PREVIEW" | jq -r '.data.matched')" = "2" ] && ok "score preview matched" || bad "score preview: $PREVIEW"
PUBLISH_BODY=$(printf '%s' "$PREVIEW" | jq -c '{rows: [.data.rows[] | {row_no,player_name,award,advanced_to_final,match_status,player_id,remark}]}')
RESP=$(call POST "/api/v1/admin/events/$RANK_EVENT_ID/scores?level=U13&dance_type=classical" "$PUBLISH_BODY")
[ "$(printf '%s' "$RESP" | jq -r '.data.published')" = "2" ] && ok "score publish" || bad "score publish: $RESP"

RESP=$(curl -s "$BASE/api/v1/config")
[ "$(printf '%s' "$RESP" | jq -r '.data.advance_quota')" = "20" ] && ok "public config" || bad "public config: $RESP"
RESP=$(curl -s "$BASE/api/v1/ranking?level=U13&dance_type=classical")
RANK_ROWS=$(printf '%s' "$RESP" | jq -r '.data.ranking | length')
[ "$RANK_ROWS" = "2" ] && ok "public ranking rows=2" || bad "public ranking: $RESP"
ENTRY_ID=$(printf '%s' "$RESP" | jq -r '.data.ranking[0].id')
RESP=$(curl -s "$BASE/api/v1/dancers/$ENTRY_ID")
[ "$(printf '%s' "$RESP" | jq -r '.data.total_points')" -gt 0 ] && ok "dancer detail" || bad "dancer detail: $RESP"
RESP=$(curl -s "$BASE/api/v1/institution/ranking")
[ "$(printf '%s' "$RESP" | jq -r '.data.institutions | length')" -ge 0 ] && ok "institution ranking" || bad "institution ranking: $RESP"
RESP=$(call GET "/api/v1/admin/prize-statements?season_id=$SEASON_ID")
[ "$(printf '%s' "$RESP" | jq -r '.data.list | length')" -ge 1 ] && ok "prize statements" || bad "prize statements: $RESP"

step "11. S5：公开内容与导出"
RESP=$(curl -s "$BASE/api/v1/contents/announcement")
[ "$(printf '%s' "$RESP" | jq -r '.data.key')" = "announcement" ] && ok "public contents" || bad "public contents: $RESP"
RESP=$(curl -s "$BASE/api/v1/contents/not_a_key")
[ "$(code_of "$RESP")" = "40401" ] && ok "unknown content key rejected" || bad "unknown content key: $RESP"

META=$(curl -s -o /dev/null -w '%{http_code} %{content_type}' -H "Authorization: Bearer $TOKEN" \
  "$BASE/api/v1/admin/orders/export?event_id=$EVENT_ID")
[ "${META%% *}" = "200" ] && printf '%s' "$META" | grep -q 'spreadsheetml' && ok "order export xlsx" || bad "order export: $META"
META=$(curl -s -o /dev/null -w '%{http_code} %{content_type}' -H "Authorization: Bearer $TOKEN" \
  "$BASE/api/v1/admin/waitlists/export?event_id=$EVENT_ID")
[ "${META%% *}" = "200" ] && printf '%s' "$META" | grep -q 'spreadsheetml' && ok "waitlist export xlsx" || bad "waitlist export: $META"
META=$(curl -s -o /dev/null -w '%{http_code} %{content_type}' -H "Authorization: Bearer $TOKEN" \
  "$BASE/api/v1/admin/materials/missing/export?event_id=$EVENT_ID")
[ "${META%% *}" = "200" ] && printf '%s' "$META" | grep -q 'spreadsheetml' && ok "material export xlsx" || bad "material export: $META"
META=$(curl -s -o /dev/null -w '%{http_code} %{content_type}' -H "Authorization: Bearer $TOKEN" \
  "$BASE/api/v1/admin/events/$EVENT_ID/materials/archive")
[ "${META%% *}" = "200" ] && printf '%s' "$META" | grep -q 'application/zip' && ok "material archive zip" || bad "material archive: $META"

printf '\n\033[1mSummary\033[0m: %d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
