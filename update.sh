#!/bin/bash
# nexcore-s-ui · update
#
# 用法:
#   bash <(curl -Ls https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/update.sh)
#   bash <(curl -Ls https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/update.sh) v1.0.0
#
# 与 install.sh 的区别:
#   - 不重装系统依赖
#   - ${INSTALL_DIR}/db/ 永远不动(数据库完整保留)
#   - .service 文件 release 中变化时刷 + 备份旧版到 .bak.<timestamp>
#   - 只:下载 tarball → stop → 替换 sui + ${CMD_NAME}.sh + bin/ → 刷 unit(如有改) → migrate → start
#
# 与上游 alireza0/s-ui 完全独立,可在同一台机器共存(详见 install.sh 头部注释)。
#
# 可覆盖的环境变量:
#   GH_OWNER  GH_REPO  INSTALL_DIR  PKG_PREFIX  CMD_NAME  SERVICE_NAME

set -eo pipefail

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

CMD_NAME="${CMD_NAME:-nexcore-s-ui}"
SERVICE_NAME="${SERVICE_NAME:-${CMD_NAME}}"
GH_OWNER="${GH_OWNER:-DoBestone}"
GH_REPO="${GH_REPO:-nexcore-s-ui}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/${CMD_NAME}}"
PKG_PREFIX="${PKG_PREFIX:-nexcore-s-ui}"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

# ---------- preflight ----------

[[ $EUID -ne 0 ]] && {
    echo -e "${red}必须以 root 身份运行此脚本${plain}" >&2
    exit 1
}

if [[ ! -f "${SERVICE_FILE}" ]] || [[ ! -x "${INSTALL_DIR}/sui" ]]; then
    echo -e "${red}${CMD_NAME} 未安装或安装不完整。请先运行 install.sh${plain}" >&2
    exit 1
fi

if ! command -v systemctl >/dev/null 2>&1; then
    echo -e "${red}本机没有 systemd${plain}" >&2
    exit 1
fi

case $(uname -m) in
    x86_64|x64|amd64)            ARCH=amd64 ;;
    i*86|x86)                    ARCH=386 ;;
    aarch64|arm64|armv8*|armv8)  ARCH=arm64 ;;
    armv7l|armv7*|armv7|arm)     ARCH=armv7 ;;
    armv6*|armv6)                ARCH=armv6 ;;
    armv5*|armv5)                ARCH=armv5 ;;
    s390x)                       ARCH=s390x ;;
    *) echo -e "${red}未支持的 CPU 架构:$(uname -m)${plain}" >&2; exit 1 ;;
esac

# ---------- resolve target version ----------

