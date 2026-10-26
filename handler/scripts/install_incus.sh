#!/bin/bash
# Incus 一键安装脚本（面板集成版，多发行版 + 源码编译）
# 参考 https://github.com/oneclickvirt/incus 与 https://linuxcontainers.org/incus/docs/main/installing/
#
# 用法: install_incus.sh <storage_backend> <storage_size_gb> <pool_name> <bridge_name> <log_file> [install_method]
#   install_method: auto(默认, 优先包管理器, 失败回退源码) | package(仅包管理器) | source(仅源码编译)
#
set -euo pipefail

BACKEND="${1:-btrfs}"
SIZE_GB="${2:-50}"
POOL="${3:-default}"
BRIDGE="${4:-incusbr0}"
LOG_FILE="${5:-/tmp/incus_install.log}"
METHOD="${6:-auto}"

exec > >(tee -a "$LOG_FILE") 2>&1

log() {
    local level="$1"
    shift
    echo "[$(date '+%H:%M:%S')] [$level] $*"
}

# 只读 incus 查询统一加超时。守护进程半死（或 socket 激活竞态）时 CLI 会无限等待，
# 无超时会让脚本一直挂到后端硬超时才被强杀，日志里看不到任何原因。
inc() { timeout 30 incus "$@"; }

# 无论脚本成功或中途失败，退出时都必须输出 [DONE] 行，
# 否则前端轮询永远无法结束。后端通过行尾 (rc=0) 判定成败。
INSTALL_OK=0
finish() {
    local code=$?
    if [ "$code" -eq 0 ] && [ "$INSTALL_OK" -eq 1 ]; then
        log DONE "Incus 安装/初始化完成。存储池: $POOL, 网桥: $BRIDGE (rc=0)"
    else
        log DONE "安装未完成，脚本异常退出，请检查上方 [ERROR] 日志 (rc=$code)"
    fi
}
trap finish EXIT

# ---------- 1. 操作系统检测 ----------
if [ -f /etc/os-release ]; then
    . /etc/os-release
else
    log ERROR "无法识别操作系统 (/etc/os-release 不存在)"
    exit 1
fi
log STEP "操作系统: $NAME ${VERSION_ID:-} (ID=$ID, METHOD=$METHOD)"

if [ "$(id -u)" -ne 0 ]; then
    log ERROR "必须以 root 权限运行安装脚本"
    exit 1
fi

# 发行版家族与包管理器
PKG_MGR=""
DISTRO_FAMILY="unknown"
case "$ID" in
    debian|ubuntu|linuxmint|raspbian) PKG_MGR=apt; DISTRO_FAMILY=debian ;;
    arch|archarm)                     PKG_MGR=pacman; DISTRO_FAMILY=arch ;;
    alpine)                           PKG_MGR=apk; DISTRO_FAMILY=alpine ;;
    centos|rocky|almalinux|rhel|ol|fedora) PKG_MGR=dnf; DISTRO_FAMILY=rhel ;;
    opensuse*|sles)                  PKG_MGR=zypper; DISTRO_FAMILY=suse ;;
    *)                               PKG_MGR=""; DISTRO_FAMILY=unknown ;;
esac
log STEP "发行版家族: $DISTRO_FAMILY, 包管理器: ${PKG_MGR:-无}"

HAS_SYSTEMD=0
if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
    HAS_SYSTEMD=1
fi

# ---------- 2. 文件系统支持检测 + 工具安装（按后端） ----------
# 只安装所选后端需要的工具。原实现无论选哪个后端都会 apt install zfsutils-linux，
# 等于给纯 btrfs 用户塞进一整套 ZFS 用户态（libzfs/zfs-zed 常驻守护进程）。
fs_supported() {
    local fs="$1"
    grep -qw "$fs" /proc/filesystems 2>/dev/null && return 0
    command -v modprobe >/dev/null 2>&1 && modprobe "$fs" >/dev/null 2>&1 \
        && grep -qw "$fs" /proc/filesystems 2>/dev/null
}

install_fs_tools() {
    case "$BACKEND" in
        btrfs)
            case "$DISTRO_FAMILY" in
                debian) apt-get install -y -qq btrfs-progs >/dev/null 2>&1 || true ;;
                arch)   pacman -S --noconfirm --needed btrfs-progs >/dev/null 2>&1 || true ;;
                alpine) apk add --no-cache btrfs-progs >/dev/null 2>&1 || true ;;
                rhel)   dnf install -y btrfs-progs >/dev/null 2>&1 || true ;;
                suse)   zypper --non-interactive install btrfsprogs >/dev/null 2>&1 || true ;;
            esac
            ;;
        zfs)
            case "$DISTRO_FAMILY" in
                debian) apt-get install -y -qq zfsutils-linux >/dev/null 2>&1 || true ;;
                arch)   pacman -S --noconfirm --needed zfs-utils >/dev/null 2>&1 || true ;;
                alpine) apk add --no-cache zfs >/dev/null 2>&1 || true ;;
                rhel)   dnf install -y zfs >/dev/null 2>&1 || true ;;
                suse)   zypper --non-interactive install zfs >/dev/null 2>&1 || true ;;
            esac
            ;;
        *) : ;;   # dir 后端不需要额外工具
    esac
}

# 后端可用性预检：RHEL/CentOS 系内核默认不含 btrfs 模块，
# 直接按 btrfs 建池会失败并白白重试多轮，提前降级为 dir 更直观。
check_backend_support() {
    case "$BACKEND" in
        btrfs|zfs)
            if ! fs_supported "$BACKEND"; then
                log WARN "当前内核不支持 $BACKEND 文件系统（/proc/filesystems 无此项，modprobe 亦失败）"
                log WARN "自动改用 dir 后端（功能完整，仅快照为全量拷贝）"
                BACKEND=dir
            fi
            ;;
    esac
}

# ---------- 3. 包管理器安装 ----------
# Debian/Ubuntu 把 CLI 拆到 incus-client：只装 incus 时守护进程在、incus 命令却没有，
# 后续所有 incus 调用失败会被误报成「守护进程 120 秒未就绪」，非常难排查。
ensure_incus_cli_debian() {
    command -v incus >/dev/null 2>&1 && return 0
    log WARN "incus 命令缺失，补装 incus-client..."
    if ! out=$(timeout 300 apt-get install -y incus-client 2>&1); then
        log WARN "incus-client 安装失败：$(echo "$out" | tail -2 | tr '\n' ' ')"
    fi
    command -v incus >/dev/null 2>&1
}

