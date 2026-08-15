<template>
  <div class="dashboard">
    <!-- Hero:面板/内核版本 + 资源进度环 + 快捷动作。所有信息一屏可见 -->
    <div class="hero nc-card">
      <div class="hero__brand">
        <img src="@/assets/logo.svg" alt="" class="hero__logo" />
        <div class="hero__brand-text">
          <h3 class="hero__title">{{ panelName }}</h3>
          <div class="hero__versions">
            <span class="hero__ver">
              <span class="hero__ver-label">{{ $t('main.dash.panel') }}</span>
              <span class="hero__ver-value mono">v{{ panelVersion || '—' }}</span>
            </span>
            <span class="hero__ver hero__ver--core" :class="{ 'is-stale': isCoreStale }">
              <span class="hero__ver-label">{{ $t('main.dash.core') }}</span>
              <span class="hero__ver-value mono">{{ coreVersionLabel }}</span>
              <el-tooltip v-if="isCoreStale" :content="$t('main.dash.coreStale', { cur: coreVersion, latest: LATEST_KNOWN_CORE, n: stalePatchCount })" placement="top">
                <el-icon class="hero__ver-warn"><Warning /></el-icon>
              </el-tooltip>
            </span>
          </div>
        </div>
      </div>

      <div class="hero__status" :class="sbdRunning ? 'is-up' : 'is-down'">
        <span class="hero__status-dot"></span>
        <div>
          <div class="hero__status-text">{{ sbdRunning ? $t('main.dash.running') : $t('main.dash.stopped') }}</div>
          <div class="hero__status-meta">{{ uptimeText }}</div>
        </div>
      </div>

      <div class="hero__actions">
        <el-button @click="checkUpdate" :icon="UploadFilled" size="small" :loading="updating">{{ $t('main.dash.checkUpdate') }}</el-button>
        <el-button @click="logModal.visible = true" :icon="Document" size="small">{{ $t('basic.log.title') }}</el-button>
        <el-button @click="backupModal.visible = true" :icon="DocumentCopy" size="small">{{ $t('main.dash.backup') }}</el-button>
        <el-button @click="usageStatsModal.visible = true" :icon="DataAnalysis" size="small">{{ $t('main.dash.stats') }}</el-button>
        <el-button v-if="sbdRunning" type="warning" plain :icon="Refresh" size="small" :loading="restarting" @click="restartSingbox">{{ $t('main.dash.restartCore') }}</el-button>
      </div>
    </div>

    <!-- 升级提示 dialog — checkUpdate 后弹出,显示版本对比 + 升级命令(只读)。
         不做面板自动重启升级:sui binary 自更新涉及 systemd reload + 自我替换,
         风险太高;让操作员在 SSH 终端跑 update.sh 才安全。 -->
    <el-dialog v-model="updateDialog.visible" :title="$t('main.dash.checkUpdate')" width="540">
      <div v-if="updateDialog.loading" style="text-align:center;padding:24px">
        <el-icon class="is-loading" :size="22"><Loading /></el-icon>
        <div style="margin-top:8px;color:var(--nc-text-muted)">{{ $t('main.dash.fetchingRelease') }}</div>
      </div>
      <div v-else-if="updateDialog.error" class="upd-err">
        <el-alert :title="updateDialog.error" type="error" :closable="false" show-icon />
      </div>
      <div v-else>
        <div class="upd-row"><span class="upd-k">{{ $t('main.dash.currentVersion') }}</span><span class="mono upd-v">v{{ updateDialog.current }}</span></div>
        <div class="upd-row"><span class="upd-k">{{ $t('main.dash.latestStable') }}</span>
          <span class="mono upd-v">{{ updateDialog.latestTag }}</span>
          <el-tag v-if="updateDialog.hasUpdate" type="warning" size="small">{{ $t('main.dash.hasUpdate') }}</el-tag>
          <el-tag v-else type="success" size="small">{{ $t('main.dash.upToDate') }}</el-tag>
        </div>
        <div v-if="updateDialog.publishedAt" class="upd-row">
          <span class="upd-k">{{ $t('main.dash.publishTime') }}</span>
          <span class="mono upd-v">{{ new Date(updateDialog.publishedAt).toLocaleString() }}</span>
        </div>
        <el-divider />
        <div v-if="updateDialog.hasUpdate">
          <p style="margin:0 0 6px;font-size:13px;color:var(--nc-text-1);font-weight:600">{{ $t('main.dash.upgradeCmdHint') }}</p>
          <div class="upd-cmd">
            <code class="mono">{{ updateDialog.upgradeCmd }}</code>
            <el-button size="small" :icon="CopyDocument" @click="copyCmd">{{ $t('main.dash.copy') }}</el-button>
          </div>
          <p style="margin:8px 0 0;font-size:12px;color:var(--nc-text-muted)">
            {{ $t('main.dash.coreBundledNote') }}
          </p>
        </div>
        <div v-else style="font-size:13px;color:var(--nc-text-muted)">{{ $t('main.dash.noUpdateNeeded') }}</div>
        <div style="margin-top:10px;text-align:right">
          <el-button size="small" @click="openReleasePage">{{ $t('main.dash.viewReleaseNotes') }}</el-button>
        </div>
      </div>
    </el-dialog>

    <!-- KPI 卡 + 实时网速 sparkline -->
    <div class="kpis">
      <div class="kpi-card nc-card">
        <span class="kpi__label">CPU</span>
        <div class="kpi__row">
          <span class="kpi__value mono">{{ cpuPct.toFixed(0) }}<span class="kpi__unit">%</span></span>
          <Ring :value="cpuPct" :color="ringColor(cpuPct)" />
        </div>
        <span class="kpi__meta">{{ $t('main.dash.cores', { n: cpuCores }) }}</span>
      </div>

      <div class="kpi-card nc-card">
        <span class="kpi__label">{{ $t('main.info.memory') }}</span>
        <div class="kpi__row">
          <span class="kpi__value mono">{{ memPct.toFixed(0) }}<span class="kpi__unit">%</span></span>
          <Ring :value="memPct" :color="ringColor(memPct)" />
        </div>
        <span class="kpi__meta">{{ HumanReadable.sizeFormat(memUsed) }} / {{ HumanReadable.sizeFormat(memTotal) }}</span>
      </div>

      <div class="kpi-card nc-card">
        <span class="kpi__label">↑ {{ $t('home.topTraffic.up') }} / ↓ {{ $t('home.topTraffic.down') }}</span>
        <div class="kpi__rate">
          <span class="kpi__rate-up mono">{{ rateUpText }}/s</span>
          <span class="kpi__rate-down mono">{{ rateDownText }}/s</span>
        </div>
        <Sparkline :series-up="upHistory" :series-down="downHistory" />
      </div>

      <div class="kpi-card nc-card">
        <span class="kpi__label">{{ $t('main.dash.connOnline') }}</span>
        <div class="kpi__row kpi__row--multi">
          <div class="kpi__pair"><span class="kpi__pair-k">TCP</span><span class="kpi__pair-v mono">{{ connStats.tcp }}</span></div>
          <div class="kpi__pair"><span class="kpi__pair-k">UDP</span><span class="kpi__pair-v mono">{{ connStats.udp }}</span></div>
          <div class="kpi__pair"><span class="kpi__pair-k">{{ $t('objects.user') }}</span><span class="kpi__pair-v mono">{{ onlineUsers }}</span></div>
          <div class="kpi__pair"><span class="kpi__pair-k">IP</span><span class="kpi__pair-v mono">{{ onlineIps }}</span></div>
        </div>
      </div>
    </div>

    <Logs v-model="logModal.visible" :control="logModal" :visible="logModal.visible" />
    <Backup v-model="backupModal.visible" :control="backupModal" :visible="backupModal.visible" />
    <UsageStats v-model:visible="usageStatsModal.visible" />
  </div>
