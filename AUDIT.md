# 全面审计报告 · nexcore-s-ui v1.7.10

> 生成日期:2026-05-10
> 范围:Go 后端(14.5K LOC)+ Vue/EP 前端(19.8K LOC)+ 构建/部署/数据库
> 方法:并行 5 个专项子代理(安全 / 后端 / 前端 / 构建 / 数据库)→ 主代理交叉验证 + 重新定级

> **⬇️ 第二轮审计(2026-08-15,针对 v1.7.30)见文件末尾「第二轮」章节 —— 修复了一个可致面板完全接管的 CRITICAL、清零 32+7 个已知 CVE,并附端到端漏洞复现证据。**

---

## 🔴 CRITICAL(立刻修)

### 1. 密码 / Token 明文存数据库
- `database/model/model.go` User.Password / Tokens.Token 都是 `string` 列,**未哈希**
- `service/user.go:63` 登录用 `WHERE username = ? and password = ?` 直接比对明文
- `service/user.go:151` `Token: common.Random(32)` 生成后**原样落库**,无 sha256/bcrypt
- 影响:DB 文件一旦被泄(误备份 / 误打包 / 攻击拿到 SQLite),所有凭证瞬间失守,token 无法防御
- 修复:登录走 bcrypt(cost ≥ 10);token 落库前 `sha256()`,客户端拿到的明文 token 只在生成那一刻返回一次

### 2. 默认管理员凭证 admin / admin 硬编码
- `database/db.go:initUser()` — 用户表为空时建 `Username:"admin", Password:"admin"`
- 如果用户绕过 `install.sh`(直接 `go build && ./sui`)或 `install.sh` 的 `sui admin -username/-password` 步骤失败,公网就是 admin/admin
- 修复:首次启动如果没有人调过 `sui admin`,**拒绝启动**并打印一次性引导(或随机生成密码并写日志/stdout 一次)

### 3. Session Cookie 缺关键安全标志
- `api/session.go:21-24`:`Secure: false`,`HttpOnly` / `SameSite` 都没设
- `gorilla/sessions` 的默认 `HttpOnly` 是 false(不是 true),即 cookie 可被前端 JS 读到 → XSS 偷 session
- 修复:`HttpOnly: true`、`SameSite: http.SameSiteLaxMode`、HTTPS 时 `Secure: true`(可根据 `webCertFile` 是否非空动态判断)

### 4. 登录无任何防爆破
- `service/user.go:Login` 没有失败计数 / IP 节流 / 延迟
- 修复:每 IP 失败 ≥ 5 次,加 5s/15s/60s 阶梯延迟,并写审计日志

### 5. `util/genLink.go` 多处类型断言无 ok 检查 → 可被坏数据 panic
- 例如 `oTls["reality"].(map[string]interface{})`、`addr["server"].(string)`、`alpnList[i] = v.(string)` 等
- 客户端 / 入站若有部分协议不带 reality 或 alpn 中混入非字符串,生成链接会 panic 整个 HTTP 进程(gin 默认 recover,但请求直接 500)
- 修复:全部改成 `if x, ok := v.(T); ok {...}` 形式

---

## 🟠 HIGH

| # | 问题 | 位置 | 一句话修复 |
|---|---|---|---|
| H1 | SQLite `PRAGMA foreign_keys` 没开 | `database/db.go:58` DSN | DSN 追加 `&_foreign_keys=on` |
| H2 | `clients.inbounds` json 列被 `json_each()` 当数组扫,空/坏 JSON 静默返 0 行,导致孤儿 client 误删 / 入站删除残留 | `service/client.go:343,351,399…` 多处 `json.Unmarshal` 不 check err | 加 err 处理,坏数据走告警分支不走"什么都没发生" |
| H3 | `service/config.go` 重启 sing-box 的 goroutine 无 ctx 取消,`lastStartFailTime` 读写无锁 race | `service/config.go:259,271,123,145` | sync.WaitGroup + 把 `lastStartFailTime` 走 atomic |
| H4 | Cron 任务 Stop 不等飞行中事务结束 | `cronjob/cronJob.go:40` | `cron.Stop()` 返回的 ctx.Done() 上 `Wait` |
| H5 | settings 表 key 没 UNIQUE 索引,saveSetting 是 read-then-write 竞态 | `database/model/model.go:7` + `service/setting.go:169` | 加 UNIQUE(key) + 改用 UPSERT |
| H6 | systemd unit 零加固,以 root 运行,无 `ProtectSystem` / `NoNewPrivileges` 等 | `nexcore-s-ui.service` + `install.sh:379-396` 兜底 unit | 加上述两个 + `PrivateTmp=true`,sing-box 的 `CAP_NET_ADMIN` 用 `AmbientCapabilities` 单独给 |
| H7 | `Logs.vue:28` `v-html="line"` 渲染后端日志 | `frontend/src/layouts/modals/Logs.vue:28` | 改 `{{ line }}` + CSS `white-space: pre-wrap`(日志里只有少量 ANSI 颜色,可走前端解析后用具名 class) |
| H8 | `entrypoint.sh` 写死 `/app/db/s-ui.db` 老路径 | 顶层 `entrypoint.sh` | docker 都已删 → 这个文件也该删,不然将来谁拷它会出鬼故障 |