install_via_package() {
    case "$DISTRO_FAMILY" in
        debian)
            # 所有包管理器操作都加 timeout：apt 等锁/慢镜像会无限挂起，
            # 挂住后只能靠后端 45 分钟硬超时强杀，日志里看不到任何原因
            timeout 300 apt-get update -qq >/dev/null 2>&1 || true
            # 捕获输出：原实现 2>/dev/null 把失败原因全吞了，日志里只剩「安装失败」无从排查
            if out=$(timeout 600 apt-get install -y incus 2>&1); then
                log OK "通过系统源安装 incus 成功"
                ensure_incus_cli_debian || log WARN "系统源未提供 incus CLI，后续初始化可能失败"
                return 0
            fi
            log WARN "系统源安装失败：$(echo "$out" | tail -2 | tr '\n' ' ')"
            # zabbly 源需要 curl 与 gpg 才能导入签名；缺失时 apt 会以 NO_PUBKEY 失败
            timeout 300 apt-get install -y -qq ca-certificates curl gnupg >/dev/null 2>&1 || true
            CODENAME="${VERSION_CODENAME:-${UBUNTU_CODENAME:-}}"
            if [ -z "$CODENAME" ]; then
                log ERROR "无法确定发行版代号（VERSION_CODENAME/UBUNTU_CODENAME 均为空），无法配置 zabbly 源"
                return 1
            fi
            log WARN "添加 zabbly 官方源（$CODENAME）后重试..."
            mkdir -p /etc/apt/keyrings
            curl -fsSL --connect-timeout 15 --max-time 60 https://pkgs.zabbly.com/key.asc | gpg --batch --yes --dearmor -o /etc/apt/keyrings/zabbly.gpg 2>/dev/null || true
            cat >/etc/apt/sources.list.d/zabbly-incus.list <<EOF
deb [signed-by=/etc/apt/keyrings/zabbly.gpg] https://pkgs.zabbly.com/incus/stable $CODENAME main
EOF
            timeout 300 apt-get update -qq >/dev/null 2>&1 || true
            if out=$(timeout 600 apt-get install -y incus incus-client incus-tools 2>&1) \
                || out=$(timeout 600 apt-get install -y incus incus-client 2>&1) \
                || out=$(timeout 600 apt-get install -y incus 2>&1); then
                log OK "通过 zabbly 源安装 incus 成功"
                ensure_incus_cli_debian || log WARN "zabbly 源未提供 incus CLI，后续初始化可能失败"
                return 0
            fi
            log ERROR "zabbly 源安装失败：$(echo "$out" | tail -3 | tr '\n' ' ')"
            return 1
            ;;
        arch)
            # -Sy 单独装包会造成部分升级（新包依赖新版 glibc 而系统未升级），必须 -Syu
            if out=$(timeout 600 pacman -Syu --noconfirm incus 2>&1); then
                log OK "pacman 安装 incus 成功"
                return 0
            fi
            log ERROR "pacman 安装失败：$(echo "$out" | tail -3 | tr '\n' ' ')"
            return 1
            ;;
        alpine)
            # 确保 community 仓库已启用
            sed -i 's/^#\(.*\/community\)/\1/' /etc/apk/repositories 2>/dev/null || true
            timeout 300 apk update >/dev/null 2>&1 || true
            if out=$(timeout 600 apk add incus incus-client 2>&1) || out=$(timeout 600 apk add incus 2>&1); then
                log OK "apk 安装 incus 成功"
                return 0
            fi
            log ERROR "apk 安装失败：$(echo "$out" | tail -3 | tr '\n' ' ')"
            return 1
            ;;
        rhel)
            # 1) EPEL 9+ 自带 incus，是最省事的来源。
            #    注意：epel-release 已安装但被禁用时，install epel-release 是空操作，
            #    epel 仍然 disabled → incus 依赖的 lxc 解析不到 → 报「uninstallable packages」。
            #    必须显式 set-enabled，或安装时带 --enablerepo=epel。
            timeout 300 dnf install -y dnf-plugins-core >/dev/null 2>&1 || true
            timeout 300 dnf install -y epel-release >/dev/null 2>&1 || true
            dnf config-manager --set-enabled epel >/dev/null 2>&1 || true

            dnf_ok=0
            for try in 1 2 3; do
                if out=$(timeout 600 dnf install -y --enablerepo=epel incus incus-client incus-tools 2>&1); then
                    dnf_ok=1; break
                fi
                log INFO "dnf(EPEL) 安装 incus 第 $try/3 次失败：$(echo "$out" | tail -2 | tr '\n' ' ')"
                sleep 3
            done
            [ "$dnf_ok" = 1 ] && { log OK "dnf(EPEL) 安装 incus 成功"; return 0; }

            # incus-tools 在部分源里没有，缺它不应让整个安装失败
            if out=$(timeout 600 dnf install -y --enablerepo=epel incus incus-client 2>&1); then
                log OK "dnf(EPEL) 安装 incus 成功（incus-tools 不可用，已跳过）"
                return 0
            fi
            log WARN "EPEL 源安装失败：$(echo "$out" | tail -2 | tr '\n' ' ')"

            # 2) RHEL/CentOS/Rocky/Alma 系经 COPR neelc/incus 安装
            #    必须用 dnf copr enable（copr 插件按当前发行版自动选择 chroot），
            #    不能硬编码 rhel+epel-<ver> 的 repo URL（该 chroot 在 COPR 上不存在，会 404）
            if ! out=$(timeout 180 dnf copr enable -y neelc/incus 2>&1); then
                log ERROR "启用 COPR neelc/incus 失败：$(echo "$out" | tail -2 | tr '\n' ' ')"
                return 1
            fi
            if out=$(timeout 600 dnf install -y --enablerepo=epel incus incus-client 2>&1); then
                log OK "dnf(COPR) 安装 incus 成功"
                return 0
            fi
            log ERROR "dnf 安装失败：$(echo "$out" | tail -3 | tr '\n' ' ')"
            return 1
            ;;
        suse)
            if out=$(timeout 600 zypper --non-interactive install incus 2>&1); then
                log OK "zypper 安装 incus 成功"
                return 0
            fi
            log ERROR "zypper 安装失败：$(echo "$out" | tail -3 | tr '\n' ' ')"
            return 1
            ;;
        *)
            log ERROR "未知发行版（ID=$ID），无法通过包管理器安装"
            return 1
            ;;
    esac
}

