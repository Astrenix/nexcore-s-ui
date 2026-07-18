# NexCore s-ui

[简体中文](README.md) | **English** | [日本語](README.ja.md) | [繁體中文](README.zh-TW.md)

A sing-box node control panel forked from [alireza0/s-ui](https://github.com/alireza0/s-ui), built around
**API-first design + automated deployment + master-panel interoperability**. Ideal for self-hosted node
servers that plug into a business system / proxy scheduling system as managed nodes; equally suitable for
personal single-machine deployments.

> Differences from upstream: complete frontend rewrite in Vue 3 + Element Plus, unattended install.sh / update.sh,
> fully randomized default credentials, random port on first install, a complete `/api/v1/*` REST suite that is
> **100% compatible with [nexcore-x-ui](https://github.com/DoBestone/nexcore-x-ui) master-panel integration code**,
> one-click DNS-01 automatic certificate issuance + auto-renewal via Cloudflare API Token, API call audit logging,
> embedded API documentation, and paths / service names / ports fully independent from upstream `s-ui`, allowing
> both to coexist on the same machine.

---

## System Requirements

- **Recommended**: **Ubuntu 24.04 LTS** (this repo's primary test target — end-to-end verified from install → upgrade → all-protocol inbounds / outbounds / relaying)
- **Compatible**: Ubuntu 20.04+ / Debian 11+ / CentOS Stream 8+ / OpenCloudOS / any modern Linux with systemd
- **Architectures**: `amd64` / `arm64` / `386` / `armv5` / `armv6` / `armv7` / `s390x`
- **Binaries**: GitHub Actions CI builds with **static musl compilation** (Bootlin toolchain), **no dependency on the host glibc version**; even older distros like Ubuntu 20.04 can run the release packages directly
- **Runs as**: root (needed to bind low ports 80/443 + write ACME certs + create tun devices)

---

## One-Click Install

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/install.sh)
```

Install a specific version:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/install.sh) v1.0.0
```

Force reinstall (overwrites the binary, **keeps the db**):

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/install.sh) --force
```

When installation finishes, the script **prints the login information directly** (random username / random password / panel URI), like:

```
═════════════════════════════════════════════
  nexcore-s-ui 已部署
═════════════════════════════════════════════
Current panel settings:
        Panel port:      3095
        Panel path:      /app/

访问地址:
http://1.2.3.4:3095/app/

首装明文凭据 (★ 立即记录,后续只能 nexcore-s-ui 菜单重置):
  用户名: admin_a3f9k2
  密 码:  9KqL4mPzN2vR
```

The credentials are shown **only this once** — close the terminal and they are gone for good. Record them immediately.

Supported CPU architectures: `amd64` / `386` / `arm64` / `armv7` / `armv6` / `armv5` / `s390x` (Linux + systemd).

---

## Management Commands (`nexcore-s-ui` CLI)

### Service Control
```text
nexcore-s-ui                  进入交互菜单
nexcore-s-ui start|stop|restart  启停服务
nexcore-s-ui status           状态摘要
nexcore-s-ui enable|disable   开机自启
nexcore-s-ui log              查看日志
```

### Install Lifecycle
```text
nexcore-s-ui install [tag]    安装 / 升级
nexcore-s-ui update [tag]     等价于 install (db 自动保留)
nexcore-s-ui uninstall        卸载(连数据一起删)
```

### Credentials / Ports

```text
sui admin -show                              显示当前管理员
sui admin -username <u> -password <p>        修改账号密码
sui setting -port <N>                        改面板端口
sui setting -path </app/>                    改面板路径
sui setting -subPort <N>                     改订阅端口
sui setting -show                            显示所有 settings
sui uri                                      打印面板访问 URL(含 LAN / 公网)
```

> `sui` is the underlying binary (`/usr/local/nexcore-s-ui/sui`), while `nexcore-s-ui` is the systemd /
> installation-layer wrapper. Both coexist.

Full menu: `nexcore-s-ui help`.

---

## Panel Features

- **Runtime overview** — CPU / memory / disk / network throughput / sing-box status / goroutine count
- **Inbound management** — protocol coverage: VLESS / VMess / Trojan / Shadowsocks / ShadowTLS /
  Hysteria(2) / Naive / TUIC / AnyTLS / WireGuard / Tailscale / Warp / Tor /
  SSH / Reality / ECH / full XTLS
- **Routing / blocking rules** — two-tier management of rule sets + rules, with **one-click templates**: block ads / malware /
  phishing, direct-connect mainland China, direct-connect private IPs, recommended bundle
- **Client subscriptions** — native links, JSON, Clash + metadata (traffic / upload-download / expiry),
  with embedded QR codes
- **TLS center** — regular certificates + Reality + ECH + ACME (including **one-click Cloudflare automation**)
- **Traffic statistics** — three dimensions (inbound / outbound / user) + Top 5 client traffic leaderboard
- **API console** (new in this repo)
  - **Token management**: named tokens, TTL, revocation, one-time plaintext display
  - **Call logs**: every `/apiv2/*` and `/api/v1/*` call is persisted with method/path/status/
    latency/IP/token note, with filtering + pagination + one-click purge
  - **API docs**: embedded in the frontend — base URL + curl examples + quick reference for all endpoints +
    the `/api/v1` compatibility mapping

---

## One-Click TLS Issuance via Cloudflare

Panel → **TLS Settings** → **Cloudflare one-click issuance** — DNS resolution + certificate issuance in 3 steps:

1. **Token + email** — paste a Cloudflare API Token (permissions `Zone:DNS:Edit + Zone:Read`;
   Global Key is also supported), and enter the ACME registration email
2. **DNS** — pick the root domain, choose a prefix strategy (random / custom / root domain), enter the public IP (can be
   auto-detected with one click), and decide whether to route through the CF proxy
3. **Issue** — name the TLS config and submit. Done

Behind the scenes: the panel calls the CF API to add an A record → writes a sing-box ACME-via-Cloudflare TLS config →
on startup, sing-box's built-in ACME client completes the DNS-01 challenge to issue the certificate, with **automatic renewal afterwards**.
The token is not persisted (it is only handed to sing-box for embedded use).

API-driven version:

```bash
TOKEN=...  ; CF=...  ; BASE=http://node:3095/app/api/v1

# 1) 查可签发的 zone
curl -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -X POST -d "{\"token\":\"$CF\"}" $BASE/sui/cloudflare/zones

# 2) 加 A 记录(随机前缀)
curl -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -X POST $BASE/sui/cloudflare/dns/upsert-a -d "{
    \"token\":\"$CF\",\"zoneId\":\"<zone-id>\",
    \"random\":true,\"prefix\":\"nodeA\",
    \"ip\":\"1.2.3.4\",\"proxied\":false}"

# 3) 生成内嵌 ACME 配置的 TLS 记录
curl -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -X POST $BASE/sui/cloudflare/tls/issue -d "{
    \"name\":\"cf-auto\",\"fqdn\":\"nodeA-x9k3m2.example.com\",
    \"email\":\"a@b.c\",\"token\":\"$CF\"}"
```

---

## REST API

Three layers coexist, each with its own trade-offs:

| Prefix | Auth | Response envelope | Purpose |
|---|---|---|---|
| `/api/*` | session cookie | `{success, msg, obj}` | the panel UI itself |
| `/apiv2/*` | Bearer / X-API-Token / Token | `{success, msg, obj}` | general-purpose script integration |
| `/api/v1/*` | Bearer / X-API-Token | `{data}` / `{error,code,message,details}` | **`nexcore-x-ui` compatible**, direct master-panel integration |

Full documentation is embedded in the panel — after logging in, go to **API Management → API Docs**. Overview (`/api/v1`):

| Resource | Endpoints |
|---|---|
| Liveness | `GET /api/v1/health` |
| Auth self-check | `GET /api/v1/me` |
| Server | `GET /server/status` |
| sing-box | `GET /xray/status` · `POST /xray/restart` · `GET /xray/config` · `GET /xray/logs` (the `xray` naming is kept for master-panel compatibility) |
| Inbounds | `GET\|POST /inbounds` · `GET\|PUT\|DELETE /inbounds/:id` |
| Outbounds | `GET\|POST /outbounds` · `GET\|PUT\|DELETE /outbounds/:id` |
| Endpoints / Services / TLS | `GET /endpoints` · `GET /services` · `GET /tls` |
| Clients | `GET /clients` · `GET /clients/:identifier/traffic` · `POST /clients/:identifier/reset-traffic` |
| Onlines | `GET /onlines` · `GET /online-ips[/:tag]` · `GET /online-ips-by-email` |
| Traffic | `GET /traffic` · `GET /traffic/live` |
| Tokens | `GET\|POST /tokens` · `DELETE /tokens/:id` |
| Settings | `GET\|PATCH /settings` |
| Access logs | `GET\|DELETE /access-logs` |
| System | `POST /system/restart-panel` |
| Cloudflare (s-ui only) | `POST /sui/cloudflare/zones` · `POST /sui/cloudflare/dns/upsert-a` · `POST /sui/cloudflare/tls/issue` |
| sing-box raw (s-ui only) | `GET /sui/singbox/raw-config` · `GET /sui/subscription-uri` |

Business system integration example:

```bash
TOKEN=...  ; BASE=http://node:3095/app/api/v1

# 健康
curl $BASE/health

# 当前身份
curl -H "Authorization: Bearer $TOKEN" $BASE/me

# 列出所有入站
curl -H "Authorization: Bearer $TOKEN" $BASE/inbounds

# 改面板订阅端口
curl -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -X PATCH $BASE/settings -d '{"subPort":3199}'

# 客户端流量
curl -H "Authorization: Bearer $TOKEN" $BASE/clients/alice/traffic
```

---

## Integrating with the nexcore-x-ui Master Panel

`/api/v1` fully mirrors the REST shape of [nexcore-x-ui](https://github.com/DoBestone/nexcore-x-ui):
**same path layout, same auth headers, same response envelope, same status codes, same error code naming, unix millisecond timestamps**.
Master-panel integration code written for x-ui can point at this node directly with no modification:

```diff
- HOST=https://x-node.example.com/api/v1     # nexcore-x-ui 节点
+ HOST=https://s-node.example.com/app/api/v1 # nexcore-s-ui 节点
  # 同样的 Authorization: Bearer <token>
  # 同样的 {data} / {error,code,message} 响应壳
  # 同样的 HTTP 状态码语义
```

The only differences are at the schema level: the settings returned by `/inbounds` use sing-box protocols (not xray).
When rendering, the master panel simply branches on the `impl` field returned by `/health`
(`nexcore-s-ui` vs `nexcore-x-ui`).

---

## Online Updates

CLI (on machines already installed):

```bash
nexcore-s-ui update            # 升级到最新 release
nexcore-s-ui update v1.0.0     # 升级 / 降级到指定 tag
```

One-click script (no CLI needed beforehand — great for automated fleet upgrades):

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/update.sh)
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/update.sh) v1.0.0
```

How `update.sh` differs from `install.sh`: it never touches `db/` (database + TLS + client records fully preserved),
does not reinstall system dependencies, and refreshes the systemd unit only when it changed in the release (backing up the old one). It only:
**downloads the tarball → verifies SHA256 → stops the service → replaces sui + bin/ + service →
migrates → starts the service**. For a full reinstall, use `install.sh --force` instead.

---

## Configuration Locations

| Path | Contents |
|---|---|
| `/usr/local/nexcore-s-ui/sui` | main binary |
| `/usr/local/nexcore-s-ui/bin/sing-box` | sing-box child process |
| `/usr/local/nexcore-s-ui/db/nexcore-s-ui.db` | sqlite database (subscriptions / clients / inbounds / TLS / tokens / call logs) |
| `/etc/systemd/system/nexcore-s-ui.service` | systemd unit |
| `/usr/bin/nexcore-s-ui` | management CLI (points to `/usr/local/nexcore-s-ui/nexcore-s-ui.sh`) |

Environment variables:

| Variable | Default | Description |
|---|---|---|
| `SUI_DB_FOLDER` | `<binary dir>/db` | database folder path |
| `SUI_BIN_FOLDER` | `bin` | sing-box child process directory |
| `SUI_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `SUI_DEBUG` | `false` | debug mode |
| `GH_OWNER` / `GH_REPO` | `DoBestone` / `nexcore-s-ui` | self-update source (install.sh / update.sh) |
| `INSTALL_DIR` | `/usr/local/nexcore-s-ui` | custom install directory (install.sh / update.sh) |

---

## Coexistence with Upstream alireza0/s-ui

`nexcore-s-ui` is **completely independent** from upstream `s-ui` in paths / service names / database / ports /
command names / browser cookies — both can be installed on the same machine simultaneously without interfering with each other.

| Dimension | Upstream `s-ui` | `nexcore-s-ui` |
|---|---|---|
| Install directory | `/usr/local/s-ui/` | `/usr/local/nexcore-s-ui/` |
| Database | `db/s-ui.db` | `db/nexcore-s-ui.db` |
| systemd | `s-ui.service` | `nexcore-s-ui.service` |
| Management command | `/usr/bin/s-ui` | `/usr/bin/nexcore-s-ui` |
| Default panel port | 2095 | **3095** |
| Default subscription port | 2096 | **3096** |
| Browser cookie | `s-ui` | `nexcore-s-ui` |
| `sui -v` | `S-UI Panel 1.4.x` | `nexcore-s-ui 1.0.0` |

The two panels must each use different ports (the defaults are already staggered). Uninstalling `nexcore-s-ui` never touches
`/usr/local/s-ui/`.

---

## Development

```bash
git clone https://github.com/DoBestone/nexcore-s-ui.git
cd nexcore-s-ui
./build.sh
```

`build.sh` runs, in order: `cd frontend && npm i && npm run build` → `cp -R frontend/dist/* web/html/` →
`go build -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_musl,badlinkname,tfogo_checklinkname0,with_tailscale" -o sui main.go`.
The final artifact is `./sui` (about 75 MB on Linux / macOS arm64).

Frontend-only development:

```bash
cd frontend
npm i
npm run dev    # vite dev server, 默认 :3000,代理 /app/api → :3095
```

Push a tag to trigger CI (GitHub Actions cross-compiles for 7 linux archs + 2 windows archs
and publishes the release automatically):

```bash
git tag v1.0.1 && git push origin v1.0.1
```

---

## Acknowledgements

Forked from [alireza0/s-ui](https://github.com/alireza0/s-ui).
sing-box comes from [SagerNet](https://github.com/SagerNet/sing-box).
Frontend components are built on [Element Plus](https://element-plus.org/) + [Vue 3](https://vuejs.org/).
DNS / ACME automation uses the [Cloudflare API](https://developers.cloudflare.com/api/).

GPL v3.
