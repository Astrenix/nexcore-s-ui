# NexCore s-ui

A sing-box node control panel based on [alireza0/s-ui](https://github.com/alireza0/s-ui), with
**API-First + Automated Deployment + Master-Node Communication** as the goal. Suitable for self-deployed node servers, integrate it into business systems/
proxy scheduling systems as controlled nodes; also suitable for personal standalone deployment.

> **Differences from the original**: Frontend completely rewritten with Vue 3 + Element Plus, unattended install.sh / update.sh,
> default credentials fully randomized, random port on first installation, `/api/v1/*` complete REST system and **100% compatible with**
> **[nexcore-x-ui](https://github.com/DoBestone/nexcore-x-ui) master controller integration code**,
> Cloudflare API Token one-click DNS-01 automatic certificate signing + automatic renewal, API call audit logging,
> embedded API documentation, completely independent from upstream `s-ui` in path / service name / port, can coexist on the same machine.

---

## System Requirements

- **Recommended**: **Ubuntu 24.04 LTS** (main test target of this repository, verified end-to-end from install → upgrade → all protocols inbound / outbound / relay)
- **Compatible**: Ubuntu 20.04+ / Debian 11+ / CentOS Stream 8+ / OpenCloudOS / any modern Linux with systemd
- **Architecture**: `amd64` / `arm64` / `386` / `armv5` / `armv6` / `armv7` / `s390x`
- **Binary**: GitHub Actions CI using **musl static compilation** (Bootlin toolchain), **not dependent on host glibc version**; can run release packages directly on old distributions like Ubuntu 20.04
- **Running Identity**: root (required to bind ports 80/443 + write ACME cert + create tun device)

---

## One-Click Installation

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/install.sh)
```

Specify version:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/install.sh) v1.0.0
```

Force reinstall (overwrite binary, **preserve db**):

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/install.sh) --force
```

After installation, the script will **directly print login information** (random username / random password / panel URI), like:

```
═════════════════════════════════════════════
  nexcore-s-ui deployed
═════════════════════════════════════════════
Current panel settings:
        Panel port:      3095
        Panel path:      /app/

Access address:
http://1.2.3.4:3095/app/

First installation plaintext credentials (★ Record immediately, can only be reset via nexcore-s-ui menu):
  Username: admin_a3f9k2
  Password:  9KqL4mPzN2vR