# ---------- 4. 源码编译安装 ----------
# debian 在受限网络下默认 deb.debian.org 可能慢/不可达，优先切换为国内镜像（best-effort）
switch_apt_mirror() {
    [ "$DISTRO_FAMILY" = debian ] || return 0
    command -v apt-get >/dev/null 2>&1 || return 0
    local codename
    codename=$( ( . /etc/os-release 2>/dev/null; echo "${VERSION_CODENAME:-}" ) )
    [ -n "$codename" ] || codename=trixie
    local m="https://mirrors.tuna.tsinghua.edu.cn/debian"
    local ms="https://mirrors.tuna.tsinghua.edu.cn/debian-security"
    # 海外机器切到国内镜像只会更慢：先探测可达性，不通就保持默认源
    if ! curl -4fsS --connect-timeout 5 --max-time 10 -o /dev/null "$m/dists/" 2>/dev/null; then
        log WARN "清华镜像不可达，保持默认 apt 源"
        return 0
    fi
    log INFO "debian: 尝试切换 apt 镜像为 $m"
    # 覆盖 sources.list 前先留一份原文件，便于回滚
    [ -f /etc/apt/sources.list ] && cp -f /etc/apt/sources.list /etc/apt/sources.list.bak-incus 2>/dev/null || true
    # Debian 12+ 默认用 deb822 格式的 /etc/apt/sources.list.d/debian.sources，
    # 只覆盖 /etc/apt/sources.list 不会停用它 → 必须一并挪走，否则默认源仍在生效
    if [ -f /etc/apt/sources.list.d/debian.sources ]; then
        mv -f /etc/apt/sources.list.d/debian.sources /etc/apt/sources.list.d/debian.sources.bak-incus 2>/dev/null || true
    fi
    cat >/etc/apt/sources.list <<EOF
deb $m $codename main contrib non-free non-free-firmware
deb $m ${codename}-updates main contrib non-free non-free-firmware
deb $m ${codename}-backports main contrib non-free non-free-firmware
deb $ms ${codename}-security main contrib non-free non-free-firmware
EOF
    if timeout 120 apt-get update -qq >/dev/null 2>&1; then
        log OK "apt 镜像切换成功"
    else
        log WARN "apt 镜像切换失败，回退默认源"
        cat >/etc/apt/sources.list <<EOF
deb http://deb.debian.org/debian $codename main contrib non-free non-free-firmware
deb http://deb.debian.org/debian ${codename}-updates main contrib non-free non-free-firmware
deb http://security.debian.org/debian-security ${codename}-security main contrib non-free non-free-firmware
EOF
        timeout 120 apt-get update -qq >/dev/null 2>&1 || true
    fi
}