---

## 🟡 MED(质量 / 健壮性)

- Session `MaxAge` 默认 `0`(永不过期)— `service/setting.go:54` → 至少给 7 天默认
- 订阅 `?host=` 入参不校验 — `api/v1/subscription.go:131` → 白名单或至少正则
- `stats(resource, tag, date_time)` 无复合索引,`settings(key)` 无索引 — 量起来后慢
- `service/client.go:611` `ResetDays ≤ 0` 时 `NextReset` 会被算成过去时间,陷入即时重置循环 — 加保护
- `prepareTls`、`buildLinkRemarkCtx` 静默吞 unmarshal err
- `database/backup.go` 未 `BEGIN IMMEDIATE` 拍快照,WAL 期间撕裂可能性
- `Settings.vue` 大量硬编码中文字符串(`"节点名称"`、`"分享链接域名来源"` 等),没走 `$t(...)` — 即便只两种语言,英文用户也看不到对应翻译
- `service/inbounds.go:239` `UpdateOutJsons` 循环更新无统一 tx,中途失败半破坏

---

## 🟢 LOW(打磨)

- `Login.vue:159` 等多处 `setTimeout(..., 350)` 凑动画时序
- `AppBar.vue` 还有原生 `<button>`,跟其它地方 `<el-button>` 不齐
- 表单必填项缺 `*` 标记
- `database/db.go:38` `os.MkdirAll(dir, 01740)` → 改 `0o700` 更干净
- `nexcore-s-ui.sh` 菜单里 `bash <(curl ... install.sh)` 的 pipe 语义,文档要说清

---

## ⚠️ 子代理判定修正记录

| Agent 报的 | 主代理修正 |
|---|---|
| 前端 Agent:**i18n 只剩 2 语言 = CRITICAL 违反 CLAUDE.md** | 实际:`locales/index.ts` 顶部注释 `"只剩两种语言,直接全量同步加载"` 表明是**主动收窄**。结论:不是回退,而是 **CLAUDE.md 第 8 条已与现状脱节**,要么改 CLAUDE.md 把 6 语言 → 2 语言,要么补回 4 语言。当前不算 bug,定级 INFO。 |
| 安全 Agent:**db dir 0o740 = CRITICAL** | `01740` 是 sticky + 740,owner 完全控制,group 只读,other 无访问。**HIGH 都不到**,放 LOW。 |
| 安全 Agent:**CSRF 是 CRITICAL** | 当前 panel 一般部署在 path 加随机后缀(`install.sh` 的 `random_slug`)+ 短期内非高价值目标,CSRF 实战门槛较高;但配合上面"无 SameSite"会放大风险。先把 SameSite 加上,CSRF 放 HIGH。 |
| 后端 Agent:多处 `json.Unmarshal` 不 check err = CRITICAL | 大部分位置有上下文 fallback,真正会导致**误删 client / 误清孤儿**的就是 H2 那条。其余降 MED。 |

---

## ✅ 验证干净的地方(可跳过)

