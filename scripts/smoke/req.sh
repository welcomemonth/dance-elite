#!/usr/bin/env bash
# 带 Bearer token 的通用请求包装（token 由 login.sh 生成）。
#
# 用法：
#   bash scripts/smoke/req.sh GET /api/v1/admin/seasons
#   bash scripts/smoke/req.sh GET "/api/v1/admin/orders?page=1&page_size=5"
#   bash scripts/smoke/req.sh POST /api/v1/admin/seasons season.json   # body 用 UTF-8 文件，规避 GBK 乱码
#   bash scripts/smoke/req.sh PUT /api/v1/admin/seasons/1 season.json
#   BASE_URL=http://localhost:8002 bash scripts/smoke/req.sh GET /api/v1/ping   # 走前端代理验证
#
# 第 3 个参数为 JSON body 文件路径（相对调用目录或绝对路径），用 --data-binary 发送。
set -euo pipefail

cd "$(dirname "$0")"
BASE="${BASE_URL:-http://localhost:8080}"
METHOD="$1"; PATH_="$2"; BODY_FILE="${3:-}"

ARGS=(-s -X "$METHOD" "$BASE$PATH_" -H "Authorization: Bearer $(cat .token)")
if [ -n "$BODY_FILE" ]; then
  ARGS+=(-H 'Content-Type: application/json; charset=utf-8' --data-binary "@$BODY_FILE")
fi

curl "${ARGS[@]}"
echo