install_via_source() {
    # 禁止 go 自动下载 toolchain（避免受限网络下静默超时卡死）；Go 版本由 ensure_go 保证
    export GOTOOLCHAIN=local
    switch_apt_mirror
    log STEP "源码编译安装 Incus（需要 Go 工具链与构建依赖）..."

    # 4.1 安装/升级 Go
    ensure_go || { log ERROR "Go 工具链不可用，源码编译中止"; exit 1; }

    # 4.2 构建依赖（每个发行版包名不同）
    # 注意：除了编译期依赖，还必须安装 incusd 运行期依赖（setfattr/ip/newuidmap 等），
    # 否则编译通过但守护进程启动会立即崩溃（已在 debian/13 实测：setfattr、ip 缺失）。
    log STEP "安装构建依赖与运行期依赖..."
    case "$DISTRO_FAMILY" in
        debian)
            timeout 300 apt-get update -qq >/dev/null 2>&1 || true
            # 核心构建工具（git/编译链）必须就位，先独立安装，避免单包缺失导致整组回滚
            timeout 420 apt-get install -y -qq build-essential git curl jq pkg-config autoconf automake libtool \
                gettext rsync xz-utils squashfs-tools dnsmasq-base nftables >/dev/null 2>&1 || true
            # lxc 开发库：包名随 Debian 版本变化（trixie=lxc-dev, bookworm=liblxc-dev）
            LXC_DEV="lxc-dev"; apt-cache show lxc-dev >/dev/null 2>&1 || LXC_DEV="liblxc-dev"
            timeout 300 apt-get install -y -qq "$LXC_DEV" libacl1-dev libcap-dev libseccomp-dev libudev-dev \
                libuv1-dev liblz4-dev libnftnl-dev libtirpc-dev libsqlite3-dev >/dev/null 2>&1 || true
            # 运行期依赖（incusd 启动必需）：setfattr(attr) / ip(iproute2) / newuidmap(uidmap) / iptables
            timeout 300 apt-get install -y -qq attr iproute2 uidmap iptables >/dev/null 2>&1 || true
            ;;
        arch)
            pacman -S --noconfirm --needed base-devel git curl jq \
                lxc acl libcap libseccomp libuv liblz4 pkgconf libtool autoconf automake \
                gettext rsync xz squashfs-tools dnsmasq nftables \
                iproute2 attr iptables shadow >/dev/null 2>&1 || true
            ;;
        alpine)
            apk add --no-cache build-base git curl jq \
                lxc-dev acl-dev libcap-dev libseccomp-dev libuv-dev liblz4-dev \
                libtool gettext-dev eudev-dev sqlite-dev autoconf automake \
                rsync xz squashfs-tools dnsmasq nftables \
                attr iproute2 iptables shadow-uidmap >/dev/null 2>&1 || true
            ;;
        rhel)
            dnf install -y dnf-plugins-core >/dev/null 2>&1 || true
            dnf config-manager --enable crb >/dev/null 2>&1 || true
            # RHEL 9 系官方仓库已移除 lxc（Red Hat 改推 podman），先装其他编译依赖
            dnf install -y gcc gcc-c++ git curl jq \
                libcap-devel libseccomp-devel libacl-devel libudev-devel libuv-devel \
                lz4-devel libnftnl-devel libtirpc-devel sqlite-devel autoconf automake libtool \
                pkgconfig gettext rsync xz squashfs-tools dnsmasq nftables \
                attr iproute2 iptables shadow-utils >/dev/null 2>&1 || true
            # 检测 lxc 开发库是否可获得；RHEL/CentOS 9 系不含 lxc，源码编译不可行，需早失败并提示
            if ! rpm -q lxc-devel >/dev/null 2>&1; then
                if ! dnf list available --enablerepo=crb lxc-devel >/dev/null 2>&1; then
                    log ERROR "本发行版（RHEL/CentOS 9 系）官方仓库不含 lxc 开发库（lxc-devel），源码编译 Incus 不可行"
                    log ERROR "请改用「仅包管理器」方式安装（脚本会自动启用 COPR neelc/incus，已包含 lxc 依赖）"
                    exit 1
                fi
                dnf install -y lxc-devel >/dev/null 2>&1 || true
            fi
            ;;
        suse)
            zypper --non-interactive install -y gcc gcc-c++ git curl jq \
                liblxc-devel libcap-devel libseccomp-devel libacl-devel libudev-devel libuv-devel \
                liblz4-devel sqlite3-devel libtool pkg-config gettext tcl rsync xz \
                squashfs dnsmasq nftables \
                attr iproute2 iptables >/dev/null 2>&1 || true
            ;;
    esac

    # 运行期依赖自检：setfattr / ip / newuidmap 缺失会直接导致 incusd 启动崩溃，提前报错更直观
    for bin in setfattr ip newuidmap; do
        command -v "$bin" >/dev/null 2>&1 || log WARN "运行期依赖缺失: $bin（incusd 可能无法启动，请检查上面依赖安装是否成功）"
    done

    # 确保 git 可用（克隆源码必需）
    if ! command -v git >/dev/null 2>&1; then
        log STEP "安装 git（克隆源码必需）..."
        case "$DISTRO_FAMILY" in
            debian) apt-get update -qq >/dev/null 2>&1 || true; apt-get install -y -qq git >/dev/null 2>&1 || true ;;
            arch)   pacman -Sy --noconfirm --needed git >/dev/null 2>&1 || true ;;
            alpine) apk add --no-cache git >/dev/null 2>&1 || true ;;
            rhel)   dnf install -y git >/dev/null 2>&1 || true ;;
            suse)   zypper --non-interactive install git >/dev/null 2>&1 || true ;;
        esac
    fi
    command -v git >/dev/null 2>&1 || { log ERROR "git 未安装，无法克隆源码"; exit 1; }

    # 4.3 获取最新稳定版 tag 并克隆源码
    # 受限网络下 github 直连可能不稳定，克隆加重试（失败清理后重试）
    SRC_DIR=/tmp/incus-src
    rm -rf "$SRC_DIR"
    LATEST_TAG=$(timeout 120 git ls-remote --tags --sort=-v:refname https://github.com/lxc/incus 2>/dev/null \
        | grep -oE 'refs/tags/(v[0-9]+\.[0-9]+(\.[0-9]+)?)$' | head -1 | sed 's#refs/tags/##')
    LATEST_TAG="${LATEST_TAG:-v6.0.4}"
    log STEP "克隆 Incus 源码 (tag=$LATEST_TAG)..."
    clone_ok=0
    for attempt in 1 2 3; do
        log INFO "克隆尝试 $attempt/3 ..."
        if timeout 900 git clone --depth 1 --branch "$LATEST_TAG" https://github.com/lxc/incus "$SRC_DIR" >/tmp/clone.log 2>&1; then
            clone_ok=1; break
        fi
        tail -3 /tmp/clone.log
        rm -rf "$SRC_DIR"; sleep 5
    done
    [ "$clone_ok" = 1 ] || { log ERROR "克隆 Incus 源码失败（网络不可达？）"; exit 1; }
    cd "$SRC_DIR" || { log ERROR "进入源码目录失败"; exit 1; }

    # 4.4 构建依赖库（cowsql/raft）—— make deps 内部会 git clone github 子依赖，受限网络易中断，加重试
    log STEP "make deps（编译 cowsql/raft 依赖库，约需 1-3 分钟）..."
    mkdir -p "$(go env GOPATH)/bin"
    # 受限网络下 github 直连常因 TLS 中断（GnuTLS recv error -110）失败，
    # 通过切换到 HTTP/1.1、增大 postBuffer 提升克隆成功率
    git config --global http.version HTTP/1.1 2>/dev/null || true
    git config --global http.postBuffer 524288000 2>/dev/null || true
    deps_ok=0
    for attempt in 1 2 3 4 5 6; do
        log INFO "make deps 尝试 $attempt/6 ..."
        if timeout 1200 make deps >/tmp/incus_make_deps.log 2>&1; then
            deps_ok=1; break
        fi
        tail -5 /tmp/incus_make_deps.log
        log WARN "make deps 第 $attempt 次失败，清理依赖目录后重试..."
        rm -rf "$(go env GOPATH)/deps"
        sleep 10
    done
    [ "$deps_ok" = 1 ] || { log ERROR "make deps 失败"; exit 1; }
    # 提取 make deps 结尾打印的环境变量并导出（若存在）
    ENV_LINES=$(grep -E '^[[:space:]]*#?[[:space:]]*export ' /tmp/incus_make_deps.log | sed -E 's/^[[:space:]]*#+[[:space:]]*//')
    [ -n "$ENV_LINES" ] && eval "$ENV_LINES" || true

    # 4.5 编译
    log STEP "make（编译 incusd / incus，约需 2-5 分钟）..."
    timeout 2400 make 2>&1 | tail -25 || { log ERROR "make 失败（或编译超时）"; exit 1; }

    # 4.6 安装二进制到 /usr/local/bin
    BIN_SRC="$(go env GOPATH)/bin"
    if [ -x "$BIN_SRC/incusd" ]; then
        install -m 0755 "$BIN_SRC"/incusd /usr/local/bin/incusd
        install -m 0755 "$BIN_SRC"/incus  /usr/local/bin/incus
    elif [ -x ./bin/incusd ]; then
        install -m 0755 ./bin/incusd /usr/local/bin/incusd
        install -m 0755 ./bin/incus  /usr/local/bin/incus
    else
        log ERROR "编译产物缺失（incusd 未生成），源码编译失败"
        exit 1
    fi
    # 面板可能由 systemd 以精简 PATH 拉起（不含 /usr/local/bin），
    # 不补 PATH 会导致「刚装好却说 incus 命令不可用」
    export PATH="/usr/local/bin:$PATH"
    hash -r 2>/dev/null || true
    log OK "二进制已安装: $(incus --version 2>/dev/null || echo unknown)"

    # 4.7 持久化运行时环境变量（incusd 需要 cowsql/raft 动态库）
    mkdir -p /etc/default
    {
        echo "# 由 incus-web 源码安装生成"
        echo "LD_LIBRARY_PATH=$(go env GOPATH)/deps/cowsql/.libs/:$(go env GOPATH)/deps/raft/.libs/:${LD_LIBRARY_PATH:-}"
        # 注意：必须写死完整 PATH（含 sbin 目录），不能用 PATH=\$PATH:... 。
        # 实测 debian/13 容器内 systemd 继承的 PATH 不含 /usr/sbin，\$PATH 展开后
        # incusd 找不到 /usr/sbin/ip 而崩溃。
        echo "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:$(go env GOPATH)/bin"
    } > /etc/default/incus-env

    # 4.8 subuid/subgid 与 incus-admin 组
    groupadd -f incus-admin
    grep -q '^root:1000000:' /etc/subuid  || echo "root:1000000:1000000000" >> /etc/subuid
    grep -q '^root:1000000:' /etc/subgid  || echo "root:1000000:1000000000" >> /etc/subgid

    # 4.9 服务托管单元（源码构建无包内 unit，需按 init 系统自行生成）
    if [ "$HAS_SYSTEMD" -eq 1 ]; then
        cat >/etc/systemd/system/incus.service <<'EOF'