</template>

<script lang="ts" setup>
import { computed, defineAsyncComponent, h, onBeforeUnmount, onMounted, ref } from 'vue'
import HttpUtils from '@/plugins/httputil'
import { HumanReadable } from '@/plugins/utils'
import Data from '@/store/modules/data'
import { Document, DocumentCopy, DataAnalysis, Refresh, Warning, UploadFilled, CopyDocument, Loading } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { i18n } from '@/locales'

const Logs = defineAsyncComponent(() => import('@/layouts/modals/Logs.vue'))
const Backup = defineAsyncComponent(() => import('@/layouts/modals/Backup.vue'))
const UsageStats = defineAsyncComponent(() => import('@/layouts/modals/UsageStats.vue'))

// LATEST_KNOWN_CORE 是 release 时已知的 sing-box stable tag。本地比较版本不
// 需要每次 hit GitHub —— 落后多少个 patch 都是参考性提示,不影响功能。
// release 时随同更新本常量(脚本可在 update.sh 阶段拉一次更新此值)。
const LATEST_KNOWN_CORE = '1.13.11'

// ---------- backing data ----------
const tilesData = ref<any>({})
const sbdRunning = computed(() => !!tilesData.value?.sbd?.running)
const panelVersion = computed(() => tilesData.value?.sys?.appVersion || '')
const panelName = computed(() => 'NexCore Panel')

