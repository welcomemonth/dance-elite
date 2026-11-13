#!/usr/bin/env bash
# 登录后台管理端，把 token 写进 .token 文件（与 login.sh 同目录）。
# 用法：bash scripts/smoke/login.sh
# 依赖：仓库根目录 .env（读取 ADMIN_INIT_USERNAME / ADMIN_INIT_PASSWORD）。
set -euo pipefail

cd "$(dirname "$0")"
ROOT="$(cd ../.. && pwd)"
BASE="${BASE_URL:-http://localhost:8080}"

# 从 .env 读取初始管理员账号（不 source 整个文件，避免副作用）
ADMIN_USERNAME=$(grep -E '^ADMIN_INIT_USERNAME=' "$ROOT/.env" | cut -d= -f2-)
ADMIN_PASSWORD=$(grep -E '^ADMIN_INIT_PASSWORD=' "$ROOT/.env" | cut -d= -f2-)
ADMIN_USERNAME=${ADMIN_USERNAME:-admin}

BODY=$(printf '{"username":"%s","password":"%s"}' "$ADMIN_USERNAME" "$ADMIN_PASSWORD")
RESP=$(curl -s -X POST "$BASE/api/v1/admin/auth/login" \
  -H 'Content-Type: application/json' -d "$BODY")

CODE=$(printf '%s' "$RESP" | python -c "import sys,json;print(json.load(sys.stdin)['code'])")
if [ "$CODE" != "0" ]; then
  echo "login failed: $RESP" >&2
  exit 1
fi

printf '%s' "$RESP" | python -c "import sys,json;print(json.load(sys.stdin)['data']['token'])" > .token
echo "token saved to scripts/smoke/.token (base=$BASE, user=$ADMIN_USERNAME)"