[Unit]
Description=Incus - Linux system containers
Documentation=man:incusd(1)
After=network-online.target
Wants=network-online.target

[Service]
Type=notify
EnvironmentFile=-/etc/default/incus-env
ExecStart=/usr/local/bin/incusd
ExecStartPost=/usr/local/bin/incusd waitready --timeout=600
KillMode=process
Restart=on-failure
RestartSec=2s
LimitNOFILE=1048576
LimitNPROC=infinity
TasksMax=infinity
Delegate=yes
TasksAccounting=false

[Install]
WantedBy=multi-user.target
EOF
        systemctl daemon-reload >/dev/null 2>&1 || true
        systemctl enable incus >/dev/null 2>&1 || true
    elif command -v rc-update >/dev/null 2>&1 || [ "$DISTRO_FAMILY" = alpine ]; then
        # OpenRC（Alpine 等）：生成 init 脚本并加入默认运行级，使源码安装的 incusd 可被托管
        cat >/etc/init.d/incusd <<'EOF'
#!/sbin/openrc-run
name="Incus daemon"
description="Incus - Linux system containers"
command="/usr/local/bin/incusd"
command_args=""
pidfile="/run/incusd.pid"
command_background="yes"
output_log="/var/log/incusd.log"
error_log="/var/log/incusd.log"

depend() {
    # 用 use 而非 need：嵌套容器/精简系统里可能没有 net 服务，need 会导致无法启动
    use net
    after firewall
    after localmount
}

start_pre() {
    # 注入运行期环境：LD_LIBRARY_PATH（cowsql/raft 动态库）与 PATH（setfattr/ip 等）
    if [ -f /etc/default/incus-env ]; then
        . /etc/default/incus-env
    fi
    mkdir -p /var/log
}
EOF
        chmod +x /etc/init.d/incusd
        rc-update add incusd default >/dev/null 2>&1 || true
        log OK "已生成 OpenRC 脚本 /etc/init.d/incusd 并加入默认运行级"
    else
        log WARN "非 systemd/OpenRC 系统，源码安装后将无法自动托管服务（建议改用包安装）"
    fi

    log OK "源码编译安装完成"
}

# 检查当前 go 命令是否存在且版本 >= 1.26（Incus 主干对 Go 版本要求逐年提高，宁高勿低）
go_version_ok() {
    command -v go >/dev/null 2>&1 || return 1
    local have
    have=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | tr -d 'go')
    [ -z "$have" ] && return 1
    local maj min
    maj=$(echo "$have" | cut -d. -f1)
    min=$(echo "$have" | cut -d. -f2)
    if [ "${maj:-0}" -gt 1 ]; then return 0; fi
    if [ "${maj:-0}" -eq 1 ] && [ "${min:-0}" -ge 26 ]; then return 0; fi
    return 1
}

# 确保 Go 版本足够新：优先系统包管理器，缺失或过旧再下载官方二进制（强制 IPv4）
ensure_go() {
    if go_version_ok; then
        log OK "Go 版本满足要求: $(go version | awk '{print $3}')"
        return 0
    fi
    log STEP "系统 Go 不满足或缺失，尝试通过包管理器安装（限时 180s，超时/失败则改用官方镜像）..."
    case "$DISTRO_FAMILY" in
        debian) timeout 180 apt-get update -qq >/dev/null 2>&1 || true; timeout 180 apt-get install -y -qq golang-go >/dev/null 2>&1 || true ;;
        arch)   timeout 180 pacman -Sy --noconfirm --needed go >/dev/null 2>&1 || true ;;
        alpine) timeout 180 apk update >/dev/null 2>&1 || true; timeout 180 apk add --no-cache go >/dev/null 2>&1 || true ;;
        rhel)   timeout 180 dnf install -y golang >/dev/null 2>&1 || true ;;
        suse)   timeout 180 zypper --non-interactive refresh >/dev/null 2>&1 || true; timeout 180 zypper --non-interactive install go >/dev/null 2>&1 || true ;;
    esac
    if go_version_ok; then
        log OK "包管理器安装的 Go: $(go version | awk '{print $3}')"
        return 0
    fi
    log STEP "下载官方 Go 二进制（强制 IPv4，优先国内镜像）..."
    # 精简系统常没有 curl / ca-certificates，缺失时任何下载都必然失败，先补齐再下载
    if ! command -v curl >/dev/null 2>&1; then
        case "$DISTRO_FAMILY" in
            debian) timeout 180 apt-get install -y -qq ca-certificates curl >/dev/null 2>&1 || true ;;
            arch)   timeout 180 pacman -Sy --noconfirm --needed curl >/dev/null 2>&1 || true ;;
            alpine) timeout 180 apk add --no-cache curl ca-certificates >/dev/null 2>&1 || true ;;
            rhel)   timeout 180 dnf install -y curl ca-certificates >/dev/null 2>&1 || true ;;
            suse)   timeout 180 zypper --non-interactive install curl ca-certificates >/dev/null 2>&1 || true ;;
        esac
    fi
    command -v curl >/dev/null 2>&1 || { log ERROR "curl 不可用且自动安装失败，无法下载 Go"; return 1; }

    # 官方 Go 按 CPU 架构分发，硬编码 amd64 会让 arm64（树莓派/Graviton/大量廉价 VPS）
    # 解出无法执行的二进制，表现为「Go 安装后仍不可用」而无从排查。
    local goarch
    case "$(uname -m)" in
        x86_64|amd64)   goarch=amd64 ;;
        aarch64|arm64)  goarch=arm64 ;;
        armv7l|armv6l)  goarch=armv6l ;;
        i386|i686)      goarch=386 ;;
        ppc64le)        goarch=ppc64le ;;
        s390x)          goarch=s390x ;;
        riscv64)        goarch=riscv64 ;;
        *) log ERROR "不支持的 CPU 架构 $(uname -m)，无法下载官方 Go 二进制"; return 1 ;;
    esac

    local ver
    ver=$(curl -4s --connect-timeout 15 --max-time 30 https://go.dev/dl/?mode=json | grep -m1 '"version"' | grep -oE 'go[0-9.]+' | head -1)
    ver="${ver:-go1.26.0}"
    export GOPROXY="https://goproxy.cn,direct"
    local ok=0 url
    for base in "https://golang.google.cn/dl" "https://mirrors.aliyun.com/golang" "https://go.dev/dl"; do
        url="$base/${ver}.linux-${goarch}.tar.gz"
        log INFO "尝试从镜像下载 $ver ($goarch): $base"
        if curl -4fsSL --connect-timeout 20 --max-time 420 "$url" -o /tmp/go.tar.gz 2>/dev/null; then ok=1; break; fi
    done
    [ "$ok" = 1 ] || { log ERROR "下载 Go 失败（已尝试 golang.google.cn / aliyun / go.dev 多个镜像）"; return 1; }
    rm -rf /usr/local/go
    if ! tar -C /usr/local -xzf /tmp/go.tar.gz; then
        rm -f /tmp/go.tar.gz
        log ERROR "解压 Go 失败（磁盘空间不足？）"
        return 1
    fi
    rm -f /tmp/go.tar.gz
    export PATH="/usr/local/go/bin:$PATH"
    export GOPATH="/root/go"
    if go_version_ok; then
        log OK "官方 Go 已安装: $(/usr/local/go/bin/go version | awk '{print $3}')"
        return 0
    fi
    log ERROR "Go 安装后仍不可用"
    return 1
}

