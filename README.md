# NexCore s-ui

基于 [alireza0/s-ui](https://github.com/alireza0/s-ui) 二开的 sing-box 节点控制面板,以
**API 优先 + 自动化部署 + 主控互通**为目标。适合自部署节点服务器,把它接入业务系统/
代理调度系统作为受控节点;也适合个人单机部署。

> 与原版的差异:前端 Vue 3 + Element Plus 完整重写、无人值守 install.sh / update.sh、
> 默认凭据全随机、首装即随机端口、`/api/v1/*` 完整 REST 体系且 **与
> [nexcore-x-ui](https://github.com/DoBestone/nexcore-x-ui) 主控对接代码 100% 兼容**、
> Cloudflare API Token 一键 DNS-01 自动签证书 + 自动续签、API 调用日志审计、
> 内嵌 API 文档,与上游 `s-ui` 路径 / 服务名 / 端口完全独立,可同机共存。

---

## 一键安装

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/install.sh)
```

指定版本:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/install.sh) v1.0.0
```

强制重装(覆盖二进制,**保留 db**):

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/install.sh) --force
```

安装结束后脚本会**直接打印登录信息**(随机用户名 / 随机密码 / 面板 URI),形如:

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

凭据**只此一次**,关掉终端就再也回看不到 — 立即记录。

支持的 CPU 架构:`amd64` / `386` / `arm64` / `armv7` / `armv6` / `armv5` / `s390x`(Linux + systemd)。

---

## 管理命令(`nexcore-s-ui` CLI)

### 服务控制
```text
nexcore-s-ui                  进入交互菜单
nexcore-s-ui start|stop|restart  启停服务
nexcore-s-ui status           状态摘要
nexcore-s-ui enable|disable   开机自启
nexcore-s-ui log              查看日志
```

### 安装生命周期
```text
nexcore-s-ui install [tag]    安装 / 升级
nexcore-s-ui update [tag]     等价于 install (db 自动保留)
nexcore-s-ui uninstall        卸载(连数据一起删)
```

### 凭据 / 端口

```text
sui admin -show                              显示当前管理员
sui admin -username <u> -password <p>        修改账号密码
sui setting -port <N>                        改面板端口
sui setting -path </app/>                    改面板路径
sui setting -subPort <N>                     改订阅端口
sui setting -show                            显示所有 settings
sui uri                                      打印面板访问 URL(含 LAN / 公网)
```

> `sui` 是底层二进制(`/usr/local/nexcore-s-ui/sui`),`nexcore-s-ui` 是 systemd /
> 安装层包装。两者并存。

完整菜单:`nexcore-s-ui help`。

---

## 面板能力

- **运行态总览** — CPU / 内存 / 磁盘 / 网络速率 / sing-box 运行状态 / Goroutine 数
- **入站管理** — 协议覆盖 VLESS / VMess / Trojan / Shadowsocks / ShadowTLS /
  Hysteria(2)/ Naive / TUIC / AnyTLS / WireGuard / Tailscale / Warp / Tor /
  SSH / Reality / ECH / 全 XTLS
- **路由 / 屏蔽规则** — 规则集 + 规则双层管理,**一键模板**:屏蔽广告 / 恶意 /
  钓鱼 / 中国大陆直连 / 私有 IP 直连 / 推荐套装
- **客户端订阅** — 原生链接、JSON、Clash + 元信息(流量 / 上下行 / 过期),
  二维码内嵌
- **TLS 中心** — 普通证书 + Reality + ECH + ACME(含 **Cloudflare 一键自动**)
- **流量统计** — 入站 / 出站 / 用户三维度 + 客户端流量榜 Top 5
- **API 控制台**(本仓库新增)
  - **Token 管理**:命名 token、TTL、撤销、明文一次回显
  - **调用日志**:每次 `/apiv2/*` 与 `/api/v1/*` 调用的 method/path/status/
    latency/IP/Token 备注都落库,带筛选 + 分页 + 一键清空
  - **API 文档**:从前端嵌入,基础 URL + curl 示例 + 全部端点速查 +
    `/api/v1` 兼容映射

---

## Cloudflare 一键签发 TLS

面板 → **TLS 设置** → **Cloudflare 一键签发**,3 步完成域名解析 + 证书签发:

1. **Token + 邮箱** — 粘贴 Cloudflare API Token(权限 `Zone:DNS:Edit + Zone:Read`,
   Global Key 也支持),填 ACME 注册邮箱
2. **DNS** — 选根域、选前缀策略(随机 / 自定义 / 根域)、填公网 IP(可一键
   自动获取)、决定是否走 CF 反代
3. **签发** — 给 TLS 配置取个名,提交。完事

幕后:面板调 CF API 加 A 记录 → 写入 sing-box ACME-via-Cloudflare TLS 配置 →
sing-box 启动时由内置 ACME 客户端走 DNS-01 挑战签证书,**后续自动续签**,
Token 不持久化(只下发给 sing-box 内嵌使用)。

API 调用版:

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

三层并存,各有取舍:

| 前缀 | 鉴权 | 响应壳 | 用途 |
|---|---|---|---|
| `/api/*` | session cookie | `{success, msg, obj}` | 面板 UI 自身 |
| `/apiv2/*` | Bearer / X-API-Token / Token | `{success, msg, obj}` | 通用脚本对接 |
| `/api/v1/*` | Bearer / X-API-Token | `{data}` / `{error,code,message,details}` | **`nexcore-x-ui` 兼容**,主控直接接入 |

完整文档已嵌入面板,登录后进入 **API 管理 → API 文档** 即可看到。导览(`/api/v1`):

| 资源 | 端点 |
|---|---|
| Liveness | `GET /api/v1/health` |
| 鉴权自检 | `GET /api/v1/me` |
| Server | `GET /server/status` |
| sing-box | `GET /xray/status` · `POST /xray/restart` · `GET /xray/config` · `GET /xray/logs`(`xray` 命名兼容主控) |
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

业务系统接入示例:

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

## 与 nexcore-x-ui 主控对接

`/api/v1` 完全镜像 [nexcore-x-ui](https://github.com/DoBestone/nexcore-x-ui) 的 REST 形态:
**同路径布局、同鉴权头、同响应壳、同状态码、同错误码命名、unix 毫秒时间戳**。
为 x-ui 写的主控对接代码可直接指向本节点,无需修改:

```diff
- HOST=https://x-node.example.com/api/v1     # nexcore-x-ui 节点
+ HOST=https://s-node.example.com/app/api/v1 # nexcore-s-ui 节点
  # 同样的 Authorization: Bearer <token>
  # 同样的 {data} / {error,code,message} 响应壳
  # 同样的 HTTP 状态码语义
```

差异只在 schema 层:`/inbounds` 返回的 settings 是 sing-box 协议(不是 xray),
主控渲染时按 `/health` 返回的 `impl` 字段(`nexcore-s-ui` vs `nexcore-x-ui`)
分支即可。

---

## 在线更新

CLI(已安装机器):

```bash
nexcore-s-ui update            # 升级到最新 release
nexcore-s-ui update v1.0.0     # 升级 / 降级到指定 tag
```

一键脚本(无需先装 CLI,适合自动化批量升级):

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/update.sh)
bash <(curl -fsSL https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/update.sh) v1.0.0
```

`update.sh` 与 `install.sh` 的区别:不动 `db/`(数据库 + TLS + 客户端记录完整保留)、
不重装系统依赖、systemd unit 仅在 release 中变化时刷新且备份旧版,只:
**下载 tarball → 校验 SHA256 → 停服务 → 替换 sui + bin/ + service →
migrate → 启服务**。完整重装请改用 `install.sh --force`。

---

## 配置位置

| 路径 | 内容 |
|---|---|
| `/usr/local/nexcore-s-ui/sui` | 主二进制 |
| `/usr/local/nexcore-s-ui/bin/sing-box` | sing-box 子进程 |
| `/usr/local/nexcore-s-ui/db/nexcore-s-ui.db` | sqlite 数据库(订阅 / 客户端 / 入站 / TLS / Token / 调用日志) |
| `/etc/systemd/system/nexcore-s-ui.service` | systemd 单元 |
| `/usr/bin/nexcore-s-ui` | 管理 CLI(指向 `/usr/local/nexcore-s-ui/nexcore-s-ui.sh`) |

环境变量:

| 变量 | 默认 | 说明 |
|---|---|---|
| `SUI_DB_FOLDER` | `<binary 目录>/db` | 数据库文件夹路径 |
| `SUI_BIN_FOLDER` | `bin` | sing-box 子进程目录 |
| `SUI_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `SUI_DEBUG` | `false` | 调试模式 |
| `GH_OWNER` / `GH_REPO` | `DoBestone` / `nexcore-s-ui` | 自更新源(install.sh / update.sh) |
| `INSTALL_DIR` | `/usr/local/nexcore-s-ui` | 自定义安装目录(install.sh / update.sh) |

---

## 与上游 alireza0/s-ui 共存

`nexcore-s-ui` 在路径 / 服务名 / 数据库 / 端口 / 命令名 / 浏览器 cookie 上与上游
`s-ui` **完全独立**,可同机同时安装、互不干扰。

| 维度 | 上游 `s-ui` | `nexcore-s-ui` |
|---|---|---|
| 安装目录 | `/usr/local/s-ui/` | `/usr/local/nexcore-s-ui/` |
| 数据库 | `db/s-ui.db` | `db/nexcore-s-ui.db` |
| systemd | `s-ui.service` | `nexcore-s-ui.service` |
| 管理命令 | `/usr/bin/s-ui` | `/usr/bin/nexcore-s-ui` |
| 默认面板端口 | 2095 | **3095** |
| 默认订阅端口 | 2096 | **3096** |
| 浏览器 cookie | `s-ui` | `nexcore-s-ui` |
| `sui -v` | `S-UI Panel 1.4.x` | `nexcore-s-ui 1.0.0` |

两套面板需各自占用不同端口(默认值已错开)。卸载 `nexcore-s-ui` 不会触碰
`/usr/local/s-ui/`。

---

## 开发

```bash
git clone https://github.com/DoBestone/nexcore-s-ui.git
cd nexcore-s-ui
./build.sh
```

`build.sh` 依次:`cd frontend && npm i && npm run build` → `cp -R frontend/dist/* web/html/` →
`go build -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_musl,badlinkname,tfogo_checklinkname0,with_tailscale" -o sui main.go`。
最终产物 `./sui`(Linux / macOS arm64 约 75 MB)。

仅开发前端:

```bash
cd frontend
npm i
npm run dev    # vite dev server, 默认 :3000,代理 /app/api → :3095
```

打 tag 触发 CI(GitHub Actions 跨编译 7 个 linux arch + 2 个 windows arch,
自动发布 release):

```bash
git tag v1.0.1 && git push origin v1.0.1
```

---

## 致谢

Forked from [alireza0/s-ui](https://github.com/alireza0/s-ui)。
sing-box 来自 [SagerNet](https://github.com/SagerNet/sing-box)。
前端组件基于 [Element Plus](https://element-plus.org/) + [Vue 3](https://vuejs.org/)。
DNS / ACME 自动化用 [Cloudflare API](https://developers.cloudflare.com/api/)。

GPL v3。
