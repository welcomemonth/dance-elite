#!/bin/sh
set -eu

cd "$(dirname "$0")/.."

# ---------- 先构建前端 ----------
echo "→ Building frontend (dance-admin)..."
if [ -d "dance-admin" ]; then
  (cd dance-admin && pnpm run build)
else
  echo "Warning: dance-admin 目录不存在，跳过前端构建"
fi

# ---------- 再构建 Go 二进制 ----------
OS=$(uname -s)

case "$OS" in
  *CYGWIN*|*MINGW*|*MSYS*)
    OUTPUT="./build/dance-elite.exe"
    ;;
  *)
    OUTPUT="./build/dance-elite"
    ;;
esac

echo "→ Building for $OS..."

mkdir -p ./build/.gocache ./build/.gomodcache
export GOCACHE="$(pwd)/build/.gocache"
export GOMODCACHE="$(pwd)/build/.gomodcache"

go build -o "$OUTPUT" ./cmd/dance-elite

echo "✓ Build successful: $OUTPUT"
echo ""
echo "Run with:"
echo "  PORT=8090 DEBUG=true $OUTPUT"
echo "（密钥通过环境变量注入，模板见 .env.example）"