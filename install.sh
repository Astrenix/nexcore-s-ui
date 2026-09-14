#!/bin/bash
# nexcore-s-ui · install (fresh install only — for upgrades use update.sh
# or `nexcore-s-ui update`).
#
# 用法:
#   bash <(curl -Ls https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/install.sh)
#   bash <(curl -Ls https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/install.sh) v1.0.0
#   bash <(curl -Ls https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/install.sh) --force
#   bash <(curl -Ls https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/install.sh) --help
#
# 默认:
#   - 端口随机生成(10000-60000,且检测端口未被占用)
#   - 面板路径随机生成(16 字符 url-safe slug)
#   - 启用 --secure-entry 时 slug 升级为 32 字符
#   - 用 --fixed 回退到老默认 3095 / /app/
#
# 与上游 alireza0/s-ui 完全独立,可在同一台机器共存:
#   - 安装目录    /usr/local/nexcore-s-ui/      (上游是 /usr/local/s-ui/)
#   - systemd     nexcore-s-ui.service          (上游是 s-ui.service)
#   - CLI 命令    /usr/bin/nexcore-s-ui         (上游是 /usr/bin/s-ui)
#   - 数据库      /usr/local/nexcore-s-ui/db/nexcore-s-ui.db
#                                               (上游是 /usr/local/s-ui/db/s-ui.db)
#
# 可覆盖默认值的环境变量:
#   GH_OWNER  GH_REPO  INSTALL_DIR  PKG_PREFIX  CMD_NAME  SERVICE_NAME
#   PANEL_PORT  PANEL_PATH  SECURE_ENTRY  SECURE_ENTRY_PATH  USE_FIXED_DEFAULT
#
# 首装全自动随机凭据 + 随机端口 + 随机 path,装完立即 echo 出全部信息。

set -eo pipefail

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
blue='\033[0;34m'
cyan='\033[0;36m'
plain='\033[0m'

CMD_NAME="${CMD_NAME:-nexcore-s-ui}"
SERVICE_NAME="${SERVICE_NAME:-${CMD_NAME}}"
GH_OWNER="${GH_OWNER:-DoBestone}"
GH_REPO="${GH_REPO:-nexcore-s-ui}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/${CMD_NAME}}"
PKG_PREFIX="${PKG_PREFIX:-nexcore-s-ui}"   # tarball 解压顶层目录名,与 release 包结构一致
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
DB_PATH="${INSTALL_DIR}/db/${CMD_NAME}.db"

FORCE=false
TARGET_VERSION=""
# 安装期可定制的面板首装参数。空值 = 走"安全随机"路径(见 apply_initial_settings)。
# CLI 参数和 ENV 变量等价,CLI 优先(ENV 是 fallback)。
PANEL_PORT="${PANEL_PORT:-}"
PANEL_PATH="${PANEL_PATH:-}"
# --secure-entry:打开 = path 用更长 slug(32 字符)+ 显式 echo 警告"务必记录"。
# --secure-entry=mySlug:用指定 slug,跳过随机化(自定义安全入口)。
SECURE_ENTRY="${SECURE_ENTRY:-}"
SECURE_ENTRY_PATH="${SECURE_ENTRY_PATH:-}"
# --fixed:用老默认值(3095 / /app/),给"我习惯老地址"的运维一个回退。
# 等同 PANEL_PORT=3095 PANEL_PATH=/app/ 的语法糖。
USE_FIXED_DEFAULT="${USE_FIXED_DEFAULT:-}"

# 安装信息回调 — 给主控/云厂商用的"装完把凭据 POST 到 webhook"机制。
# 安全设计:
#   - URL 走 CLI / ENV 都行(URL 不算秘密)
#   - REPORT_KEY 强制 ENV(走 CLI 会进 /proc/<pid>/cmdline,泄露给 ps)
#   - HMAC-SHA256(body, REPORT_KEY) → X-NexCore-Signature 头
#   - 默认 HTTPS,--report-allow-http 才放明文(测试用)
#   - 单次 POST + 10s 超时,失败 warn 不 die — 凭据已经在终端 / journal 里
# 用法:
#   REPORT_KEY=secret bash <(curl ... install.sh) --report-url=https://x/cb
#   REPORT_URL=https://... REPORT_KEY=secret bash <(curl ... install.sh)
REPORT_URL_RAW="${REPORT_URL:-}"
REPORT_KEY_RAW="${REPORT_KEY:-}"
REPORT_ALLOW_HTTP="${REPORT_ALLOW_HTTP:-}"
REPORT_KEY_VIA_CLI=0