const coreVersion = computed(() => tilesData.value?.sbd?.version || '')
const coreVersionLabel = computed(() => coreVersion.value ? 'v' + coreVersion.value : '—')
const isCoreStale = computed(() => {
  if (!coreVersion.value) return false
  return cmpSemver(coreVersion.value, LATEST_KNOWN_CORE) < 0
})
const stalePatchCount = computed(() => {
  const cur = coreVersion.value.split('.').map(Number)
  const lat = LATEST_KNOWN_CORE.split('.').map(Number)
  if (cur.length < 3 || lat.length < 3) return 0
  if (cur[0] !== lat[0] || cur[1] !== lat[1]) return 0
  return Math.max(0, lat[2] - cur[2])
})

const uptimeSec = computed(() => Number(tilesData.value?.sbd?.stats?.Uptime ?? 0))
const uptimeText = computed(() => sbdRunning.value
  ? i18n.global.t('main.dash.uptime', { t: HumanReadable.formatSecond(uptimeSec.value) })
  : i18n.global.t('main.dash.clickToStart'))

// CPU/内存
// 后端 GetCpuPercent() 返回单个 float64(整机聚合 CPU%);早期写成 cpu[0] 是
// 误把 gopsutil 的 percpu=false 切片误当成多核数组,实际只有一个元素已在后端
// 拆出来直接返。
const cpuPct = computed(() => Number(tilesData.value?.cpu ?? 0))
const cpuCores = computed(() => tilesData.value?.sys?.cpuCount ?? 0)
const memUsed = computed(() => Number(tilesData.value?.mem?.current ?? 0))
const memTotal = computed(() => Number(tilesData.value?.mem?.total ?? 1))
const memPct = computed(() => memTotal.value > 0 ? (memUsed.value / memTotal.value) * 100 : 0)

// 实时网速:60 点 SMA(每点 1.5s,共 90s 时间窗)
const upHistory = ref<number[]>([])
const downHistory = ref<number[]>([])
const rateUp = ref(0)
const rateDown = ref(0)
const lastSample = ref<{ up: number; down: number; ts: number } | null>(null)
const fmt = (n: number) => (!n || n < 0 || !isFinite(n)) ? '0 B' : HumanReadable.sizeFormat(n)
const rateUpText = computed(() => fmt(rateUp.value))
const rateDownText = computed(() => fmt(rateDown.value))

// TCP/UDP/在线
const connStats = ref({ tcp: 0, udp: 0 })
const onlineUsers = computed(() => Data().onlines?.user?.length ?? 0)
const onlineIps = computed(() => {
  const m = Data().onlines?.user_ips ?? {}
  // user_ips: tag(name) → ip count;总 IP 数取所有用户 IP 数总和
  return Object.values(m as Record<string, number>).reduce((s, n) => s + (n || 0), 0)
})

// ---------- helpers ----------
function ringColor(p: number): string {
  if (p < 60) return 'var(--el-color-success)'
  if (p < 85) return 'var(--el-color-warning)'
  return 'var(--el-color-danger)'
}

function cmpSemver(a: string, b: string): number {
  const pa = a.split('.').map(Number); const pb = b.split('.').map(Number)
  for (let i = 0; i < 3; i++) {
    const x = pa[i] ?? 0, y = pb[i] ?? 0
    if (x !== y) return x - y
  }
  return 0
}

