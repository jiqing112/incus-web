#!/usr/bin/env bash
# IncusPanel 一键安装脚本（Linux）
#
# 从 GitHub Releases 下载最新版二进制，安装到 /usr/local/bin，
# 生成随机 API Token，并注册 systemd 服务开机自启。
#
# 用法：
#   curl -fsSL https://raw.githubusercontent.com/jiqing112/incus-web/main/scripts/install.sh | sudo bash
#   # 或指定版本 / 端口 / Token：
#   curl -fsSL .../install.sh | sudo bash -s -- v0.1.0
#   sudo PORT=9090 API_TOKEN=xxx bash install.sh
set -euo pipefail

REPO="jiqing112/incus-web"
INSTALL_DIR="/usr/local/bin"
SERVICE="incus-web"
PORT="${PORT:-8080}"

log()  { printf '\033[1;32m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[警告]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[错误]\033[0m %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "请用 root 运行：sudo bash install.sh"
command -v curl >/dev/null 2>&1 || command -v wget >/dev/null 2>&1 || die "需要 curl 或 wget 之一"

case "$(uname -m)" in
  x86_64 | amd64) ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
  *) die "不支持的架构：$(uname -m)（Release 提供 linux amd64 / arm64）" ;;
esac

fetch() { # fetch <url> <outfile>
  if command -v curl >/dev/null 2>&1; then
    curl -fSL --retry 3 --connect-timeout 15 -o "$2" "$1"
  else
    wget -q --tries=3 -O "$2" "$1"
  fi
}

# 最新 tag：跟随 releases/latest 重定向取最终 URL，避免 GitHub API 限流
latest_tag() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSI -o /dev/null -w '%{url_effective}\n' "https://github.com/${REPO}/releases/latest" |
      sed 's#.*/tag/##' | tr -d '\r' | tail -n1
  else
    wget -S --max-redirect=5 -O /dev/null "https://github.com/${REPO}/releases/latest" 2>&1 |
      grep -i '^ *[Ll]ocation:' | tail -n1 | sed 's#.*/tag/##' | tr -d '\r\n '
  fi
}

TAG="${1:-}"
if [ -z "$TAG" ]; then
  log "查询最新版本…"
  TAG="$(latest_tag)"
fi
[ -n "$TAG" ] || die "无法获取版本号，请手动指定：install.sh v0.1.0"

FILE="incus-web-${TAG}-linux-${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${FILE}"
BIN_NAME="incus-web-${TAG}-linux-${ARCH}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

log "下载 ${URL}"
fetch "$URL" "${TMP}/${FILE}"

log "解压并安装到 ${INSTALL_DIR}/incus-web"
tar -xzf "${TMP}/${FILE}" -C "$TMP"
install -m 0755 "${TMP}/${BIN_NAME}" "${INSTALL_DIR}/incus-web"

# Token：未显式提供时生成随机值，并持久化，重启不变
if [ -z "${API_TOKEN:-}" ]; then
  if [ -f /etc/default/${SERVICE} ] && grep -q '^API_TOKEN=' /etc/default/${SERVICE} 2>/dev/null; then
    API_TOKEN="$(grep '^API_TOKEN=' /etc/default/${SERVICE} | head -n1 | cut -d= -f2-)"
    log "复用已有 Token（/etc/default/${SERVICE}）"
  else
    API_TOKEN="$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')"
    log "已生成随机 API Token"
  fi
fi

mkdir -p "/var/lib/${SERVICE}"
if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
  cat > "/etc/default/${SERVICE}" <<EOF
# IncusPanel 环境配置（修改后执行 systemctl restart ${SERVICE} 生效）
PORT=${PORT}
API_TOKEN=${API_TOKEN}
# BIND_ADDR=0.0.0.0
# INCUS_SOCKET=/var/lib/incus/unix.socket
EOF

  cat > "/etc/systemd/system/${SERVICE}.service" <<EOF
[Unit]
Description=IncusPanel - Incus web panel
Documentation=https://github.com/${REPO}
After=network-online.target incus.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/incus-web
EnvironmentFile=/etc/default/${SERVICE}
Environment=DATA_DIR=/var/lib/${SERVICE}
Restart=on-failure
RestartSec=3

# 面板持有 Incus 完整控制权，收紧基础权限（保留访问 unix.socket 的能力）
NoNewPrivileges=true
ProtectHome=read-only

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable --now "${SERVICE}" >/dev/null 2>&1 || systemctl restart "${SERVICE}"
  log "systemd 服务已启动：systemctl status ${SERVICE}"
else
  warn "未检测到 systemd，请手动运行："
  warn "  sudo PORT=${PORT} API_TOKEN=${API_TOKEN} ${INSTALL_DIR}/incus-web"
fi

IP="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
log "──────────────────────────────────────────────"
log "IncusPanel ${TAG} 安装完成"
log "  面板地址  http://${IP:-<服务器IP>}:${PORT}"
log "  API Token ${API_TOKEN}"
log "  配置文件  /etc/default/${SERVICE}"
log "──────────────────────────────────────────────"
warn "面板拥有 Incus 完整控制权：请勿将端口暴露公网，必要时用 BIND_ADDR=127.0.0.1 配合反向代理。"