show_help() {
    # 用 printf -- "%b" 让 ANSI escape 真正生效;heredoc 在 cat 下是字面字符。
    printf -- "%b\n" \
"${green}nexcore-s-ui · 一键安装${plain}

${cyan}用法:${plain}
  bash <(curl -Ls https://raw.githubusercontent.com/${GH_OWNER}/${GH_REPO}/main/install.sh) [VERSION] [OPTIONS]

${cyan}VERSION${plain} (可选): vX.Y.Z 指定版本,缺省用最新 release。

${cyan}OPTIONS:${plain}
  --port=N             指定面板端口(缺省随机 10000-60000 内未占端口)
  --path=/xxx/         指定面板路径(缺省随机 16 字符 slug)
  --secure-entry       启用安全入口(随机 path 升到 32 字符,更难暴力扫)
  --secure-entry=xxx   启用安全入口 + 指定 slug
  --fixed              用老默认值(端口 3095 / 路径 /app/),回退兼容
  --force, -f          强制重装(会覆盖 ${INSTALL_DIR},保留 db/)
  --help, -h           显示本帮助

${cyan}回调(主控自动装节点):${plain}
  --report-url=URL     装完 HMAC 签名 POST 凭据到此 URL(主控接收 webhook)
  --report-allow-http  允许明文 http://(默认必须 https,测试用)
  REPORT_KEY=secret    HMAC 签名 KEY(必填,只能 ENV 传入,CLI 会泄露)

${cyan}ENV(等价 CLI):${plain}
  PANEL_PORT=N         同 --port=N
  PANEL_PATH=/xxx/     同 --path=/xxx/
  SECURE_ENTRY=1       同 --secure-entry
  SECURE_ENTRY_PATH=x  同 --secure-entry=x
  USE_FIXED_DEFAULT=1  同 --fixed
  REPORT_URL=URL       同 --report-url=URL
  REPORT_ALLOW_HTTP=1  同 --report-allow-http
  GH_OWNER GH_REPO INSTALL_DIR PKG_PREFIX CMD_NAME SERVICE_NAME

${cyan}例子:${plain}
  # 全部随机(端口 + path,首装最安全)
  bash <(curl -Ls .../install.sh)

  # 指定端口,path 随机
  bash <(curl -Ls .../install.sh) --port=33095

  # 长 32 字符安全入口(给公网生产环境)
  bash <(curl -Ls .../install.sh) --secure-entry

  # 老默认 3095 + /app/(我熟悉这个就用这个)
  bash <(curl -Ls .../install.sh) --fixed

  # 升级到指定版本 + 强制重装
  bash <(curl -Ls .../install.sh) v1.5.2 --force"
}

for arg in "$@"; do
    case "$arg" in
        --force|-f)              FORCE=true ;;
        --help|-h)               show_help; exit 0 ;;
        --port=*)                PANEL_PORT="${arg#*=}" ;;
        --path=*)                PANEL_PATH="${arg#*=}" ;;
        --secure-entry)          SECURE_ENTRY=1 ;;
        --secure-entry=*)        SECURE_ENTRY=1; SECURE_ENTRY_PATH="${arg#*=}" ;;
        --fixed)                 USE_FIXED_DEFAULT=1 ;;
        --report-url=*)          REPORT_URL_RAW="${arg#*=}" ;;
        --report-allow-http)     REPORT_ALLOW_HTTP=1 ;;
        --report-key=*)
            # 标记为"通过 CLI 传入了 KEY",arg-parse 阶段 warn() 还没定义,
            # 后面起 helpers 后再发警告。
            REPORT_KEY_RAW="${arg#*=}"
            REPORT_KEY_VIA_CLI=1
            ;;
        v*|V*|[0-9]*)            TARGET_VERSION="$arg" ;;
        *) printf -- "%b\n" "${red}未知参数: $arg${plain}" >&2
           echo "用 --help 看支持的参数" >&2
           exit 1 ;;
    esac
done

# ---------- output helpers ----------

step() { echo -e "${blue}▸${plain} $*"; }
ok()   { echo -e "${green}✓${plain} $*"; }
warn() { echo -e "${yellow}!${plain} $*" >&2; }
err()  { echo -e "${red}✗${plain} $*" >&2; }
die()  { err "$*"; exit 1; }

# ---------- preflight ----------

[[ $EUID -ne 0 ]] && die "必须以 root 身份运行此脚本"

if ! command -v systemctl >/dev/null 2>&1; then
    die "本机没有 systemd,无法安装。仅支持带 systemd 的发行版"
fi

case $(uname -m) in
    x86_64|x64|amd64)            ARCH=amd64 ;;
    i*86|x86)                    ARCH=386 ;;
    aarch64|arm64|armv8*|armv8)  ARCH=arm64 ;;
    armv7l|armv7*|armv7|arm)     ARCH=armv7 ;;
    armv6*|armv6)                ARCH=armv6 ;;
    armv5*|armv5)                ARCH=armv5 ;;
    s390x)                       ARCH=s390x ;;
    *) die "未支持的 CPU 架构:$(uname -m)" ;;
esac

# detect existing install — 装过了就让用 update.sh,除非 --force
existing_install=false
if [[ -f "${SERVICE_FILE}" ]] || [[ -x "${INSTALL_DIR}/sui" ]]; then
    existing_install=true