// ---------- Inline subcomponents ----------
// Ring: 极简 SVG 进度环。Vue 3 函数组件必须显式声明 props,否则
// 父组件传的属性会落到 attrs 上,props 参数全 undefined。
const Ring: any = (props: { value: number; color: string }) => {
  const value = Number(props?.value ?? 0)
  const color = props?.color ?? 'var(--el-color-primary)'
  const r = 18, c = 2 * Math.PI * r
  const offset = c - (Math.max(0, Math.min(100, value)) / 100) * c
  return h('svg', { width: 44, height: 44, viewBox: '0 0 44 44', class: 'ring' }, [
    h('circle', { cx: 22, cy: 22, r, fill: 'none', stroke: 'var(--el-fill-color)', 'stroke-width': 4 }),
    h('circle', {
      cx: 22, cy: 22, r, fill: 'none', stroke: color, 'stroke-width': 4,
      'stroke-dasharray': c, 'stroke-dashoffset': offset, 'stroke-linecap': 'round',
      transform: 'rotate(-90 22 22)', style: 'transition: stroke-dashoffset 0.3s ease, stroke 0.3s'
    }),
  ])
}
Ring.props = ['value', 'color']

// Sparkline: 双线 SVG 折线(↑ 上行 实色 / ↓ 下行 虚线)。固定高度 32px,
// 宽度自适应父级。max 取两线最大值,缺数据时画 0。
const Sparkline: any = (props: { seriesUp: number[]; seriesDown: number[] }) => {
  const seriesUp = Array.isArray(props?.seriesUp) ? props.seriesUp : []
  const seriesDown = Array.isArray(props?.seriesDown) ? props.seriesDown : []
  const W = 200, H = 32
  const all = [...seriesUp, ...seriesDown]
  const max = Math.max(1, ...all)
  const path = (arr: number[]) => {
    if (arr.length === 0) return ''
    const step = arr.length > 1 ? W / (arr.length - 1) : 0
    return arr.map((v, i) => {
      const x = i * step
      const y = H - (Math.max(0, v) / max) * (H - 2) - 1
      return (i === 0 ? 'M' : 'L') + x.toFixed(1) + ',' + y.toFixed(1)
    }).join(' ')
  }
  return h('svg', { class: 'spark', viewBox: `0 0 ${W} ${H}`, preserveAspectRatio: 'none' }, [
    h('path', { d: path(seriesDown), fill: 'none', stroke: 'var(--el-color-primary)', 'stroke-width': 1.5, 'stroke-dasharray': '3,2', opacity: 0.7 }),
    h('path', { d: path(seriesUp), fill: 'none', stroke: 'var(--el-color-success)', 'stroke-width': 1.5 }),
  ])
}
Sparkline.props = ['seriesUp', 'seriesDown']

// ---------- network ----------
let sysTimer: ReturnType<typeof setInterval> | null = null
let liveTimer: ReturnType<typeof setInterval> | null = null

const reloadSys = async () => {
  const data = await HttpUtils.get('api/status', { r: 'cpu,mem,sys,sbd' })
  if (data.success) tilesData.value = data.obj
}

const reloadLive = async () => {
  const [live, conn] = await Promise.all([
    HttpUtils.get('api/liveTotals', { resource: 'inbound' }),
    HttpUtils.get('api/connStats'),
  ])
  if (conn.success && conn.obj) connStats.value = { tcp: conn.obj.tcp || 0, udp: conn.obj.udp || 0 }
  if (live.success && live.obj) {
    const m: Record<string, { up: number; down: number }> = live.obj
    const nowUp = Object.values(m).reduce((s, t) => s + (t.up || 0), 0)
    const nowDown = Object.values(m).reduce((s, t) => s + (t.down || 0), 0)
    const ts = Date.now()
    if (lastSample.value && ts > lastSample.value.ts) {
      const dt = (ts - lastSample.value.ts) / 1000
      rateUp.value = Math.max(0, nowUp - lastSample.value.up) / dt
      rateDown.value = Math.max(0, nowDown - lastSample.value.down) / dt
    }
    lastSample.value = { up: nowUp, down: nowDown, ts }
    upHistory.value.push(rateUp.value); if (upHistory.value.length > 60) upHistory.value.shift()
    downHistory.value.push(rateDown.value); if (downHistory.value.length > 60) downHistory.value.shift()
  }
}