# ---------- 5. 执行安装 ----------
if command -v incus >/dev/null 2>&1; then
    VERSION=$(incus --version 2>/dev/null || echo "unknown")
    log INFO "incus 已安装 (版本 $VERSION)，跳过安装步骤"
else
    log STEP "开始安装 incus (方式=$METHOD)..."
    case "$METHOD" in
        package)
            install_via_package || { log ERROR "包安装失败，请改用源码编译方式重试"; exit 1; }
            ;;
        source)
            install_via_source || { log ERROR "源码编译安装失败"; exit 1; }
            ;;
        auto|*)
            if install_via_package; then
                :
            else
                log WARN "包管理器安装失败，回退到源码编译..."
                install_via_source || { log ERROR "源码编译安装也失败"; exit 1; }
            fi
            ;;
    esac
fi

# 后续建池/建网桥/配 profile 全部依赖 incus CLI。缺失时若不在此处早失败，
# 会一路走到「等待守护进程就绪」循环并报「120 秒未就绪」，掩盖真正原因。
if ! command -v incus >/dev/null 2>&1; then
    log ERROR "安装流程结束但 incus 命令不可用（PATH=$PATH），请检查上方包管理器日志"
    exit 1
fi

# ---------- 5.5 运行时前提：idmap ----------
# 包管理器安装不会替你写 /etc/subuid、/etc/subgid。
# 缺失时 incusd 报 "System doesn't have a functional idmap setup"，
# 容器一个都起不来（CentOS Stream 9 实测），而安装脚本却报成功。
# 源码编译路径原来有这段，包安装路径没有 —— 这里统一补齐。
ensure_idmap() {
    groupadd -f incus-admin 2>/dev/null || true
    local need=0
    grep -q '^root:' /etc/subuid 2>/dev/null || need=1
    grep -q '^root:' /etc/subgid 2>/dev/null || need=1
    if [ "$need" -eq 1 ]; then
        log STEP "配置 idmap（/etc/subuid、/etc/subgid），容器运行必需..."
        case "$DISTRO_FAMILY" in
            debian) apt-get install -y -qq uidmap >/dev/null 2>&1 || true ;;
            arch)   pacman -S --noconfirm --needed shadow >/dev/null 2>&1 || true ;;
            alpine) apk add --no-cache shadow-uidmap >/dev/null 2>&1 || true ;;
            rhel)   dnf install -y shadow-utils-subid >/dev/null 2>&1 || true ;;
            suse)   zypper --non-interactive install shadow >/dev/null 2>&1 || true ;;
        esac
        grep -q '^root:' /etc/subuid 2>/dev/null || echo "root:1000000:1000000000" >> /etc/subuid
        grep -q '^root:' /etc/subgid 2>/dev/null || echo "root:1000000:1000000000" >> /etc/subgid
    fi
    if ! command -v newuidmap >/dev/null 2>&1 || ! command -v newgidmap >/dev/null 2>&1; then
        log WARN "newuidmap/newgidmap 缺失，容器可能无法启动（请安装 uidmap 或 shadow-utils-subid）"
    else
        log OK "idmap 已就绪（$(grep '^root:' /etc/subuid 2>/dev/null | head -1)）"
    fi
}

# ---------- 6. 启动服务 ----------
ensure_idmap
log STEP "检查 incus 服务..."
if [ "$HAS_SYSTEMD" -eq 1 ]; then
    # 只在服务未运行时启动。对已运行的服务做 restart 会让 incusd 停掉所有运行中实例，
    # 实例较多时还会长时间卡在 stop 阶段（实测 Debian 13 有运行中容器时 >5 分钟未退出），
    # 期间面板一直处于「安装中」，用户体验极差且会打断业务。
    if systemctl is-active --quiet incus; then
        log INFO "incus 服务已在运行，跳过重启（避免中断正在运行的实例）"
    else
        systemctl start incus >/dev/null 2>&1 || systemctl start incusd >/dev/null 2>&1 || true
        systemctl enable incus >/dev/null 2>&1 || true
    fi
elif command -v rc-service >/dev/null 2>&1; then
    if rc-service incusd status >/dev/null 2>&1; then
        log INFO "incusd 已在运行，跳过重启（避免中断正在运行的实例）"
    else
        rc-update add incusd >/dev/null 2>&1 || true
        rc-service incusd start >/dev/null 2>&1 || true
    fi
fi