- Token 验证用 `subtle.ConstantTimeCompare`(`api/v1/middleware.go:95-114`)
- `install.sh` SHA256 校验 + HTTPS 拉包 + 安装后落地权限
- `update.sh` 不动 DB 目录,systemd unit 备份
- 前端 `package.json` 已无 vuetify / mdi / notivue / moment 残留(CLAUDE.md 第 6 条达成)
- `docker` 残留代码已彻底清理(只剩 entrypoint.sh 这个孤儿,见 H8)
- `vue-tsc` / vite build 通过,bundle 主入口 213KB(gzip 79KB)
- 链接生成 `LinkAddrSource = panel|tls` 双分支逻辑正确
- 所有迁移脚本 `AutoMigrate` 不 drop 数据
- 版本号 1.7.10 在 `config/version` / 二进制 / CLI `-v` 三处一致

---

## 建议修复顺序(投入产出比)

**周内必做**(攻击面直接闭合):
1. 哈希密码(bcrypt)+ 哈希 token 落库 + 旧明文一次性迁移脚本(CRITICAL #1)
2. 拒启动 admin/admin(CRITICAL #2)
3. session cookie HttpOnly + SameSite + 动态 Secure(CRITICAL #3)
4. 登录失败节流(CRITICAL #4)
5. `genLink.go` 类型断言全加 ok(CRITICAL #5)

**两周内**:H1–H4(数据一致性 / 进程稳定性) + H7 v-html

**有空时**:H5–H8 + 全部 MED + LOW

---
---

# 第二轮审计 · nexcore-s-ui v1.7.30

> 生成日期:2026-08-15
> 范围:v1.7.11 → v1.7.30 新增代码(订阅池 / 屏蔽规则 / firewall / cloudflare / warp / 主页重写)+ 第一轮修复复核 + 依赖/工具链 CVE
> 方法:4 个专项子代理(旧修复复核 / 订阅池 / 其余后端 / 前端)+ 主代理亲自核验高危结论 + **在隔离 Ubuntu 测试机上端到端复现并验证修复**
> 工具:govulncheck、staticcheck、go vet、npm audit

## 🔴 CRITICAL(已修复 + 端到端验证)

### R1. `/apiv2` 认证绕过 → 面板完全接管(第一轮 C1 修复引入的回归)
- **根因**:`service/user.go` 的 `LoadTokens()` 为避免泄露 token hash,把 token 字段掩码成 `"****"`;但 `/apiv2` 鉴权的 `ReloadTokens()` 复用了它,内存鉴权表里每条 token 的值都是 `"****"`,而 `identifyToken()`(`api/apiV2Handler.go`)用裸 `t.Token == token` 比较。
- **双重后果**:
  1. **认证绕过** — 只要库里存在任意一条未过期 token,攻击者**无需任何凭证**,发 `Token: ****` 即命中,以该用户身份获得 `/apiv2` 全部能力(save / importdb / getdb / restartApp / changePass / setting / cfIssueTls …)= 未授权面板完全接管;
  2. **合法 token 全部失效** — 真实 token 在 `/apiv2` 永远匹配不上。
- **端到端复现证据**(隔离测试机,高位端口):
  | | 攻击 `Token: ****` | 合法 token |
  |---|---|---|
  | 漏洞版(v1.7.30 原始) | `BYPASSED` — 通过鉴权,`/apiv2/load` 读到完整面板配置 | `BROKEN` — `invalid token` |
  | 修复版 | `BLOCKED` — 拒绝 | `WORKS` — 恢复 |
- **修法**:新增 `LoadTokensForAuth()`(返回真实 token,仅供内存鉴权,绝不出网)+ `MatchToken()`(sha256/legacy 双模式 + `subtle.ConstantTimeCompare`,与 `/api/v1` 中间件同语义,并显式拒绝掩码值 `"****"`);`identifyToken` 改用之,鉴权表加 `sync.RWMutex`。回归测试 `service/user_token_test.go`。

## 🟠 HIGH(已修复)

- **R2. 登录爆破节流可被 `X-Forwarded-For` 伪造绕过**(第一轮 C4 残留)。`getRemoteIp`/`remoteIP` 无条件信任 XFF 首段,攻击者每请求换一个伪造 IP 即让 per-IP 节流计数归零。**修法**:新增 `util/common/ClientIP()` —— 仅当**直连对端本身是回环/私网/link-local**(即面板前挂着同机或内网反代)时才采信 XFF,公网直连一律用 TCP 层真实对端。两种部署形态都正确、无需配置。回归测试 `util/common/clientip_test.go`(11 例)。
- **R3. 订阅 fetch 无 SSRF 防护**。用户提交的订阅 URL 由服务端 fetch,无 scheme 白名单、无内网/metadata 地址过滤、重定向不校验 → 可把面板变成打内网服务 / `169.254.169.254` 云 metadata / 回环端口的跳板。**修法**:新增 `service/ssrf_guard.go` —— scheme 仅允许 http/https;`DialContext` 对**实际解析出的每个 IP** 做黑名单校验(同时挡域名解析到内网 + DNS rebinding);重定向每跳重校验、限 5 跳。回归测试 `service/ssrf_guard_test.go`。

## 🟡 MED(已修复)

- **R4. 三处认证后 SQL 注入**(拼接 query 参数进 SQL):`service/config.go` 的 `GetChanges`(actor/key)、`CheckChanges`(lu)、`service/inbounds.go` 的 `initUsers`(initUsers 表单值进 `db.Raw` 的 `IN(...)`)。虽为认证后,但对"受限 API token 持有者"是越权面。**修法**:全部改参数化(`Where("actor = ?", ...)`);`initUsers` 逐个 `ParseUint`,非数字直接拒绝。
- **R5. panic 面(裸类型断言 / 越界)**:`util/outJson.go` 的 `reality["enabled"].(bool)` / `tlsConfig["reality"].(map)` 等 4 处(第一轮 C5 只修了 genLink.go,漏了同链路的 outJson.go);`service/warp.go` 处理 Cloudflare 响应的 `.(string)` 断言与 `peers[0]`/`errors[0]` 越界(CF 返回限流页/错误 JSON 时 panic 在写事务里)。**修法**:全部 comma-ok + 长度检查。
- **R6. warp 两处 `http.Client{}` 无超时**,CF 端 stall 时请求与 SQLite 写事务无限挂起,拖垮 `busy_timeout` 触发全库 `database is locked`。**修法**:统一 `Timeout: 15s`。

## 🔵 依赖 / 工具链 CVE(已清零)

- **后端 govulncheck:32 个 code-reachable 漏洞 → 0**。
  - Go 工具链 `go.mod` directive `1.25.7 → 1.25.13`(修 ~20 个标准库 CVE:net/http、crypto/tls、html/template、net/url、crypto/x509 等;CI 用 `go-version-file: go.mod` 自动生效)。
  - `x/crypto 0.50→0.54`、`x/net 0.53→0.57`、`x/text 0.36→0.40`、`grpc 1.79.3→1.82.1`、`quic-go` replace `0.57.1→0.59.1`(修 GO-2026-5676;linux/amd64 重新构建 + 冒烟测试确认运行时兼容)。
- **前端 npm audit:7 个 high → 0**(`npm audit fix`,postcss / nanoid 等构建期依赖,仅动 lock 文件,`vite build` 通过)。

## ⚪ 已验证干净(高风险面复核结论)

- 命令注入:全仓仅 `service/firewall.go` 调 exec,命令名走 `LookPath`、参数为字面量(`status`/`--list-ports`),**无用户输入进 argv**,且全部只读。
- 路径穿越:备份/恢复用固定临时路径 + SQLite magic 校验,不接受任意用户路径。
- 鉴权覆盖:`/api/v1`(除 `/health`)、`/apiv2`、`/app/api`(除 login/logout)全部有中间件闸门;R1 修复后 `/apiv2` 闸门恢复有效。
- 前端 XSS:全仓 0 处 `v-html`/`innerHTML`/`eval`;分享链接 `encodeURIComponent`;二维码走属性绑定。
- 订阅解析器:base64/URI/vmess 解析全程 comma-ok + 边界检查,不可信订阅内容不触发 panic。

## 🟣 稳定性 / 性能(已修复 + `-race` 验证)

- **R7. core 运行时共享状态 data race → 进程偶发崩溃**。`Core.isRunning`/`instance` 及包级 `inbound_manager`/`router`/`factory`/`globalCtx` 被 `Start`/`Stop`(Save 触发的重启 goroutine)与 `AddX`/`RemoveX`(Save 直接调)、`IsRunning`/`GetInstance`(CheckCoreJob 5s / StatsJob 10s)多 goroutine 无锁并发读写;`startCoreInProgress` 的 CAS 不覆盖 `AddX` → 撕裂读 / 操作已 Close 的 box。**修法**:给 `Core` 加 `sync.RWMutex`,Start/Stop 持写锁、读方法 + 全部 AddX/RemoveX 持读锁;因 RWMutex 不可重入,AddX 持读锁期间直接读包级 `globalCtx` 不回调 `GetCtx()`。回归测试 `core/main_race_test.go`(`go test -race` 下 8 读 + 2 写并发无 race)。
- **R8. `LastUpdate` / `onlineResources` 全局无同步**。`LastUpdate int64` 被 Save/CheckChanges/DepleteClients/cloudflare 多 goroutine 读写 → 改 `atomic.Int64`;`onlineResources *onlines` 被 SaveStats(cron)就地改字段、GetOnlines/入站列表 API 读 → 改 `atomic.Pointer[onlines]`(写方构造全新快照一次性 Store,读方 Load 拿一致快照)。回归测试 `service/stats_race_test.go`。
- **R9. 订阅刷新自我 DoS**。①节点数无上限(2MB body 可展开上万节点)→ `parseURILines` 加 `subMaxNodes=1000` 上限,超限截断 + 告警;②`applyOutcomes` 每节点 First+Save 的 N+1 + 全表扫描逐条 Delete → 改 `clause.OnConflict` 批量 upsert(分批避开 SQLite 999 变量上限)+ 一条 `NOT IN` 批量删,大幅缩短写事务持锁时间、减少与 StatsJob/DepleteJob 的 `database is locked` 争用;③cron 无重入保护 → `cron.WithChain(SkipIfStillRunning)`,慢作业(大订阅刷新)不再每分钟叠 goroutine。

## 📋 遗留(本轮未改,列明修法,建议下一轮)

| 项 | 位置 | 现状 | 建议修法 |
|---|---|---|---|
| 订阅**探测拨号**下一跳 SSRF | `service/sub_probe.go` → `core/sub_probe.go` | 探测节点的 `server:port` 来自不可信机场订阅,无内网校验 → 可借 alive/last_error 差异对面板内网做端口探测 | 拨号前用 `service/ssrf_guard.go` 的 `isBlockedIP` 校验 `n.Server`(域名先解析);或加 allowlist 开关 |
| 前端 i18n 欠账 | 75/118 文件、约 815 行硬编码中文 | 违反 CLAUDE.md 双语要求,英文用户在 SubPools/BlockRules/Inbounds 新页面看到纯中文 | 补齐缺失 key → 机械替换 → CI 加 `vue/no-bare-strings` 卡回归 |
| 前端 store 就地编辑幽灵保存 | `Dns.vue`/`Rules.vue`/`Settings.vue` | 直接改 Pinia store.config,未保存脏数据会被别页整份保存 + 10s 轮询静默丢弃 | 编辑页先深拷贝、保存成功再回写(`Inbound.vue` 已是正确示范) |
| SQLite 外键未开(第一轮 H1 回退) | `database/db.go` DSN | `_foreign_keys=on` 因撞 `inbound.tls_id=0` 约定被回退;`tokens.user_id`/`clients.inbounds` 靠应用层维护 | 把 `tls_id` 改真正 nullable 后再开 FK,或保持应用层校验并补测试 |

## ✅ 本轮改动文件

- 改:`api/apiV2Handler.go`、`api/utils.go`、`api/v1/middleware.go`、`service/user.go`、`service/config.go`、`service/inbounds.go`、`service/warp.go`、`service/sub_fetcher.go`、`service/sub.go`、`service/stats.go`、`core/main.go`、`core/endpoint.go`、`cronjob/cronJob.go`、`util/outJson.go`、`go.mod`/`go.sum`、`frontend/src/plugins/api.ts`、`frontend/package-lock.json`
- 增:`service/ssrf_guard.go`、`util/common/clientip.go` + 5 个 `_test.go`(token / clientip / ssrf / core-race / stats-race 回归护栏)
- 验证:`go build`/`go vet`/`go test`/`govulncheck` 全绿;前端 `vite build` 通过、`npm audit` 0 漏洞;CRITICAL R1 在隔离测试机端到端复现 + 修复验证