fi
if ${existing_install} && ! ${FORCE}; then
    err "检测到 ${CMD_NAME} 已安装(存在 ${SERVICE_FILE} 或 ${INSTALL_DIR}/sui)。"
    echo
    echo -e "  ${cyan}日常升级请用:${plain}"
    echo -e "    bash <(curl -Ls https://raw.githubusercontent.com/${GH_OWNER}/${GH_REPO}/main/update.sh)"
    echo -e "    或:  ${CMD_NAME} update"
    echo
    echo -e "  ${cyan}强制重装(覆盖二进制,保留 ${INSTALL_DIR}/db/):${plain}"
    echo -e "    bash <(curl -Ls https://raw.githubusercontent.com/${GH_OWNER}/${GH_REPO}/main/install.sh) --force"
    exit 1
fi

# DATA_DIR (db/) 父目录可写
if [[ ! -w "$(dirname "${INSTALL_DIR}")" ]]; then
    die "无法写入 $(dirname "${INSTALL_DIR}") — 检查 root 权限"
fi

free_tmp_kb=$(df -k /tmp 2>/dev/null | awk 'NR==2{print $4}')
if [[ -n "${free_tmp_kb}" && "${free_tmp_kb}" -lt 102400 ]]; then
    warn "/tmp 可用空间 < 100MB($((free_tmp_kb / 1024))MB),下载/解压可能失败"
fi

mem_kb=$(awk '/MemTotal/{print $2}' /proc/meminfo 2>/dev/null || echo 0)
if [[ "${mem_kb}" -gt 0 && "${mem_kb}" -lt 524288 ]]; then
    warn "系统内存 < 512MB($((mem_kb / 1024))MB),sing-box + 面板 + sqlite 同时运行可能 OOM"
fi

# 上游 s-ui / 3x-ui 共存提醒
if [[ -f /etc/systemd/system/s-ui.service ]] || [[ -x /usr/local/s-ui/sui ]]; then
    warn "检测到 alireza0/s-ui 也在本机。两个面板路径独立,但端口请避免冲突(本脚本随机端口可避开)"
fi

echo -e "${green}架构:${plain} ${ARCH}"

# ---------- deps ----------

ensure_pkg() {
    local pkgs="$*"
    if command -v apt-get >/dev/null 2>&1; then
        apt-get update -y >/dev/null 2>&1 || true
        DEBIAN_FRONTEND=noninteractive apt-get install -y ${pkgs} >/dev/null 2>&1 || return 1
    elif command -v dnf >/dev/null 2>&1; then
        dnf install -y ${pkgs} >/dev/null 2>&1 || return 1
    elif command -v yum >/dev/null 2>&1; then
        yum install -y ${pkgs} >/dev/null 2>&1 || return 1
    elif command -v zypper >/dev/null 2>&1; then
        zypper -q install -y ${pkgs} >/dev/null 2>&1 || return 1
    elif command -v pacman >/dev/null 2>&1; then
        pacman -Sy --noconfirm ${pkgs} >/dev/null 2>&1 || return 1
    elif command -v apk >/dev/null 2>&1; then
        apk add --no-cache ${pkgs} >/dev/null 2>&1 || return 1
    else
        return 1
    fi
}

required_tools=(curl tar awk grep sed)
missing=()
for t in "${required_tools[@]}"; do
    command -v "$t" >/dev/null 2>&1 || missing+=("$t")
done
if ! command -v sha256sum >/dev/null 2>&1; then
    missing+=(coreutils)
fi
# ss 用来探测端口占用(随机端口避坑) — iproute2 包
if ! command -v ss >/dev/null 2>&1; then
    missing+=(iproute2)