# ---------- 7. 等待守护进程就绪 ----------
log STEP "等待 incus 守护进程就绪..."
INCUS_SOCKET=""
for i in $(seq 1 60); do
    if inc info >/dev/null 2>&1; then
        # 探测实际 socket 路径：不同发行版/安装方式位置不同
        #   Debian/Arch/源码：/var/lib/incus/unix.socket
        #   Fedora/Rocky 官方包：/run/incus/unix.socket
        for s in /var/lib/incus/unix.socket /run/incus/unix.socket; do
            [ -S "$s" ] && { INCUS_SOCKET="$s"; break; }
        done
        # 兜底：在 /run/incus 与 /var/lib/incus 下找任意 .socket
        # 注意必须 || true：find 对不存在的目录返回 1，pipefail 下会静默杀死整个脚本
        if [ -z "$INCUS_SOCKET" ]; then
            INCUS_SOCKET=$(find /run/incus /var/lib/incus -maxdepth 2 -name '*.socket' 2>/dev/null | head -1 || true)
        fi
        if [ -n "$INCUS_SOCKET" ]; then
            log OK "守护进程已就绪（socket: $INCUS_SOCKET）"
        else
            INCUS_SOCKET=/var/lib/incus/unix.socket
            log WARN "守护进程已就绪，但未探测到 socket 文件，API 建池按默认路径重试"
        fi
        break
    fi
    sleep 2
done
if ! inc info >/dev/null 2>&1; then
    log ERROR "incus 守护进程在 120 秒内未就绪，请检查服务日志 (journalctl -u incus 或 rc-service incusd status)"
    exit 1
fi

# ---------- 7.5 可选能力：QEMU（虚拟机） ----------
# 面板的「创建实例」支持容器与虚拟机两种类型；没有 QEMU 时虚拟机创建会失败，
# 这里提前说明，避免用户在界面上点了虚拟机才看到难懂的报错。
if command -v qemu-system-x86_64 >/dev/null 2>&1 || command -v qemu-system-aarch64 >/dev/null 2>&1; then
    log OK "QEMU 可用，支持创建虚拟机"
else
    log WARN "未检测到 QEMU：容器可正常使用，但无法创建虚拟机"
    log WARN "如需虚拟机：Debian/Ubuntu 执行 apt install qemu-system-x86；RHEL 系执行 dnf install qemu-kvm"
fi

# ---------- 8. 初始化状态检查（不做 admin init） ----------
# 注意：不能在这里执行 `incus admin init --auto`。它会先建一个 dir 驱动的 default 池，
# 把用户选的 btrfs/zfs 后端直接顶掉。真正的初始化在第 9 步按用户选择建池，
# 只有连池都建不出来时才回落到 admin init（见 9.1）。
# 另外 `incus storage list | grep -q .` 永远为真（表格有表头），
# 必须用 --format csv（无表头，无池时为空）判断。
if [ -n "$(inc storage list --format csv 2>/dev/null)" ]; then
    log INFO "存储池已就绪，跳过初始化"
else
    log INFO "尚未创建存储池，稍后按所选后端创建"
fi

# ---------- 9. 创建存储池 ----------
log STEP "准备存储后端（$BACKEND）..."
check_backend_support
log INFO "安装文件系统工具（后端=$BACKEND）..."
install_fs_tools
create_pool_api() {
    local pool_name="$1" driver="$2" size_gb="$3" sock="$4"
    python3 - "$pool_name" "$driver" "$size_gb" "$sock" <<'PY'
import sys, http.client, json, socket
sock_path = sys.argv[4]
driver = sys.argv[2]
config = {}
# size 仅对 btrfs/zfs/lvm 有效（dir 后端不支持该 config，会返回 500 Invalid option "size"）
if driver in ("btrfs", "zfs", "lvm", "ceph"):
    config["size"] = sys.argv[3] + "GiB"
class UConn(http.client.HTTPConnection):
    def __init__(self):
        # 父类要求 host 参数；我们完全覆盖 connect()，故只需给个占位 host
        super().__init__("localhost")
    def connect(self):
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.settimeout(30)
        self.sock.connect(sock_path)
c = UConn()
body = json.dumps({"name": sys.argv[1], "driver": driver, "config": config})
c.request("POST", "/1.0/storage-pools", body=body, headers={"Content-Type": "application/json"})
r = c.getresponse()
print("STATUS", r.status)
print(r.read().decode()[:300])
PY
}

if ! inc storage show "$POOL" >/dev/null 2>&1; then
    log STEP "创建存储池 $POOL (后端=$BACKEND, 大小=${SIZE_GB}GB)..."
    api_ok=false
    if command -v python3 >/dev/null 2>&1; then
        # socket 可能因 socket 激活存在瞬时竞态：先检测 socket 就位，再重试 API 建池，
        # 命中即可保留 size 配置（比 CLI 快且不依赖 CLI 是否卡死）
        pool_api_try() {
            local try
            for try in 1 2 3 4 5; do
                if [ -S "$INCUS_SOCKET" ] && create_pool_api "$POOL" "$BACKEND" "$SIZE_GB" "$INCUS_SOCKET" | grep -q "STATUS 201"; then
                    return 0
                fi
                log INFO "API 建池未成功（socket/激活可能尚未就绪），重试 $try/5 ..."
                sleep 2
            done
            return 1
        }
        if pool_api_try; then
            api_ok=true
            log OK "存储池通过 API 创建成功（size=${SIZE_GB}GiB）"
        fi
    fi
    if [ "$api_ok" = false ]; then
        log WARN "API 创建失败，尝试 CLI（60s 超时）..."
        if timeout 60 incus storage create "$POOL" "$BACKEND" size="${SIZE_GB}GiB" >/dev/null 2>&1; then
            log OK "存储池通过 CLI 创建成功（size=${SIZE_GB}GiB）"
        elif timeout 60 incus storage create "$POOL" "$BACKEND" >/dev/null 2>&1; then
            log OK "存储池通过 CLI 创建成功（无 size 配置，dir 等不支持 size 的后端）"
        else
            log WARN "$BACKEND 创建失败，回退到 dir 驱动"
            if timeout 60 incus storage create "$POOL" dir >/dev/null 2>&1; then
                log OK "已回退创建 dir 存储池 $POOL"
            else
                # 原来这里无论成败都打 OK，导致「没有存储池也报安装成功」
                log ERROR "dir 存储池创建同样失败，请检查 incus 服务日志（journalctl -u incus）"
                exit 1
            fi
        fi
    fi