const restarting = ref(false)
const restartSingbox = async () => {
  restarting.value = true
  await HttpUtils.post('api/restartSb', {})
  setTimeout(reloadSys, 1500)
  restarting.value = false
}

onMounted(async () => {
  await Promise.all([reloadSys(), reloadLive()])
  sysTimer = setInterval(() => { if (!document.hidden) reloadSys() }, 5000)
  liveTimer = setInterval(() => { if (!document.hidden) reloadLive() }, 1500)
})
onBeforeUnmount(() => {
  if (sysTimer) clearInterval(sysTimer)
  if (liveTimer) clearInterval(liveTimer)
})

const logModal = ref({ visible: false })
const backupModal = ref({ visible: false })
const usageStatsModal = ref({ visible: false })

// ---------- 检查更新 ----------
const updating = ref(false)
const updateDialog = ref<{
  visible: boolean
  loading: boolean
  error: string
  current: string
  latest: string
  latestTag: string
  hasUpdate: boolean
  publishedAt: string
  latestUrl: string
  upgradeCmd: string
}>({ visible: false, loading: false, error: '', current: '', latest: '', latestTag: '', hasUpdate: false, publishedAt: '', latestUrl: '', upgradeCmd: '' })

const checkUpdate = async () => {
  updating.value = true
  updateDialog.value = { ...updateDialog.value, visible: true, loading: true, error: '' }
  try {
    // /api/v1/* 走 Bearer token,但 panel 内部用 cookie session — 用 v0 等价
    // (走同一会话)更稳。后端我加的是 v1 路由,但前端没 token cache;
    // 用 fetch /api/v1/system/check-update 仍然能命中 — v1 中间件接受 cookie?
    // 不:v1 强制 Bearer。所以这里直接前端 fetch GitHub —— 退回原始方案,
    // CORS 对 raw.githubusercontent.com / api.github.com 是允许的。
    const r = await fetch('https://api.github.com/repos/DoBestone/nexcore-s-ui/releases?per_page=10', {
      headers: { 'Accept': 'application/vnd.github+json' }
    })
    if (!r.ok) throw new Error(i18n.global.t('main.dash.githubReturned', { status: r.status }))
    const rels: any[] = await r.json()
    const stable = rels.find(x => !x.prerelease && x.tag_name)
    if (!stable) throw new Error(i18n.global.t('main.dash.noStableRelease'))
    const latestTag: string = stable.tag_name
    const latest = latestTag.replace(/^v/, '')
    const cur = panelVersion.value
    updateDialog.value = {
      visible: true,
      loading: false,
      error: '',
      current: cur,
      latest,
      latestTag,
      hasUpdate: cmpSemver(cur, latest) < 0,
      publishedAt: stable.published_at || '',
      latestUrl: stable.html_url || '',
      upgradeCmd: 'bash <(curl -Ls https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/update.sh)',
    }
  } catch (e: any) {
    updateDialog.value = { ...updateDialog.value, loading: false, error: i18n.global.t('main.dash.checkFailed', { msg: (e?.message || e) }) }
  }
  updating.value = false
}
const copyCmd = async () => {
  try { await navigator.clipboard.writeText(updateDialog.value.upgradeCmd); ElMessage.success(i18n.global.t('main.dash.copied')) }
  catch { ElMessage.warning(i18n.global.t('main.dash.copyFailed')) }
}
const openReleasePage = () => { window.open(updateDialog.value.latestUrl || 'https://github.com/DoBestone/nexcore-s-ui/releases', '_blank') }
</script>

<style scoped>
.dashboard { display: flex; flex-direction: column; gap: 14px; }