fi
if [[ ${#missing[@]} -gt 0 ]]; then
    step "安装缺失依赖: ${missing[*]}"
    ensure_pkg "${missing[@]}" || warn "自动安装失败 — 可手动 \`apt install ${missing[*]}\` 或等价命令"
fi
for t in curl tar awk grep sed; do
    command -v "$t" >/dev/null 2>&1 || die "依赖 '$t' 不可用,无法继续"
done

# ---------- download + verify ----------

resolve_version() {
    if [[ -n "$1" ]]; then
        echo "$1"
        return
    fi
    # 用 /releases?per_page=1 而非 /releases/latest:后者忽略所有 prerelease,
    # 而本仓库 CI 默认会把每个 tag 发为 prerelease(直到 maintainer 手动 promote
    # 成正式 release)。前者直接返回最新一条 — 不论 prerelease 与否。
    # 先把 API 响应整段读完再 grep,避免 grep -m1 早退导致 curl SIGPIPE
    # 在 set -o pipefail 下报 curl 23 ("Failure writing output to destination")。
    local raw v
    raw=$(curl -fsSL "https://api.github.com/repos/${GH_OWNER}/${GH_REPO}/releases?per_page=1" 2>/dev/null || true)
    v=$(printf '%s\n' "$raw" | grep -m1 -E '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [[ -z "$v" ]]; then
        die "无法获取最新版本号(GitHub API 限流?或仓库尚无 release)"
    fi
    echo "$v"
}

download_release() {
    # 调用方用 $(...) 捕获 tarball 路径,所有进度输出必须走 stderr
    local version="$1"
    local pkg="${PKG_PREFIX}-linux-${ARCH}.tar.gz"
    local url="https://github.com/${GH_OWNER}/${GH_REPO}/releases/download/${version}/${pkg}"
    local sum_url="https://github.com/${GH_OWNER}/${GH_REPO}/releases/download/${version}/checksums.txt"
    local dest="/tmp/${PKG_PREFIX}-${version}-${ARCH}.tar.gz"
    local sum_dest="/tmp/${PKG_PREFIX}-${version}-checksums.txt"

    step "下载: ${url}" >&2
    if ! curl -fSL --connect-timeout 10 -o "${dest}" "${url}" >&2; then
        rm -f "${dest}"
        die "下载失败 — release ${version} 可能不存在,或 GitHub 网络不通"
    fi

    # SHA256 best-effort:有 checksums.txt 就强校验,没有就 warn 后继续。
    if curl -fsSL --connect-timeout 10 -o "${sum_dest}" "${sum_url}" 2>/dev/null && command -v sha256sum >/dev/null 2>&1; then
        step "校验 SHA256…" >&2
        local expected actual
        expected=$(awk -v want="${pkg}" '$2 == want || $2 == "*"want {print $1; exit}' "${sum_dest}")
        if [[ -z "${expected}" ]]; then
            warn "checksums.txt 中没有 ${pkg} 的条目,跳过校验"
        else
            actual=$(sha256sum "${dest}" | awk '{print $1}')
            if [[ "${expected}" != "${actual}" ]]; then
                rm -f "${dest}" "${sum_dest}"
                die "SHA256 不匹配!  expected=${expected}  actual=${actual}"
            fi
            ok "  SHA256 OK: ${actual}" >&2
        fi
        rm -f "${sum_dest}"
    else
        warn "release 未提供 checksums.txt,跳过 SHA256 校验" >&2
    fi

    echo "${dest}"
}

# ---------- install ----------

handle_legacy_singbox() {
    # 老安装可能还残留 sing-box.service(上游 v1.x 旧拓扑),停掉并清掉
    # 二进制残骸 — 与原 install.sh 的 prepare_services 同语义
    if [[ -f /etc/systemd/system/sing-box.service ]]; then
        step "清理老 sing-box.service…"
        systemctl stop sing-box 2>/dev/null || true
        systemctl disable sing-box 2>/dev/null || true
        rm -f /etc/systemd/system/sing-box.service
        rm -f "${INSTALL_DIR}/bin/sing-box" "${INSTALL_DIR}/bin/runSingbox.sh" "${INSTALL_DIR}/bin/signal" 2>/dev/null || true
    fi
}

install_panel() {
    local version="$1"
    local archive="$2"

    if ${existing_install}; then
        step "停止旧服务"
        systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
    fi

    handle_legacy_singbox

    # 解压到临时目录后再 cp,这样 db/ 不会被 release 包覆盖
    local extract_dir="/tmp/${CMD_NAME}-extract"
    rm -rf "${extract_dir}"
    mkdir -p "${extract_dir}"
    tar -xzf "${archive}" -C "${extract_dir}/"
    if [[ ! -d "${extract_dir}/${PKG_PREFIX}" ]]; then
        rm -rf "${extract_dir}"
        die "压缩包结构异常(缺少 ${PKG_PREFIX}/ 目录)"
    fi

    mkdir -p "${INSTALL_DIR}"

    # 替换二进制 + 脚本 + service unit;永远不动 ${INSTALL_DIR}/db/
    install -m 0755 "${extract_dir}/${PKG_PREFIX}/sui" "${INSTALL_DIR}/sui"
    if [[ -f "${extract_dir}/${PKG_PREFIX}/${CMD_NAME}.sh" ]]; then
        install -m 0755 "${extract_dir}/${PKG_PREFIX}/${CMD_NAME}.sh" "${INSTALL_DIR}/${CMD_NAME}.sh"
        install -m 0755 "${INSTALL_DIR}/${CMD_NAME}.sh" "/usr/bin/${CMD_NAME}"
    fi
    if [[ -d "${extract_dir}/${PKG_PREFIX}/bin" ]]; then
        rm -rf "${INSTALL_DIR}/bin"
        cp -a "${extract_dir}/${PKG_PREFIX}/bin" "${INSTALL_DIR}/bin"
        chmod +x "${INSTALL_DIR}/bin/"* 2>/dev/null || true
    fi
    # GitHub Actions runner 默认 uid 1001;若上游打包未 chown,这里强制纠回 root
    chown -R root:root "${INSTALL_DIR}" 2>/dev/null || true
    mkdir -p "${INSTALL_DIR}/db"
    chmod 700 "${INSTALL_DIR}/db"

    # service file:tarball 里有就装上(常见命名 *.service)
    local svc_src
    svc_src=$(find "${extract_dir}/${PKG_PREFIX}" -maxdepth 1 -name "*.service" | head -n1)
    if [[ -n "${svc_src}" ]]; then
        install -m 0644 "${svc_src}" "${SERVICE_FILE}"
    elif [[ ! -f "${SERVICE_FILE}" ]]; then
        # tarball 没有 service 文件且本机也没有 — 现造一个含加固选项的版本
        # (AUDIT.md H6,详细说明见仓库 nexcore-s-ui.service)
        cat > "${SERVICE_FILE}" <<EOF
[Unit]
Description=${CMD_NAME} Service
After=network.target
Wants=network.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}/
ExecStart=${INSTALL_DIR}/sui
Restart=on-failure
RestartSec=10s
NoNewPrivileges=true
ProtectSystem=full
ReadWritePaths=${INSTALL_DIR}
ProtectHome=true
PrivateTmp=true
LockPersonality=true
RestrictRealtime=true
RestrictSUIDSGID=true

# 内存兜底(1 核 1G 的小节点是常态)。两层配合:
#   GOMEMLIMIT  程序启动时按本机内存自算(约 60%),软限 —— 接近时更积极 GC
#   MemoryMax   systemd 硬限(85%),真的失控才 kill,配合上面的 Restart 自愈
# 硬限必须【高于】软限,否则 Go 还没来得及 GC 就被杀,等于白设。
# 刻意不设 MemorySwapMax:小内存机器上 swap 是救命的,禁掉反而更早 OOM。
MemoryAccounting=true
MemoryMax=85%

[Install]
WantedBy=multi-user.target
EOF
        chmod 644 "${SERVICE_FILE}"
    fi

    rm -rf "${extract_dir}" "${archive}"

    systemctl daemon-reload
    systemctl enable "${SERVICE_NAME}" >/dev/null 2>&1 || true
    systemctl reset-failed "${SERVICE_NAME}" 2>/dev/null || true
}

# ---------- post-install: migrate, seed admin, setting, start ----------

# 24 字符 base64 alpha-num,去掉特殊符避免 URL escape / shell 引用陷阱。
# /dev/urandom 比 $RANDOM 强(后者 16 位熵不够撑长 slug)。
random_slug() {
    local len="${1:-16}"
    head -c 64 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c "$len"
}

random_credential() {
    head -c 12 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 12
}

# random_free_port 在 10000-60000 找一个未被监听的端口。50 次重试上限
# (理论冲突率近 0,但万一 ephemeral port pool 大半被占,留个保险出口)。
# port-in-use 探测优先 ss,fallback /proc/net/{tcp,tcp6}(纯 bash 兜底)。
random_free_port() {
    local p tries
    for tries in $(seq 1 50); do
        # $RANDOM 是 0-32767(15-bit),拼一次组合给 30-bit 撑足 10000-60000 范围
        p=$(( ( (RANDOM << 15) | RANDOM ) % 50000 + 10000 ))
        if command -v ss >/dev/null 2>&1; then
            if ! ss -tlnH 2>/dev/null | awk '{print $4}' | grep -qE "[:.]${p}\$"; then
                echo "$p"; return
            fi
        else
            # 纯 bash fallback:读 /proc/net/tcp{,6} 第二列 hex 端口
            local hex_port
            hex_port=$(printf '%04X' "$p")
            if ! grep -qE "[: ]${hex_port} " /proc/net/tcp /proc/net/tcp6 2>/dev/null; then
                echo "$p"; return
            fi
        fi
    done
    # 50 次都没找到自由端口 — 异常环境,直接退回固定 3095 让 sui 自己报冲突
    warn "50 次随机找不到自由端口,回退到 3095"
    echo "3095"
}

# apply_initial_settings 决定首装的 port/path 最终值,写入 DB。
# 必须在 systemctl start 之前(否则 sui 进程跟我们抢 sqlite lock 拿不到写权)。
INITIAL_PORT=""
INITIAL_PATH=""
apply_initial_settings() {
    local port="${PANEL_PORT}"
    local path="${PANEL_PATH}"

    if [[ "${USE_FIXED_DEFAULT}" = "1" ]]; then
        # --fixed 给"我习惯老地址"的运维一个回退,跳过任何随机化
        port="${port:-3095}"
        path="${path:-/app/}"
    else
        # 端口:CLI/ENV > 随机
        if [[ -z "${port}" ]]; then
            port="$(random_free_port)"
            step "随机分配面板端口: ${cyan}${port}${plain}"
        fi
        # path:CLI/ENV > secure-entry 自定义 > secure-entry 随机长 > 默认随机短
        if [[ -z "${path}" ]]; then
            local slug
            if [[ "${SECURE_ENTRY}" = "1" && -n "${SECURE_ENTRY_PATH}" ]]; then
                slug="${SECURE_ENTRY_PATH}"
            elif [[ "${SECURE_ENTRY}" = "1" ]]; then
                slug="$(random_slug 32)"
                step "启用安全入口(${cyan}--secure-entry${plain},32 字符 slug)"
            else
                slug="$(random_slug 16)"
                step "随机分配面板入口(16 字符 slug)"
            fi
            # 清头尾 / 再补一遍,确保拼好 /xxx/
            slug="${slug##/}"
            slug="${slug%%/}"
            path="/${slug}/"
        else
            # 用户传了 path,确保前后都带 /(sui setting 要求严格)
            [[ "${path}" != /* ]] && path="/${path}"
            [[ "${path}" != */ ]] && path="${path}/"
        fi
    fi

    # 写入 DB(panel 没启动,不会卡 lock)。failure 不 die — 用户能后续手改。
    if ! "${INSTALL_DIR}/sui" setting -port "${port}" -path "${path}" >/dev/null 2>&1; then
        warn "写入 port/path 到 settings 失败 — 装完手动: ${cyan}${CMD_NAME}${plain} 进菜单调整"
    fi
    INITIAL_PORT="${port}"
    INITIAL_PATH="${path}"
}

setup_first_run() {
    # migrate 是幂等的,跨"首装/升级"都跑一次
    step "数据库迁移…"
    "${INSTALL_DIR}/sui" migrate || warn "migrate 报错(可能是首装空 db,可忽略)"

    if [[ -f "${DB_PATH}" ]] && "${INSTALL_DIR}/sui" admin -show 2>/dev/null | grep -q '^[[:space:]]*Username:[[:space:]]*[^[:space:]]'; then
        # 已有 admin → 升级语义,不动凭据
        FRESH_USER=""
        FRESH_PASS=""
        return
    fi

    FRESH_USER="admin_$(random_credential | tr 'A-Z' 'a-z' | head -c 6)"
    FRESH_PASS="$(random_credential)"
    step "创建初始管理员凭据(随机生成)"
    # ⚠️ 不能 redirect stderr — 失败时必须能看到具体错(否则用户卡死,
    # v1.7.12 后 bcrypt 计算 + DB AutoMigrate 慢机器可能撞 timeout)。
    # stdout 拿掉是因为成功时不需要在屏幕显示中间状态(密码后面统一展示)。
    if ! "${INSTALL_DIR}/sui" admin -username "${FRESH_USER}" -password "${FRESH_PASS}" >/dev/null; then
        warn "写入 admin 凭据失败(具体错见上方 stderr)"
        warn "可手动重置:${cyan}${INSTALL_DIR}/sui admin -reset${plain}(会输出新明文密码)"
        FRESH_USER=""
        FRESH_PASS=""
    fi
}

# ── 磁盘占用防护 ────────────────────────────────────────────────────────
#
# 节点常配 10G 小盘,而 systemd-journald 默认【没有上限】——
# 生产实测一台跑了 93 天的节点,journal 吃掉 1.9G,占 20G 盘的 10%;
# 换成 10G 盘就是 19%,而且还在涨。
#
# 用 drop-in 而不是改 /etc/systemd/journald.conf:
#   - 发行版升级不会打架,用户想撤掉删一个文件就行
#   - 主配置里用户自己写的值仍然可见,便于排查
#
# 幂等 & 不越权:检测到用户已经自己配过 SystemMaxUse 就原样不动 ——
# 那是明确意图,装个面板不该去改它。
harden_journald() {
    local dropin_dir="/etc/systemd/journald.conf.d"
    local dropin="${dropin_dir}/10-${SERVICE_NAME}.conf"

    if [[ ! -d /run/systemd/system ]]; then
        return 0
    fi

    # 用户(或别的 drop-in)已经设过就不插手。排除注释行。
    if grep -rqsE '^[[:space:]]*SystemMaxUse=' /etc/systemd/journald.conf "${dropin_dir}" 2>/dev/null; then
        if [[ ! -f "${dropin}" ]]; then
            step "检测到已有 journald 容量配置,跳过(不覆盖你的设置)"
            return 0
        fi
    fi

    step "限制 systemd journal 容量(默认无上限,小盘上会吃掉几个 G)…"
    mkdir -p "${dropin_dir}"
    cat > "${dropin}" <<'JEOF'
# 由 nexcore-s-ui 安装脚本写入。删掉本文件即可恢复系统默认(无上限)。
#
# 100M 对排障足够:节点的关键日志是启动失败、证书续签、core 崩溃,
# 这些都是低频事件。真要翻更久以前的,用 journalctl --since 之前先
# 把这个值调大再 systemctl restart systemd-journald。
[Journal]
SystemMaxUse=100M
SystemMaxFileSize=20M
RuntimeMaxUse=32M
JEOF

    systemctl restart systemd-journald 2>/dev/null || true
    # 立即回收超出新上限的历史 journal,否则要等下一次轮转
    journalctl --vacuum-size=100M >/dev/null 2>&1 || true
    ok "journal 上限已设为 100M"
}

# 安装完扫一眼磁盘占用大户,超阈值就提示 —— 只报告不动手。
#
# 这些目录属于系统范畴(snapd / apt 缓存 / 运维自己放的备份),装个面板就去
# 删用户的东西是越界的;但不说一声又会让人在 10G 小盘上莫名其妙地满。
#
# 数字来自生产实测(tw1,20G 盘用了 9.8G):
#   snapd 全家 2.7G(/snap 2.0G + /var/lib/snapd 731M),装的是
#              core20/core22/lxd/snapd —— Ubuntu 默认塞的,节点一个都用不到
#   /var/log   2.4G(journal 占 1.9G —— 这条已由 harden_journald 兜住)
#   /root      405M(历史 db/二进制备份,人工运维留下的)
#   /var/cache 253M(apt 列表与包缓存)
report_disk_hogs() {
    local root_avail_mb
    root_avail_mb=$(df -Pm / 2>/dev/null | awk 'NR==2{print $4}')
    [[ -z "${root_avail_mb}" ]] && return 0
    # 盘还很宽裕就不啰嗦
    if (( root_avail_mb > 5000 )); then
        return 0
    fi

    warn "根分区可用空间仅剩 ${root_avail_mb}MB。以下是常见占用大户(仅提示,脚本不会动它们):"

    local d sz
    for d in /snap /var/lib/snapd /var/log /var/cache /root; do
        [[ -e "${d}" ]] || continue
        sz=$(du -sh "${d}" 2>/dev/null | cut -f1)
        [[ -n "${sz}" ]] && printf '    %-18s %s\n' "${d}" "${sz}"
    done

    echo
    echo "  可考虑的清理(按收益排序,请自行确认后执行):"
    if command -v snap >/dev/null 2>&1; then
        echo "    snapd 全家约 2-3G,节点服务器通常用不到:"
        echo "      systemctl disable --now snapd.service snapd.socket && apt-get purge -y snapd"
    fi
    if [[ -d /var/cache/apt ]]; then
        echo "    apt 缓存:  apt-get clean"
    fi
    echo "    历史备份:  检查 /root 下的 *.db / sui.bak-* 是否还需要保留"
}

wait_for_active() {
    local max="${1:-30}"
    for i in $(seq 1 "${max}"); do
        if systemctl is-active --quiet "${SERVICE_NAME}"; then
            return 0
        fi
        sleep 1
    done
    return 1
}

show_credentials() {
    echo
    echo -e "${green}═════════════════════════════════════════════${plain}"
    echo -e "${green}  ${CMD_NAME} 已部署${plain}"
    echo -e "${green}═════════════════════════════════════════════${plain}"

    "${INSTALL_DIR}/sui" setting -show 2>/dev/null || \
        warn "setting -show 失败 — 试试: ${cyan}journalctl -u ${SERVICE_NAME} -n 80${plain}"

    echo
    echo -e "${green}访问地址:${plain}"
    "${INSTALL_DIR}/sui" uri 2>/dev/null || warn "无法获取 URI — 检查服务状态"

    if [[ -n "${FRESH_USER}" && -n "${FRESH_PASS}" ]]; then
        echo
        echo -e "${yellow}首装明文凭据 (★ 立即记录,后续只能 ${CMD_NAME} 菜单重置):${plain}"
        echo -e "  用户名: ${green}${FRESH_USER}${plain}"
        echo -e "  密 码:  ${green}${FRESH_PASS}${plain}"
    else
        echo
        echo -e "${yellow}保留已有管理员凭据${plain}(升级/重装语义,未生成新密码)"
        echo -e "  查看当前账号: ${cyan}${INSTALL_DIR}/sui admin -show${plain}"
    fi

    if [[ -n "${INITIAL_PORT}" || -n "${INITIAL_PATH}" ]]; then
        echo
        echo -e "${yellow}首装面板配置 (★ 端口 / 路径都已随机化,务必记录):${plain}"
        [[ -n "${INITIAL_PORT}" ]] && echo -e "  端口:   ${green}${INITIAL_PORT}${plain}"
        [[ -n "${INITIAL_PATH}" ]] && echo -e "  入口:   ${green}${INITIAL_PATH}${plain}"
        if [[ "${SECURE_ENTRY}" = "1" ]]; then
            echo -e "  ${cyan}已启用安全入口(--secure-entry):路径丢了就只能 ${CMD_NAME} setting -show 后台查${plain}"
        fi
    fi

    echo
    echo -e "${green}管理命令:${plain}"
    echo "  ${CMD_NAME}                       交互菜单"
    echo "  systemctl status ${SERVICE_NAME}  服务状态"
    echo "  journalctl -u ${SERVICE_NAME} -f  实时日志"
    echo "  ${CMD_NAME} setting -show         查询当前 端口 / 路径 / 域名 / SSL"
    echo
}

# ---------- main ----------

echo -e "${green}nexcore-s-ui · install${plain}"
VERSION="$(resolve_version "${TARGET_VERSION}")"
echo -e "${green}版本:${plain} ${VERSION}"

ARCHIVE="$(download_release "${VERSION}")"
install_panel "${VERSION}" "${ARCHIVE}"

# admin / migrate / setting 都要在 systemctl start 之前 — sui CLI 跟 panel
# 进程共用同一个 sqlite,start 之后再调 admin / setting 会卡 db lock
setup_first_run
apply_initial_settings

# 主控自动装节点场景需要在 panel 起来时 token 就已在 DB 里 — apiv1
# 在 init 时一次性把 tokens 表载入内存 cache,run-time 只信缓存。
# 所以 token 必须在 systemctl restart 之前生成。这里没 REPORT_URL 时跳过。
INSTALLER_TOKEN=""
if [[ -n "${REPORT_URL_RAW}" && -n "${REPORT_KEY_RAW}" ]]; then
    step "为主控生成 admin scope API token…"
    INSTALLER_TOKEN="$("${INSTALL_DIR}/sui" token -add -desc "installer-bootstrap" 2>/dev/null | tail -1)"
    if [[ -z "${INSTALLER_TOKEN}" ]]; then
        warn "API token 生成失败 — webhook 回调里 api.token 字段会是空"
    fi
fi

# 在启动服务之前收紧 journal 上限 —— 服务一起来就开始写日志了
harden_journald

step "启动服务"
systemctl restart "${SERVICE_NAME}"

step "等待 systemd 服务激活(最长 30s)…"
if ! wait_for_active 30; then
    err "服务未在 30s 内进入 active 状态。最近日志:"
    journalctl -u "${SERVICE_NAME}" -n 60 --no-pager || true
    die "首次启动失败 — 修复后再 \`systemctl restart ${SERVICE_NAME}\` 或重跑本脚本"
fi
ok "服务已激活"

show_credentials

report_disk_hogs

# REPORT_KEY 走 CLI 是不安全的(进 /proc/<pid>/cmdline → ps 可见),
# arg-parse 阶段还没 warn() 可调,这里补告警。fallback 仍接受,只是提醒
# 操作员"下次别这么干"。
if [[ "${REPORT_KEY_VIA_CLI}" = "1" ]]; then
    warn "--report-key= 通过命令行传入会写进 /proc/<pid>/cmdline,${red}本次任务的 KEY 已经暴露给同机其他用户${plain}"
    warn "下次请用 ${cyan}REPORT_KEY=xxx bash <(curl ...)${plain} ENV 形式传入"
fi

# 主控自动装节点 — 把刚装好的 panel URL / 端口 / 密码 / API token 一次性
# HMAC 签名 POST 到 webhook。失败 warn 不 die,凭据仍在终端 + journal,
# operator 可以手补。详细安全模型见 cmd/report.go 顶部注释。
if [[ -n "${REPORT_URL_RAW}" ]]; then
    if [[ -z "${REPORT_KEY_RAW}" ]]; then
        warn "提供了 ${cyan}--report-url${plain} 但没 ${cyan}REPORT_KEY${plain} — 拒绝裸 POST 明文凭据(无 HMAC 签名)"
        warn "正确用法:${cyan}REPORT_KEY=secret bash <(curl ... install.sh) --report-url=https://...${plain}"
    else
        echo
        step "推送安装信息到回调地址(HMAC-SHA256 签名)…"

        _allow_http_flag=""
        [[ -n "${REPORT_ALLOW_HTTP}" ]] && _allow_http_flag="-allow-http"

        # 调 binary 子命令做实际 POST — 让 Go 算 HMAC、构造 payload 一气呵成,
        #   比 bash + openssl 拼接稳。FRESH_PASS / token 经 ENV 传给子进程,
        #   `KEY=val cmd` 语法只对那一条命令生效,不污染 shell 环境。
        # token 已在 systemctl restart 前预先生成(见上方),保证主控收到时
        #   panel 的内存 cache 里已经有这个 token,可直接用。
        if REPORT_KEY="${REPORT_KEY_RAW}" \
           NEXCORE_REPORT_PASSWORD="${FRESH_PASS}" \
           NEXCORE_REPORT_API_TOKEN="${INSTALLER_TOKEN}" \
           "${INSTALL_DIR}/sui" report -url "${REPORT_URL_RAW}" ${_allow_http_flag}; then
            ok "回调地址已收到凭据"
        else
            warn "回调推送失败 — 安装本体已成功,凭据仍在终端 / journal:${cyan}journalctl -u ${SERVICE_NAME} -n 80${plain}"
            warn "或本机查询:${cyan}${INSTALL_DIR}/sui admin -show${plain}"
        fi
        unset _allow_http_flag
    fi
fi
