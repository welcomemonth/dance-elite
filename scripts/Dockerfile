# =============================================================================
# dance-elite 多阶段构建
# 1. 构建前端 (dance-admin) → 产物复制到 internal/web/dist
# 2. 编译 Go 静态二进制
# 3. 最小运行时镜像（非 root）
# =============================================================================

# ---------- 1. 前端构建阶段 ----------
FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend

WORKDIR /src

# 使用国内源（可选但推荐）
RUN npm config set registry https://registry.npmmirror.com

# 安装 pnpm（避开 corepack 可能的问题）
RUN npm install -g pnpm@latest

# 只复制依赖文件，利用缓存
COPY dance-admin/package.json dance-admin/pnpm-lock.yaml* ./dance-admin/

WORKDIR /src/dance-admin

# 关键：允许依赖执行构建脚本（解决 ERR_PNPM_IGNORED_BUILDS）
RUN printf '%s\n' \
  'allowBuilds:' \
  '  esbuild: true' \
  '  "@scarf/scarf": false' \
  '  core-js: false' \
  '  core-js-pure: false' \
  '  es5-ext: false' \
  > pnpm-workspace.yaml

RUN pnpm install --frozen-lockfile

# 复制源码并构建
COPY dance-admin/ ./
COPY internal/web/ /src/internal/web/

RUN pnpm run build


# ---------- 2. Go 后端构建阶段 ----------
FROM --platform=$BUILDPLATFORM golang:1.26.2-alpine AS backend

WORKDIR /backend-build

# 安装构建依赖
RUN apk add --no-cache git ca-certificates

# 使用国内 Go 代理加速
ENV GOPROXY=https://goproxy.cn,direct

# 先复制 go.mod/go.sum，利用缓存
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# 复制全部源码
COPY . .

# 用前端阶段构建好的最新 dist 覆盖（确保嵌入的是最新前端）
COPY --from=frontend /src/internal/web/dist ./internal/web/dist

ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build \
      -trimpath \
      -ldflags="-s -w -extldflags '-static'" \
      -tags netgo,osusergo \
      -o dance-elite \
      ./cmd/dance-elite


# ---------- 3. 最终运行时镜像 ----------
FROM alpine:3.21 AS monolithic

# 安装运行时依赖 + 创建非 root 用户
RUN apk add --no-cache tzdata ca-certificates su-exec && \
    addgroup -g 10001 -S nonroot && \
    adduser -u 10001 -S -G nonroot -h /opt/dance-elite nonroot && \
    mkdir -p /opt/dance-elite /usr/local/dance-elite && \
    chown -R nonroot:nonroot /opt/dance-elite

# 复制二进制和入口脚本
COPY --from=backend /backend-build/dance-elite /usr/local/dance-elite/dance-elite
COPY --from=backend --chmod=755 /backend-build/scripts/entrypoint.sh /usr/local/dance-elite/entrypoint.sh

USER root
WORKDIR /opt/dance-elite
VOLUME /opt/dance-elite

# 非敏感配置不烘焙进镜像：运行时必须挂载 config.yaml（见 docker-compose.yml）。
ENV TZ="UTC" \
    DATA_DIR="/opt/dance-elite"

EXPOSE 8090

ENTRYPOINT ["/usr/local/dance-elite/entrypoint.sh", "/usr/local/dance-elite/dance-elite"]