else
    log INFO "存储池 $POOL 已存在，跳过"
fi

# ---------- 9.1 兜底初始化 ----------
# 走到这里仍无任何存储池，说明按所选后端建池彻底失败（极少见）。
# 此时用 admin init --auto 建一个可用的默认环境，至少保证面板能创建实例。
if [ -z "$(inc storage list --format csv 2>/dev/null)" ]; then
    log WARN "未创建出任何存储池，执行 incus admin init --auto 兜底初始化..."
    if ! timeout 120 incus admin init --auto >/dev/null 2>&1; then
        log ERROR "incus admin init 兜底也失败，请检查 incus 服务日志（journalctl -u incus）"
        exit 1
    fi
    log OK "兜底初始化完成（dir 存储池 default + incusbr0）"
fi

# ---------- 10. 创建网桥 ----------
# 注意：这里若 incus network create 失败且无超时，set -e 会静默退出（只留下 rc=1 无报错），
# 且该机器实测 CLI 创建类操作可能挂起，故加 timeout + 显式失败处理
if ! inc network show "$BRIDGE" >/dev/null 2>&1; then
    log STEP "创建网桥 $BRIDGE..."
    if timeout 60 incus network create "$BRIDGE" >/dev/null 2>&1; then
        log OK "网桥 $BRIDGE 创建成功"
    else
        log WARN "网桥 $BRIDGE 创建失败（可能缺少网络权限），后续创建容器可能无法联网"
    fi
else
    log INFO "网桥 $BRIDGE 已存在，跳过"
fi

# ---------- 10.5 将存储池与网桥接入默认 profile ----------
# 否则新建容器没有 root 磁盘 / 网卡，无法启动。仅当对应设备不存在时才添加，避免重复报错。
# 设备已存在时还要比对实际指向：面板里选的池/网桥与 profile 不一致时若不提示，
# 用户会误以为新建实例落在本次选择的池里。
HAS_DISK=0
inc profile device show default 2>/dev/null | grep -q "type: disk" && HAS_DISK=1
CUR_POOL=$(inc profile device get default root pool 2>/dev/null || true)
if [ "$HAS_DISK" -eq 0 ]; then
    # path=/ 是 profile 磁盘设备的必填项，缺了会报
    # "Disk entry is missing the required \"source\" or \"path\" property"，
    # 导致默认 profile 没有 root 磁盘、新建实例全部失败（原实现就缺这一项）
    if timeout 60 incus profile device add default root disk pool="$POOL" path=/ >/dev/null 2>&1; then
        log OK "默认 profile 已挂载存储池 $POOL 作为 root 磁盘"
    else
        log WARN "默认 profile 挂载存储池 $POOL 失败，新建实例可能因缺少 root 磁盘而无法启动"
    fi
elif [ -n "$CUR_POOL" ] && [ "$CUR_POOL" != "$POOL" ]; then
    log WARN "默认 profile 的 root 磁盘已指向存储池 $CUR_POOL，本次选择的 $POOL 不会自动用于新实例"
    log WARN "如需切换：incus profile device set default root pool=$POOL"
elif [ -n "$CUR_POOL" ]; then
    log INFO "默认 profile 已使用存储池 $CUR_POOL"
fi

HAS_NIC=0
inc profile device show default 2>/dev/null | grep -q "type: nic" && HAS_NIC=1
CUR_NET=$(inc profile device get default eth0 network 2>/dev/null || true)
if [ "$HAS_NIC" -eq 0 ]; then
    if timeout 60 incus profile device add default eth0 nic name=eth0 network="$BRIDGE" >/dev/null 2>&1; then
        log OK "默认 profile 已接入网桥 $BRIDGE 作为 eth0 网卡"
    else
        log WARN "默认 profile 接入网桥 $BRIDGE 失败，新建实例可能无法联网"
    fi
elif [ -n "$CUR_NET" ] && [ "$CUR_NET" != "$BRIDGE" ]; then
    log WARN "默认 profile 的 eth0 已使用网桥 $CUR_NET，本次选择的 $BRIDGE 不会自动用于新实例"
elif [ -n "$CUR_NET" ]; then
    log INFO "默认 profile 已使用网桥 $CUR_NET"
fi

# ---------- 11. 安装后验证 ----------
# 验证镜像优先取本地：`incus launch images:alpine/3.21` 会强制走远程，
# 即便本地已有同名镜像也要重新下载；受限网络下 300s 都下不完（实测 3.7kB/s），
# 且 timeout 只杀掉 CLI，服务端仍在建实例，留下孤儿实例与未结束的操作。
log STEP "安装后验证（创建临时容器并打快照）..."
TEST_NAME="incus-install-verify-$(date +%s)"
VERIFY_SRC=""
VERIFY_IMG_CREATED=0
if inc image info alpine/3.21 >/dev/null 2>&1; then
    VERIFY_SRC="alpine/3.21"
    log INFO "使用本地已有镜像 alpine/3.21 验证"
else
    log INFO "本地无 alpine/3.21，尝试下载验证镜像（限时 240s）..."
    if timeout 240 incus image copy images:alpine/3.21 local: --alias incus-verify-img >/dev/null 2>&1; then
        VERIFY_SRC="incus-verify-img"
        VERIFY_IMG_CREATED=1
    else
        log WARN "验证镜像下载超时或失败（网络受限），跳过安装后验证（不影响安装结果）"
    fi
fi

if [ -n "$VERIFY_SRC" ]; then
    # 镜像已在本地，创建实例本身很快，120s 足够
    if timeout 120 incus launch "$VERIFY_SRC" "$TEST_NAME" --ephemeral >/dev/null 2>&1; then
        sleep 4
        if timeout 60 incus snapshot create "$TEST_NAME" verify-snap >/dev/null 2>&1; then
            log OK "快照验证通过"
        else
            log WARN "快照创建验证失败（实例可创建，快照不可用）"
        fi
        timeout 60 incus delete "$TEST_NAME" --force >/dev/null 2>&1 || true
    else
        log WARN "验证容器创建失败（嵌套/网络限制），安装本身可能已完成，请手动检查"
        # CLI 被杀不代表服务端停下，主动清理半成品实例
        timeout 60 incus delete "$TEST_NAME" --force >/dev/null 2>&1 || true
    fi
fi
if [ "$VERIFY_IMG_CREATED" -eq 1 ]; then
    timeout 60 incus image delete incus-verify-img >/dev/null 2>&1 || true
fi

INSTALL_OK=1
