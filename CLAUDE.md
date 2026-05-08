# Claude 工作交接(本仓库专用)

> 本文件由用户在 2026-05-09 凌晨交付。Claude 进入仓库后必须先读本文件,严格按以下规则工作。

---

## 当前正在进行的工作

**任务**:把 `frontend/` 从 Vuetify 4 全量迁移到 Element Plus,严格遵循 NexCore 统一 UI 设计规范。

**计划文档**:`UI_MIGRATION_PLAN.md`(项目根)。所有阶段、文件清单、验收标准、风险登记都在里面。
**任务追踪**:Claude Code 内置 Task 系统,共 6 个任务(阶段 0 已完成)。

---

## 用户交代的执行规则(必须遵守)

1. **不停**:接到指令后直到所有阶段完成,**不要中途询问、不要等用户确认**。用户要去睡觉,期望睡醒前看到全部前端完成重构。
2. **自审**:每阶段完成后自己跑反 AI 味 checklist + 常见踩坑 checklist + dev server / build 验证,自己发现自己的问题自己修。
3. **自决**:实现细节、组件选择、样式调整、依赖处理等全部自行决定,不需要问。
4. **自更**:每阶段完成后,在 `UI_MIGRATION_PLAN.md` 对应章节标 `✅ 完成于 YYYY-MM-DD`,记录踩坑、规范偏离。
5. **不破坏功能**:这是全量"UI 重新设计 + 框架迁移",**业务功能 100% 保留**。后端 API、router、store 不动。
6. **不留 Vuetify**:阶段 5 结束时 `package.json` 不能有 vuetify、`@mdi/font`、`material-design-icons-iconfont`、`roboto-fontface`、`vite-plugin-vuetify`、`notivue`、`moment`(全部已替换为 EP / dayjs / ElMessage)。
7. **构建产物路径不变**:`frontend/dist/` → `web/html/` → Go `embed.FS`,这个契约由 `build.sh` 与 `web/web.go` 硬编码,绝对不动。
8. **国际化全保留**:6 语言(en / fa / vi / zhHans / zhHant / ru)。
9. **主色统一 NexCore 标准** `#3b82f6`。
10. **严格遵循 NexCore UI 规范**:`~/.claude/skills/nexcore-ui/`(已激活)。`design-system.md` + `anti-ai-slop.md` + `common-pitfalls.md` 三件套。

---

## 阶段进度

- ✅ 阶段 0 · 计划文档(完成 2026-05-09)
- ✅ 阶段 1 · 脚手架(完成 2026-05-09)
- ✅ 阶段 2 · 高频主操作页(完成 2026-05-09)
- ✅ 阶段 3 · 配置类页面(完成 2026-05-09)
- ✅ 阶段 4 · 协议/传输/TLS/服务子组件(完成 2026-05-09)
- ✅ 阶段 5 · 全量验收(完成 2026-05-09)

**整套迁移已完成**。端到端验证通过:
- `npx vite build` → `frontend/dist/` 输出 2.2 MB
- `cp -R frontend/dist/* web/html/` → Go embed 正常
- `go build` → 53 MB 二进制
- 启动 `./sui` → HTTP 200,EP 前端正确加载

后续如需细化协议字段表单、字段化 modal 编辑、实时图表 tile,见 `UI_MIGRATION_PLAN.md` 末尾「未来增强」章节。

具体清单见 `UI_MIGRATION_PLAN.md`。

---

## 已经清理掉的(不要再加回来)

- 整个 docker 体系(Dockerfile / docker-compose / .dockerignore / .github/workflows/docker.yml / docker-build-test.sh)
- `logger/logger.go` 中 `/.dockerenv` 容器检测逻辑
- README / CONTRIBUTING 中的 Docker 章节

---

## 与上游的关系

仓库源自 `https://github.com/alireza0/s-ui`(v1.4.1 · commit 1f393fc),作为二次开发分支。**前端会与上游永久脱钩**(Vuetify → EP 不可逆),所以不再需要保留与上游合并的能力。后端代码与上游差异最小化(只删 docker 相关 + logger 修剪)。

---

## 工作风格(用户偏好)

- 中文沟通
- 简洁,不堆冗余总结
- 不擅自创建文档(本文件和 UI_MIGRATION_PLAN.md 是用户明确要求的例外)
- 改完代码自己跑 build / dev 验证,不要让用户帮忙测
- 遇到根本性阻塞才报告,小问题自己修
