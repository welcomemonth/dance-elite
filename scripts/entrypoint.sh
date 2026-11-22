#!/bin/sh

set -eu

# 密钥与微信账户通过容器运行时环境注入（仓库根目录 .env.example 为模板），
# 镜像与仓库中不存放任何真实密钥。非敏感配置由挂载的 config.yaml 提供
# （CONFIG_FILE 指向它）。后端启动自检失败时 fail-fast，并列出缺失项。
#
# 默认数据目录（可用环境变量覆盖）
DATA_DIR="${DATA_DIR:-/opt/dance-elite}"
export DATA_DIR

# 确保 SQLite 数据目录存在，并赋予 nonroot 用户读写权限
mkdir -p "${DATA_DIR}"
chown -R nonroot:nonroot "${DATA_DIR}"

# 降权到 nonroot 运行应用
exec su-exec nonroot "$@"