```

Credentials **only this once**, close the terminal and you'll never see them again — record immediately.

Supported CPU architectures: `amd64` / `386` / `arm64` / `armv7` / `armv6` / `armv5` / `s390x` (Linux + systemd).

---

## Management Commands (`nexcore-s-ui` CLI)

### Service Control
```text
nexcore-s-ui                  Enter interactive menu
nexcore-s-ui start|stop|restart  Start/stop service
nexcore-s-ui status           Status summary
nexcore-s-ui enable|disable   Boot autostart
nexcore-s-ui log              View logs
```

### Installation Lifecycle
```text
nexcore-s-ui install [tag]    Install / Upgrade
nexcore-s-ui update [tag]     Equivalent to install (db automatically preserved)
nexcore-s-ui uninstall        Uninstall (delete data too)
```

### Credentials / Port

```text
sui admin -show                              Show current administrator
sui admin -username <u> -password <p>        Change account password
sui setting -port <N>                        Change panel port
sui setting -path </app/>                    Change panel path
sui setting -subPort <N>                     Change subscription port
sui setting -show                            Show all settings
sui uri                                      Print panel access URL (LAN / Public)
```

> `sui` is the underlying binary (`/usr/local/nexcore-s-ui/sui`), `nexcore-s-ui` is the systemd /
> installation layer wrapper. Both coexist.

Full menu: `nexcore-s-ui help`.

---

## Panel Features

- **Runtime Overview** — CPU / Memory / Disk / Network Speed / sing-box running status / Goroutine count
- **Inbound Management** — Protocol coverage VLESS / VMess / Trojan / Shadowsocks / ShadowTLS /
  Hysteria(2)/ Naive / TUIC / AnyTLS / WireGuard / Tailscale / Warp / Tor /
  SSH / Reality / ECH / Full XTLS
- **Routing / Blocking Rules** — Rule set + two-layer rule management, **one-click templates**: block ads / malware /
  phishing / China mainland direct connection / private IP direct connection / recommended packages
- **Client Subscription** — Native links, JSON, Clash + metadata (traffic / upload/download / expiry),
  QR code embedded
- **TLS Center** — Regular certificates + Reality + ECH + ACME (including **Cloudflare one-click automatic**)
- **Traffic Statistics** — Inbound / Outbound / User three dimensions + top 5 client traffic leaderboard
- **API Console** (new in this repository)
  - **Token Management**: named tokens, TTL, revocation, plaintext one-time display
  - **Call Logs**: each call to `/apiv2/*` and `/api/v1/*` method/path/status/
    latency/IP/Token remarks are persisted, with filtering + pagination + one-click clear
  - **API Documentation**: embedded from frontend, base URL + curl examples + all endpoints quick reference +
    `/api/v1` compatibility mapping

---

## Cloudflare One-Click TLS Signing

Panel → **TLS Settings** → **Cloudflare One-Click Signing**, complete in 3 steps: domain resolution + certificate signing:

1. **Token + Email** — Paste Cloudflare API Token (permissions `Zone:DNS:Edit + Zone:Read`,
   Global Key also supported), fill ACME registration email
2. **DNS** — Select root domain, choose prefix strategy (random / custom / root domain), fill public IP (can
   auto-fetch one-click), decide whether to proxy via CF
3. **Sign** — Name the TLS configuration, submit. Done.

Behind the scenes: panel calls CF API to add A record → write sing-box ACME-via-Cloudflare TLS configuration →
sing-box automatically signs certificate via built-in ACME client using DNS-01 challenge, **automatic renewal afterwards**,
Token not persisted (only sent to sing-box for embedded use).

API call version:

```bash
TOKEN=...  ; CF=...  ; BASE=http://node:3095/app/api/v1

# 1) Query signable zones
curl -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -X POST -d "{\"token\":\"$CF\"}" $BASE/sui/cloudflare/zones

# 2) Add A record (random prefix)
curl -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -X POST $BASE/sui/cloudflare/dns/upsert-a -d "{
    \"token\":\"$CF\",\"zoneId\":\"<zone-id>\",
    \"random\":true,\"prefix\":\"nodeA\",
    \"ip\":\"1.2.3.4\",\"proxied\":false}"

# 3) Generate TLS record with embedded ACME configuration
curl -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -X POST $BASE/sui/cloudflare/tls/issue -d "{
    \"name\":\"cf-auto\",\"fqdn\":\"nodeA-x9k3m2.example.com\",
    \"email\":\"a@b.c\",\"token\":\"$CF\"}"
```

---

## REST API

Three layers coexist, each with tradeoffs:

| Prefix | Auth | Response Shell | Purpose |
|---|---|---|---|
| `/api/*` | session cookie | `{success, msg, obj}` | Panel UI itself |
| `/apiv2/*` | Bearer / X-API-Token / Token | `{success, msg, obj}` | Generic script integration |
| `/api/v1/*` | Bearer / X-API-Token | `{data}` / `{error,code,message,details}` | **`nexcore-x-ui` compatible**, master controller direct integration |

Complete documentation is embedded in the panel. After login, go to **API Management → API Documentation** to view. Quick reference (`/api/v1`):

| Resource | Endpoint |
|---|---|
| Liveness | `GET /api/v1/health` |
| Auth self-check | `GET /api/v1/me` |
| Server | `GET /server/status` |
| sing-box | `GET /xray/status` · `POST /xray/restart` · `GET /xray/config` · `GET /xray/logs` (`xray` naming for master controller compatibility) |
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

# Health
curl $BASE/health

# Current identity
curl -H "Authorization: Bearer $TOKEN" $BASE/me

# List all inbounds
curl -H "Authorization: Bearer $TOKEN" $BASE/inbounds

# Change panel subscription port
curl -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -X PATCH $BASE/settings -d '{"subPort":3199}'

# Client traffic
curl -H "Authorization: Bearer $TOKEN" $BASE/clients/alice/traffic
```

---

## Integration with nexcore-x-ui Master Controller

`/api/v1` completely mirrors [nexcore-x-ui](https://github.com/DoBestone/nexcore-x-ui) REST form:
**Same path layout, same auth headers, same response shell, same status codes, same error code naming, unix millisecond timestamps**.
Code written for x-ui master controller integration can point directly to this node without modification:

```diff
- HOST=https://x-node.example.com/api/v1     # nexcore-x-ui node
+ HOST=https://s-node.example.com/app/api/v1 # nexcore-s-ui node
  # Same Authorization: Bearer <token>
  # Same {data} / {error,code,message} response shell
  # Same HTTP status code semantics
```

Differences only at schema layer: `/inbounds` returns sing-box protocol settings (not xray),
when rendering in master controller, branch by the `impl` field returned in `/health` (`nexcore-s-ui` vs `nexcore-x-ui`).

---

## Online Update

CLI (already installed machine):

```bash
nexcore-s-ui update            # Upgrade to latest release
nexcore-s-ui update v1.0.0     # Upgrade / Downgrade to specific tag
```

One-click script (no need to install CLI first, suitable for automated batch upgrades):

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/update.sh)
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/update.sh) v1.0.0
```

The difference between `update.sh` and `install.sh`: doesn't touch `db/` (database + TLS + client records fully preserved),
doesn't reinstall system dependencies, systemd unit only refreshes when release changes and backs up old version, only:
**download tarball → verify SHA256 → stop service → replace sui + bin/ + service →
migrate → start service**. For complete reinstall, use `install.sh --force` instead.

---

## Configuration Locations

| Path | Content |
|---|---|
| `/usr/local/nexcore-s-ui/sui` | Main binary |
| `/usr/local/nexcore-s-ui/bin/sing-box` | sing-box subprocess |
| `/usr/local/nexcore-s-ui/db/nexcore-s-ui.db` | SQLite database (subscriptions / clients / inbounds / TLS / tokens / call logs) |
| `/etc/systemd/system/nexcore-s-ui.service` | systemd unit |
| `/usr/bin/nexcore-s-ui` | Management CLI (points to `/usr/local/nexcore-s-ui/nexcore-s-ui.sh`) |

Environment Variables:

| Variable | Default | Description |
|---|---|---|
| `SUI_DB_FOLDER` | `<binary directory>/db` | Database folder path |
| `SUI_BIN_FOLDER` | `bin` | sing-box subprocess directory |
| `SUI_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `SUI_DEBUG` | `false` | Debug mode |
| `GH_OWNER` / `GH_REPO` | `DoBestone` / `nexcore-s-ui` | Auto-update source (install.sh / update.sh) |
| `INSTALL_DIR` | `/usr/local/nexcore-s-ui` | Custom installation directory (install.sh / update.sh) |

---

## Coexistence with Upstream alireza0/s-ui

`nexcore-s-ui` is **completely independent** from upstream `s-ui` in path / service name / database / port / command name / browser cookie, can be installed simultaneously on the same machine without interference.

| Dimension | Upstream `s-ui` | `nexcore-s-ui` |
|---|---|---|
| Installation directory | `/usr/local/s-ui/` | `/usr/local/nexcore-s-ui/` |
| Database | `db/s-ui.db` | `db/nexcore-s-ui.db` |
| systemd | `s-ui.service` | `nexcore-s-ui.service` |
| Management command | `/usr/bin/s-ui` | `/usr/bin/nexcore-s-ui` |
| Default panel port | 2095 | **3095** |
| Default subscription port | 2096 | **3096** |
| Browser cookie | `s-ui` | `nexcore-s-ui` |
| `sui -v` | `S-UI Panel 1.4.x` | `nexcore-s-ui 1.0.0` |

Both panels need to occupy different ports (default values already differentiated). Uninstalling `nexcore-s-ui` will not touch `/usr/local/s-ui/`.

---

## Development

```bash
git clone https://github.com/DoBestone/nexcore-s-ui.git
cd nexcore-s-ui
./build.sh
```

`build.sh` sequentially: `cd frontend && npm i && npm run build` → `cp -R frontend/dist/* web/html/` →
`go build -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_musl,badlinkname,tfogo_checklinkname0,with_tailscale" -o sui main.go`. Final product `./sui` (Linux / macOS arm64 approximately 75 MB).

Frontend development only:

```bash
cd frontend
npm i
npm run dev    # vite dev server, default :3000, proxy /app/api → :3095
```

Push tag to trigger CI (GitHub Actions cross-compile 7 linux arch + 2 windows arch,
auto-publish release):

```bash
git tag v1.0.1 && git push origin v1.0.1
```

---

## Acknowledgments

Forked from [alireza0/s-ui](https://github.com/alireza0/s-ui).
sing-box from [SagerNet](https://github.com/SagerNet/sing-box).
Frontend components based on [Element Plus](https://element-plus.org/) + [Vue 3](https://vuejs.org/).
DNS / ACME automation uses [Cloudflare API](https://developers.cloudflare.com/api/).

GPL v3.