.hero {
  display: grid;
  grid-template-columns: 1fr auto auto;
  align-items: center;
  gap: 18px;
  padding: 14px 18px;
}
.hero__brand { display: flex; align-items: center; gap: 14px; min-width: 0; }
.hero__logo { width: 40px; height: 40px; flex: 0 0 40px; }
.hero__brand-text { min-width: 0; }
.hero__title { margin: 0; font-size: 16px; font-weight: 700; color: var(--nc-text-1); }
.hero__versions { display: flex; gap: 16px; margin-top: 4px; flex-wrap: wrap; }
.hero__ver { display: inline-flex; align-items: center; gap: 6px; font-size: 12px; color: var(--nc-text-muted); }
.hero__ver-label { opacity: 0.8; }
.hero__ver-value { color: var(--nc-text-1); font-weight: 600; }
.hero__ver--core.is-stale .hero__ver-value { color: var(--el-color-warning); }
.hero__ver-warn { color: var(--el-color-warning); font-size: 13px; cursor: help; }

.hero__status {
  display: flex; align-items: center; gap: 10px;
  padding: 8px 14px; border-radius: 8px;
  background: var(--el-fill-color-light);
}
.hero__status-dot {
  width: 10px; height: 10px; border-radius: 50%;
  background: var(--el-color-info);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--el-color-info) 30%, transparent);
}
.hero__status.is-up .hero__status-dot {
  background: var(--el-color-success);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--el-color-success) 30%, transparent);
  animation: pulse 2s infinite;
}
.hero__status.is-down .hero__status-dot {
  background: var(--el-color-danger);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--el-color-danger) 30%, transparent);
}
@keyframes pulse {
  0%,100% { box-shadow: 0 0 0 3px color-mix(in srgb, var(--el-color-success) 30%, transparent); }
  50%     { box-shadow: 0 0 0 6px color-mix(in srgb, var(--el-color-success) 10%, transparent); }
}
.hero__status-text { font-size: 13px; font-weight: 600; color: var(--nc-text-1); }
.hero__status-meta { font-size: 11.5px; color: var(--nc-text-muted); margin-top: 2px; }

.hero__actions { display: flex; gap: 6px; flex-wrap: wrap; }

.kpis {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
}
.kpi-card { padding: 14px 16px; display: flex; flex-direction: column; gap: 6px; min-height: 88px; }
.kpi__label { font-size: 11.5px; color: var(--nc-text-muted); font-weight: 600; letter-spacing: 0.3px; }
.kpi__row { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.kpi__row--multi { display: grid; grid-template-columns: repeat(2, 1fr); gap: 6px 12px; }
.kpi__value { font-size: 26px; font-weight: 700; color: var(--nc-text-1); line-height: 1; font-variant-numeric: tabular-nums; }
.kpi__unit { font-size: 14px; font-weight: 500; color: var(--nc-text-muted); margin-left: 2px; }
.kpi__meta { font-size: 11.5px; color: var(--nc-text-muted); margin-top: auto; }
.kpi__rate { display: flex; gap: 14px; align-items: baseline; }
.kpi__rate-up { font-size: 17px; font-weight: 700; color: var(--el-color-success); font-variant-numeric: tabular-nums; }
.kpi__rate-down { font-size: 17px; font-weight: 700; color: var(--el-color-primary); font-variant-numeric: tabular-nums; }
.kpi__pair { display: flex; align-items: baseline; gap: 6px; }
.kpi__pair-k { font-size: 11.5px; color: var(--nc-text-muted); font-weight: 500; }
.kpi__pair-v { font-size: 16px; font-weight: 700; color: var(--nc-text-1); font-variant-numeric: tabular-nums; }

.spark { width: 100%; height: 32px; margin-top: 4px; }

.upd-row { display: flex; align-items: center; gap: 12px; padding: 6px 0; font-size: 13px; }
.upd-k { color: var(--nc-text-muted); width: 90px; flex: 0 0 90px; }
.upd-v { color: var(--nc-text-1); font-weight: 600; flex: 1; }
.upd-cmd { display: flex; align-items: center; gap: 8px; padding: 8px 10px; background: var(--el-fill-color-light); border-radius: 6px; }
.upd-cmd code { flex: 1; font-size: 12px; color: var(--nc-text-1); word-break: break-all; }
.upd-err { padding: 8px 0; }

@media (max-width: 720px) {
  .hero { grid-template-columns: 1fr; }
  .hero__actions { justify-content: flex-end; }
}
</style>