TARGET="${1:-}"
if [[ -z "${TARGET}" ]]; then
    echo -e "${green}查询最新版本…${plain}"
    # /releases?per_page=1 取最新条目(包含 prerelease);/releases/latest 会跳过
    # prerelease,不适合本仓库默认发布策略。
    # 先把 API 响应读完整再 grep — 避免 grep -m1 早退导致 curl 收 SIGPIPE,
    # 在 set -o pipefail 下整管道非 0 + curl 23 报 "Failure writing".
    RAW=$(curl -fsSL "https://api.github.com/repos/${GH_OWNER}/${GH_REPO}/releases?per_page=1" 2>/dev/null || true)
    TARGET=$(printf '%s\n' "$RAW" | grep -m1 -E '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [[ -z "${TARGET}" ]]; then
        echo -e "${red}无法获取最新版本(GitHub API 限流?或仓库尚无 release)${plain}" >&2
        exit 1
    fi
fi

# get-current-version helper —— 同样避免 head -n1 触发 SIGPIPE 把 sui 杀掉,
# 让 pipefail 把整管道判失败,导致 || echo unknown 触发把 "unknown" 追加进变量。
get_sui_version() {
    local out
    out=$("${INSTALL_DIR}/sui" -v 2>/dev/null || true)
    printf '%s\n' "$out" | head -n1 | awk '{print $NF}'
}
CURRENT=$(get_sui_version)
[[ -z "${CURRENT}" ]] && CURRENT="unknown"
echo -e "${green}当前:${plain} ${CURRENT}  ${green}目标:${plain} ${TARGET}  ${green}架构:${plain} ${ARCH}"

if [[ "${TARGET}" == "${CURRENT}" || "${TARGET}" == "v${CURRENT}" ]]; then
    echo -e "${yellow}已经是 ${CURRENT},无需更新(强制重装走:${CMD_NAME} update force / install.sh --force)${plain}"
    exit 0
fi

# ---------- download + verify ----------

PKG_NAME="${PKG_PREFIX}-linux-${ARCH}.tar.gz"
URL="https://github.com/${GH_OWNER}/${GH_REPO}/releases/download/${TARGET}/${PKG_NAME}"
SUM_URL="https://github.com/${GH_OWNER}/${GH_REPO}/releases/download/${TARGET}/checksums.txt"
TMP=$(mktemp -d -t nexcore-s-ui-update.XXXXXX)
trap 'rm -rf "${TMP}"' EXIT

# ── 磁盘空间预检 ──────────────────────────────────────────────────────
#
# 🩸 没有这一步的话,盘满时升级会【中途】失败,而最糟的失败点是
# `install -m 0755 ... sui` 写到一半 —— 二进制损坏、服务起不来,
# 比"没升级成功"严重得多。宁可在动任何文件之前就拒绝。
#
# 需要多少:tarball ~30M + 解压 ~85M(TMP 侧),安装目录侧还要容纳
# 新二进制 85M + 备份当前二进制 85M。各留一倍余量。
require_space() {
    local path="$1" need_mb="$2" what="$3" avail
    avail=$(df -Pm "${path}" 2>/dev/null | awk 'NR==2{print $4}')
    if [[ -z "${avail}" ]]; then
        return 0   # 探不到就不拦,别让预检本身变成升级的阻碍
    fi
    if (( avail < need_mb )); then
        echo -e "${red}磁盘空间不足${plain}:${what}(${path})仅剩 ${avail}MB,至少需要 ${need_mb}MB" >&2
        echo -e "  腾空间的常见办法:" >&2
        echo -e "    apt-get clean                     # apt 缓存" >&2
        echo -e "    journalctl --vacuum-size=100M     # 系统日志" >&2
        echo -e "    ls -lh ${INSTALL_DIR}/sui.bak.*   # 历史备份(本脚本也会自动轮转)" >&2
        exit 1
    fi
}
require_space "${TMP}" 250 "临时目录"
require_space "${INSTALL_DIR}" 250 "安装目录"

echo -e "${green}下载:${plain} ${URL}"
if ! curl -fSL --connect-timeout 10 -o "${TMP}/pkg.tar.gz" "${URL}"; then
    echo -e "${red}下载失败,请检查 release ${TARGET} 是否存在${plain}" >&2
    exit 1
fi

# checksums.txt 是可选 — 上游 alireza0/s-ui 不带,本仓库 release 应带。
if curl -fsSL --connect-timeout 10 -o "${TMP}/checksums.txt" "${SUM_URL}" 2>/dev/null && command -v sha256sum >/dev/null 2>&1; then
    echo -e "${green}校验 SHA256…${plain}"
    EXPECTED=$(awk -v want="${PKG_NAME}" '$2 == want || $2 == "*"want {print $1; exit}' "${TMP}/checksums.txt")
    if [[ -z "${EXPECTED}" ]]; then
        echo -e "${yellow}!${plain} checksums.txt 中没有 ${PKG_NAME} 条目,跳过校验" >&2
    else
        ACTUAL=$(sha256sum "${TMP}/pkg.tar.gz" | awk '{print $1}')
        if [[ "${EXPECTED}" != "${ACTUAL}" ]]; then
            echo -e "${red}SHA256 不匹配!${plain}" >&2
            echo -e "${red}  expected: ${EXPECTED}${plain}" >&2
            echo -e "${red}  actual:   ${ACTUAL}${plain}" >&2
            exit 1
        fi
        echo -e "${green}  SHA256 OK: ${ACTUAL}${plain}"
    fi
else
    echo -e "${yellow}!${plain} release 未提供 checksums.txt,跳过 SHA256 校验" >&2
fi

# ---------- extract + sanity ----------

tar -xzf "${TMP}/pkg.tar.gz" -C "${TMP}/"
[[ -d "${TMP}/${PKG_PREFIX}" ]]                      || { echo -e "${red}压缩包结构异常,缺少 ${PKG_PREFIX}/ 目录${plain}" >&2; exit 1; }
[[ -f "${TMP}/${PKG_PREFIX}/sui" ]]                  || { echo -e "${red}压缩包缺少二进制 sui${plain}" >&2; exit 1; }

# ---------- swap files ----------

echo -e "${green}停止服务…${plain}"
systemctl stop "${SERVICE_NAME}" 2>/dev/null || true

# 备份当前二进制以便回滚,并轮转掉更老的。
#
# 🩸 生产实测(tw1,2026-09-14):安装目录里堆着 3 个历史 sui.bak,合计 246M
# —— 比二进制本身(85M)还多,而且【没有任何清理机制】,升一次积一个。
# 节点常配 10G 盘,这是 2.5% 且只增不减。
#
# 保留 1 个:回滚只需要"上一版",再老的版本回过去也对不上当前 DB schema
# (migrate 是单向的)。要回更早的版本应该重装指定 tag,而不是靠这里的堆积。
BACKUP_KEEP=1
if [[ -f "${INSTALL_DIR}/sui" ]]; then
    cp -a "${INSTALL_DIR}/sui" "${INSTALL_DIR}/sui.bak.$(date +%Y%m%d-%H%M%S)" 2>/dev/null || true
fi
# 轮转:按时间倒序留最新的 keep 个,其余删掉。
#
# 用 mtime(ls -t)而不是按文件名排序 —— 历史遗留的备份有两种命名格式
# (sui.bak.20260510-172744-pre1717 与 sui.bak.1778434600-pre1718),
# 按名字排会把 unix 时间戳那种排错位置。
#
# 🩸 文件名由【调用点】用 glob 展开后当参数传进来,函数内不做 glob。
# 早先的写法是把 "dir/sui.bak.*" 当字符串传进来再靠 `ls -t ${pattern}`
# 的隐式展开 —— 那依赖"未加引号的变量会被 word-split + glob",而这是
# bash 特有的行为(zsh 默认不这么做,`set -f` 也会关掉它)。一旦失效,
# 表现是【一个文件都不删且不报错】,没有任何迹象。
prune_old_backups() {
    local keep="$1"; shift
    local files=() f n=0
    for f in "$@"; do
        [[ -f "$f" ]] && files+=("$f")   # glob 没匹配到时传进来的是字面量,在这里被滤掉
    done
    (( ${#files[@]} <= keep )) && return 0
    while IFS= read -r f; do
        n=$((n + 1))
        if (( n > keep )); then
            rm -f "$f" && echo -e "  清理旧备份 $(basename "$f")"
        fi
    done < <(ls -t "${files[@]}" 2>/dev/null)
}
prune_old_backups "${BACKUP_KEEP}" "${INSTALL_DIR}"/sui.bak.*
prune_old_backups 3 "${SERVICE_FILE}".bak.*

echo -e "${green}替换二进制 + 脚本…${plain}"
install -m 0755 "${TMP}/${PKG_PREFIX}/sui" "${INSTALL_DIR}/sui"
if [[ -f "${TMP}/${PKG_PREFIX}/${CMD_NAME}.sh" ]]; then
    install -m 0755 "${TMP}/${PKG_PREFIX}/${CMD_NAME}.sh" "${INSTALL_DIR}/${CMD_NAME}.sh"
    install -m 0755 "${INSTALL_DIR}/${CMD_NAME}.sh"       "/usr/bin/${CMD_NAME}"
fi

if [[ -d "${TMP}/${PKG_PREFIX}/bin" ]]; then
    rm -rf "${INSTALL_DIR}/bin"
    cp -a "${TMP}/${PKG_PREFIX}/bin" "${INSTALL_DIR}/bin"
    chown -R root:root "${INSTALL_DIR}" 2>/dev/null || true
    chmod +x "${INSTALL_DIR}/bin/"* 2>/dev/null || true
fi

# Refresh systemd unit if release tarball ships a newer one. 我们只动 parent
# unit;.service.d/*.conf drop-in 永不触碰(那是操作员的定制面)。
NEW_UNIT=""
for f in "${TMP}/${PKG_PREFIX}"/*.service; do
    [[ -f "$f" ]] && NEW_UNIT="$f" && break
done
if [[ -n "${NEW_UNIT}" ]] && ! diff -q "${NEW_UNIT}" "${SERVICE_FILE}" >/dev/null 2>&1; then
    echo -e "${green}更新 systemd unit (备份旧版本到 ${SERVICE_FILE}.bak)…${plain}"
    cp -a "${SERVICE_FILE}" "${SERVICE_FILE}.bak.$(date +%Y%m%d-%H%M%S)" 2>/dev/null || true
    install -m 0644 "${NEW_UNIT}" "${SERVICE_FILE}"
    systemctl daemon-reload
    systemctl reset-failed "${SERVICE_NAME}" 2>/dev/null || true
fi

# ---------- journal 容量上限 ----------
#
# systemd-journald 默认【没有上限】。生产实测一台跑了 93 天的节点,journal
# 吃掉 1.9G —— 节点常配 10G 小盘,这是近 20%。
#
# ⚠️ 本段与 install.sh 的 harden_journald() 是【同一件事的两份实现】:
# 两个脚本各自独立分发(update.sh 是单独下载执行的),没法 source 共享文件。
# 改任何一处都要同步另一处,否则新装机器和升级机器的行为会分叉。
#
# 幂等:已经有人配过 SystemMaxUse(包括本脚本上次写的)就什么都不做,
# 避免每次升级都重启一次 journald。
JOURNALD_DROPIN="/etc/systemd/journald.conf.d/10-${SERVICE_NAME}.conf"
if [[ -d /run/systemd/system ]] && \
   ! grep -rqsE '^[[:space:]]*SystemMaxUse=' /etc/systemd/journald.conf /etc/systemd/journald.conf.d 2>/dev/null; then
    echo -e "${green}限制 systemd journal 容量(默认无上限)…${plain}"
    mkdir -p "$(dirname "${JOURNALD_DROPIN}")"
    cat > "${JOURNALD_DROPIN}" <<'JEOF'
# 由 nexcore-s-ui 升级脚本写入。删掉本文件即可恢复系统默认(无上限)。
[Journal]
SystemMaxUse=100M
SystemMaxFileSize=20M
RuntimeMaxUse=32M
JEOF
    systemctl restart systemd-journald 2>/dev/null || true
    journalctl --vacuum-size=100M >/dev/null 2>&1 || true
fi

# ---------- migrate (跨版本 schema 演进) ----------

echo -e "${green}数据库迁移…${plain}"
"${INSTALL_DIR}/sui" migrate || echo -e "${yellow}!${plain} migrate 报错(若无 schema 变化可忽略)" >&2

# ---------- start ----------

echo -e "${green}启动服务…${plain}"
systemctl start "${SERVICE_NAME}"

for i in $(seq 1 30); do
    systemctl is-active --quiet "${SERVICE_NAME}" && break
    sleep 1
done
if ! systemctl is-active --quiet "${SERVICE_NAME}"; then
    echo -e "${red}服务未在 30s 内激活,最近日志:${plain}" >&2
    journalctl -u "${SERVICE_NAME}" -n 60 --no-pager || true
    exit 1
fi

NEW=$(get_sui_version)
[[ -z "${NEW}" ]] && NEW="unknown"
echo
echo -e "${green}═════════════════════════════════════════════${plain}"
echo -e "${green}  升级完成: ${CURRENT} → ${NEW}${plain}"
echo -e "${green}═════════════════════════════════════════════${plain}"
systemctl status "${SERVICE_NAME}" --no-pager --lines=0 | head -8 || true
