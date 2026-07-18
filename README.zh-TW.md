# NexCore s-ui

[简体中文](README.md) | [English](README.en.md) | [日本語](README.ja.md) | **繁體中文**

基於 [alireza0/s-ui](https://github.com/alireza0/s-ui) 二次開發的 sing-box 節點控制面板,以
**API 優先 + 自動化部署 + 主控互通**為目標。適合自行部署節點伺服器,將其接入業務系統/
代理調度系統作為受控節點;也適合個人單機部署。

> 與原版的差異:前端 Vue 3 + Element Plus 完整重寫、無人值守 install.sh / update.sh、
> 預設帳密全隨機、首次安裝即隨機連接埠、`/api/v1/*` 完整 REST 體系且 **與
> [nexcore-x-ui](https://github.com/DoBestone/nexcore-x-ui) 主控串接程式碼 100% 相容**、
> Cloudflare API Token 一鍵 DNS-01 自動簽發憑證 + 自動續期、API 呼叫日誌稽核、
> 內嵌 API 文件,與上游 `s-ui` 路徑 / 服務名稱 / 連接埠完全獨立,可同機共存。

---

## 系統需求

- **建議**:**Ubuntu 24.04 LTS**(本儲存庫主要測試目標,從 install → upgrade → 全協定入站 / 出站 / 中繼都跑過端到端驗證)
- **相容**:Ubuntu 20.04+ / Debian 11+ / CentOS Stream 8+ / OpenCloudOS / 任何帶 systemd 的現代 Linux
- **架構**:`amd64` / `arm64` / `386` / `armv5` / `armv6` / `armv7` / `s390x`
- **二進位檔**:GitHub Actions CI 採用 **musl 靜態編譯**(Bootlin toolchain),**不依賴 host glibc 版本**;Ubuntu 20.04 這類較舊的發行版也能直接執行 release 套件
- **執行身分**:root(需要 bind 80/443 低連接埠 + 寫入 ACME 憑證 + 建立 tun 裝置)

---

## 一鍵安裝

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/install.sh)
```

指定版本:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/install.sh) v1.0.0
```

強制重新安裝(覆寫二進位檔,**保留 db**):

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/install.sh) --force
```

安裝結束後腳本會**直接印出登入資訊**(隨機使用者名稱 / 隨機密碼 / 面板 URI),如下所示:

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

這組帳密**僅顯示這一次**,關閉終端機後就再也看不到了 — 請立即記錄。

支援的 CPU 架構:`amd64` / `386` / `arm64` / `armv7` / `armv6` / `armv5` / `s390x`(Linux + systemd)。

---

## 管理指令(`nexcore-s-ui` CLI)

### 服務控制
```text
nexcore-s-ui                  进入交互菜单
nexcore-s-ui start|stop|restart  启停服务
nexcore-s-ui status           状态摘要
nexcore-s-ui enable|disable   开机自启
nexcore-s-ui log              查看日志
```

### 安裝生命週期
```text
nexcore-s-ui install [tag]    安装 / 升级
nexcore-s-ui update [tag]     等价于 install (db 自动保留)
nexcore-s-ui uninstall        卸载(连数据一起删)
```

### 帳密 / 連接埠

```text
sui admin -show                              显示当前管理员
sui admin -username <u> -password <p>        修改账号密码
sui setting -port <N>                        改面板端口
sui setting -path </app/>                    改面板路径
sui setting -subPort <N>                     改订阅端口
sui setting -show                            显示所有 settings
sui uri                                      打印面板访问 URL(含 LAN / 公网)
```

> `sui` 是底層二進位檔(`/usr/local/nexcore-s-ui/sui`),`nexcore-s-ui` 是 systemd /
> 安裝層包裝。兩者並存。

完整選單:`nexcore-s-ui help`。

---

## 面板功能

- **執行狀態總覽** — CPU / 記憶體 / 磁碟 / 網路速率 / sing-box 執行狀態 / Goroutine 數量
- **入站管理** — 協定涵蓋 VLESS / VMess / Trojan / Shadowsocks / ShadowTLS /
  Hysteria(2)/ Naive / TUIC / AnyTLS / WireGuard / Tailscale / Warp / Tor /
  SSH / Reality / ECH / 全 XTLS
- **路由 / 封鎖規則** — 規則集 + 規則雙層管理,**一鍵範本**:封鎖廣告 / 惡意 /
  釣魚 / 中國大陸直連 / 私有 IP 直連 / 推薦套裝
- **用戶端訂閱** — 原生連結、JSON、Clash + 中繼資訊(流量 / 上下行 / 到期),
  內嵌 QR Code
- **TLS 中心** — 一般憑證 + Reality + ECH + ACME(含 **Cloudflare 一鍵自動**)
- **流量統計** — 入站 / 出站 / 使用者三個維度 + 用戶端流量排行 Top 5
- **API 控制台**(本儲存庫新增)
  - **Token 管理**:命名 token、TTL、撤銷、明文僅回顯一次
  - **呼叫日誌**:每次 `/apiv2/*` 與 `/api/v1/*` 呼叫的 method/path/status/
    latency/IP/Token 備註都會寫入資料庫,支援篩選 + 分頁 + 一鍵清空
  - **API 文件**:內嵌於前端,基礎 URL + curl 範例 + 全部端點速查 +
    `/api/v1` 相容對應

---

## Cloudflare 一鍵簽發 TLS

面板 → **TLS 設定** → **Cloudflare 一鍵簽發**,3 步完成網域解析 + 憑證簽發:

1. **Token + 信箱** — 貼上 Cloudflare API Token(權限 `Zone:DNS:Edit + Zone:Read`,
   Global Key 也支援),填寫 ACME 註冊信箱
2. **DNS** — 選根網域、選前綴策略(隨機 / 自訂 / 根網域)、填寫公網 IP(可一鍵
   自動取得)、決定是否走 CF 反向代理
3. **簽發** — 為 TLS 設定取個名稱,送出。完成

幕後:面板呼叫 CF API 新增 A 記錄 → 寫入 sing-box ACME-via-Cloudflare TLS 設定 →
sing-box 啟動時由內建 ACME 用戶端走 DNS-01 驗證簽發憑證,**後續自動續期**,
Token 不會持久化(僅下發給 sing-box 內嵌使用)。

API 呼叫版:

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

三層並存,各有取捨:

| 前綴 | 驗證方式 | 回應外殼 | 用途 |
|---|---|---|---|
| `/api/*` | session cookie | `{success, msg, obj}` | 面板 UI 自身 |
| `/apiv2/*` | Bearer / X-API-Token / Token | `{success, msg, obj}` | 通用腳本串接 |
| `/api/v1/*` | Bearer / X-API-Token | `{data}` / `{error,code,message,details}` | **`nexcore-x-ui` 相容**,主控可直接接入 |

完整文件已內嵌於面板,登入後進入 **API 管理 → API 文件** 即可查看。導覽(`/api/v1`):

| 資源 | 端點 |
|---|---|
| Liveness | `GET /api/v1/health` |
| 驗證自檢 | `GET /api/v1/me` |
| Server | `GET /server/status` |
| sing-box | `GET /xray/status` · `POST /xray/restart` · `GET /xray/config` · `GET /xray/logs`(`xray` 命名相容主控) |
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
| Cloudflare(s-ui only) | `POST /sui/cloudflare/zones` · `POST /sui/cloudflare/dns/upsert-a` · `POST /sui/cloudflare/tls/issue` |
| sing-box raw(s-ui only) | `GET /sui/singbox/raw-config` · `GET /sui/subscription-uri` |

業務系統串接範例:

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

## 與 nexcore-x-ui 主控串接

`/api/v1` 完全鏡像 [nexcore-x-ui](https://github.com/DoBestone/nexcore-x-ui) 的 REST 形態:
**同路徑佈局、同驗證標頭、同回應外殼、同狀態碼、同錯誤碼命名、unix 毫秒時間戳記**。
為 x-ui 撰寫的主控串接程式碼可直接指向本節點,無需修改:

```diff
- HOST=https://x-node.example.com/api/v1     # nexcore-x-ui 节点
+ HOST=https://s-node.example.com/app/api/v1 # nexcore-s-ui 节点
  # 同样的 Authorization: Bearer <token>
  # 同样的 {data} / {error,code,message} 响应壳
  # 同样的 HTTP 状态码语义
```

差異只在 schema 層:`/inbounds` 回傳的 settings 是 sing-box 協定(而非 xray),
主控渲染時依 `/health` 回傳的 `impl` 欄位(`nexcore-s-ui` vs `nexcore-x-ui`)
分支處理即可。

---

## 線上更新

CLI(已安裝的機器):

```bash
nexcore-s-ui update            # 升级到最新 release
nexcore-s-ui update v1.0.0     # 升级 / 降级到指定 tag
```

一鍵腳本(無需先安裝 CLI,適合自動化批次升級):

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/update.sh)
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/update.sh) v1.0.0
```

`update.sh` 與 `install.sh` 的差異:不動 `db/`(資料庫 + TLS + 用戶端記錄完整保留)、
不重新安裝系統相依套件、systemd unit 僅在 release 有變化時才更新並備份舊版,只做:
**下載 tarball → 驗證 SHA256 → 停止服務 → 替換 sui + bin/ + service →
migrate → 啟動服務**。完整重新安裝請改用 `install.sh --force`。

---

## 設定位置

| 路徑 | 內容 |
|---|---|
| `/usr/local/nexcore-s-ui/sui` | 主二進位檔 |
| `/usr/local/nexcore-s-ui/bin/sing-box` | sing-box 子行程 |
| `/usr/local/nexcore-s-ui/db/nexcore-s-ui.db` | sqlite 資料庫(訂閱 / 用戶端 / 入站 / TLS / Token / 呼叫日誌) |
| `/etc/systemd/system/nexcore-s-ui.service` | systemd 單元 |
| `/usr/bin/nexcore-s-ui` | 管理 CLI(指向 `/usr/local/nexcore-s-ui/nexcore-s-ui.sh`) |

環境變數:

| 變數 | 預設 | 說明 |
|---|---|---|
| `SUI_DB_FOLDER` | `<binary 目录>/db` | 資料庫資料夾路徑 |
| `SUI_BIN_FOLDER` | `bin` | sing-box 子行程目錄 |
| `SUI_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `SUI_DEBUG` | `false` | 除錯模式 |
| `GH_OWNER` / `GH_REPO` | `DoBestone` / `nexcore-s-ui` | 自動更新來源(install.sh / update.sh) |
| `INSTALL_DIR` | `/usr/local/nexcore-s-ui` | 自訂安裝目錄(install.sh / update.sh) |

---

## 與上游 alireza0/s-ui 共存

`nexcore-s-ui` 在路徑 / 服務名稱 / 資料庫 / 連接埠 / 指令名稱 / 瀏覽器 cookie 上與上游
`s-ui` **完全獨立**,可同機同時安裝、互不干擾。

| 維度 | 上游 `s-ui` | `nexcore-s-ui` |
|---|---|---|
| 安裝目錄 | `/usr/local/s-ui/` | `/usr/local/nexcore-s-ui/` |
| 資料庫 | `db/s-ui.db` | `db/nexcore-s-ui.db` |
| systemd | `s-ui.service` | `nexcore-s-ui.service` |
| 管理指令 | `/usr/bin/s-ui` | `/usr/bin/nexcore-s-ui` |
| 預設面板連接埠 | 2095 | **3095** |
| 預設訂閱連接埠 | 2096 | **3096** |
| 瀏覽器 cookie | `s-ui` | `nexcore-s-ui` |
| `sui -v` | `S-UI Panel 1.4.x` | `nexcore-s-ui 1.0.0` |

兩套面板需各自使用不同連接埠(預設值已錯開)。解除安裝 `nexcore-s-ui` 不會動到
`/usr/local/s-ui/`。

---

## 開發

```bash
git clone https://github.com/DoBestone/nexcore-s-ui.git
cd nexcore-s-ui
./build.sh
```

`build.sh` 依序執行:`cd frontend && npm i && npm run build` → `cp -R frontend/dist/* web/html/` →
`go build -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_musl,badlinkname,tfogo_checklinkname0,with_tailscale" -o sui main.go`。
最終產物 `./sui`(Linux / macOS arm64 約 75 MB)。

僅開發前端:

```bash
cd frontend
npm i
npm run dev    # vite dev server, 默认 :3000,代理 /app/api → :3095
```

打 tag 觸發 CI(GitHub Actions 跨平台編譯 7 個 linux arch + 2 個 windows arch,
自動發布 release):

```bash
git tag v1.0.1 && git push origin v1.0.1
```

---

## 致謝

Forked from [alireza0/s-ui](https://github.com/alireza0/s-ui)。
sing-box 來自 [SagerNet](https://github.com/SagerNet/sing-box)。
前端元件基於 [Element Plus](https://element-plus.org/) + [Vue 3](https://vuejs.org/)。
DNS / ACME 自動化採用 [Cloudflare API](https://developers.cloudflare.com/api/)。

GPL v3。
