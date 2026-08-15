<template>
  <div class="page-container">
    <div class="page-header with-actions">
      <div class="page-header-text">
        <h2 class="page-title">{{ $t('pages.outbounds') }}</h2>
        <p class="page-desc">{{ $t('outbounds.desc') }}</p>
      </div>
      <div class="page-header-actions">
        <el-button type="primary" @click="showModal(0)">
          <el-icon><Plus /></el-icon>{{ $t('actions.add') }}
        </el-button>
        <el-button :loading="testingAll" :disabled="outbounds.length === 0" @click="checkAllOutbounds">
          <el-icon><Stopwatch /></el-icon>{{ $t('actions.testAll') }}
        </el-button>
        <el-button :loading="refreshing" @click="refresh">
          <el-icon><RefreshRight /></el-icon>{{ $t('actions.refresh') }}
        </el-button>
      </div>
    </div>

    <div class="ob-toolbar nc-card">
      <el-input
        v-model="filter"
        :placeholder="$t('actions.search')"
        clearable
        class="ob-toolbar__search"
      >
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <div class="ob-toolbar__stats">
        <span class="ob-stat"><span class="ob-stat__num">{{ outbounds.length }}</span>{{ $t('outbounds.total') }}</span>
        <span v-if="passCount > 0" class="ob-stat ob-stat--ok">
          <el-icon><CircleCheck /></el-icon>{{ passCount }} {{ $t('outbounds.pass') }}
        </span>
        <span v-if="failCount > 0" class="ob-stat ob-stat--err">
          <el-icon><CircleClose /></el-icon>{{ failCount }} {{ $t('outbounds.fail') }}
        </span>
        <span v-if="hiddenSysCount > 0" class="ob-stat ob-stat--muted">
          {{ $t('outbounds.hiddenSys', { n: hiddenSysCount }) }}
        </span>
        <el-tooltip placement="top" :content="autoProbe ? $t('outbounds.autoProbeOn', { interval: PROBE_INTERVAL_MS / 1000, next: nextProbeIn }) : $t('outbounds.autoProbeOff')">
          <span class="ob-stat ob-stat--switch">
            <el-switch v-model="autoProbe" size="small" />
            <span style="margin-left: 6px">{{ $t('outbounds.autoProbe') }}</span>
            <span v-if="autoProbe" class="ob-probe-tick mono">{{ nextProbeIn }}s</span>
          </span>
        </el-tooltip>
        <el-tooltip placement="top" :content="$t('outbounds.sysOutboundTip')">
          <span class="ob-stat ob-stat--switch">
            <el-switch v-model="showSystem" size="small" />
            <span style="margin-left: 6px">{{ $t('outbounds.showSysOutbound') }}</span>
          </span>
        </el-tooltip>
      </div>
    </div>

    <div v-if="outbounds.length === 0" class="empty-state nc-card">
      <el-empty :description="$t('noData')">
        <el-button type="primary" @click="showModal(0)">
          <el-icon><Plus /></el-icon>{{ $t('actions.add') }}
        </el-button>
      </el-empty>
    </div>

    <el-table
      v-else
      :data="filteredOutbounds"
      :row-key="rowKey"
      stripe
      size="small"
      class="nc-table ob-table"
      :empty-text="$t('noData')"
    >
      <el-table-column :label="$t('type')" width="120">
        <template #default="{ row }">
          <span class="proto-pill" :class="`proto-${row.type}`">{{ row.type }}</span>
        </template>
      </el-table-column>

      <el-table-column :label="$t('objects.tag')" min-width="160" show-overflow-tooltip>
        <template #default="{ row }">
          <span class="mono ob-tag">{{ row.tag }}</span>
        </template>
      </el-table-column>

      <el-table-column :label="$t('outbounds.relayName')" min-width="160" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.display_name" class="ob-display-name">{{ row.display_name }}</span>
          <span v-else class="ob-muted">—</span>
        </template>
      </el-table-column>

      <el-table-column :label="$t('out.addr')" min-width="220" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.server" class="mono">
            {{ row.server }}<span class="ob-port">:{{ row.server_port ?? '?' }}</span>
          </span>
          <span v-else class="ob-muted">—</span>
        </template>
      </el-table-column>

      <el-table-column :label="$t('objects.tls')" width="80" align="center">
        <template #default="{ row }">
          <el-tag
            v-if="Object.hasOwn(row, 'tls')"
            :type="row.tls?.enabled ? 'success' : 'info'"
            size="small"
            effect="plain"
          >
            {{ row.tls?.enabled ? 'ON' : 'OFF' }}
          </el-tag>
          <span v-else class="ob-muted">—</span>
        </template>
      </el-table-column>

      <el-table-column :label="$t('out.delay')" width="110" align="center">
        <template #default="{ row }">
          <!-- 整个 cell 可点 = 重测;loading 时禁点。原"测试"按钮收编进延迟值,
               腾出操作列空间给真正的操作 -->
          <el-tooltip
            :content="checkResults[row.tag]?.loading ? $t('inbounds.probing') : (checkResults[row.tag]?.errorMessage || $t('outbounds.clickRetest'))"
            placement="top"
          >
            <span class="delay-cell" :class="{ 'is-clickable': !checkResults[row.tag]?.loading }" @click="!checkResults[row.tag]?.loading && checkOutbound(row.tag)">
              <el-icon v-if="checkResults[row.tag]?.loading" class="is-loading"><Loading /></el-icon>
              <template v-else-if="checkResults[row.tag]">
                <el-tag
                  v-if="checkResults[row.tag].success"
                  :type="latencyType(checkResults[row.tag].data?.Delay ?? 0)"
                  size="small"
                  effect="plain"
                  class="mono"
                >
                  {{ checkResults[row.tag].data?.Delay }} ms
                </el-tag>
                <el-icon v-else style="color: var(--nc-danger)"><CircleClose /></el-icon>
              </template>
              <el-icon v-else><Stopwatch /></el-icon>
            </span>
          </el-tooltip>
        </template>
      </el-table-column>

      <el-table-column :label="$t('actions.action')" width="140" align="center">
        <template #default="{ row }">
          <div class="ob-actions">
            <el-tooltip v-if="Data().enableTraffic" :content="$t('stats.graphTitle')" placement="top">
              <el-button text size="small" @click="showStats(row.tag)">
                <el-icon><DataLine /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip :content="$t('actions.edit')" placement="top">
              <el-button text size="small" @click="showModal(row.id)">
                <el-icon style="color: var(--nc-primary)"><Edit /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip :content="$t('actions.del')" placement="top">
              <el-popconfirm
                :title="$t('confirm')"
                :confirm-button-text="$t('yes')"
                :cancel-button-text="$t('no')"
                @confirm="delOutbound(row.tag)"
              >
                <template #reference>
                  <el-button text size="small">
                    <el-icon style="color: var(--nc-danger)"><Delete /></el-icon>
                  </el-button>
                </template>
              </el-popconfirm>
            </el-tooltip>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <OutboundVue
      v-model="modal.visible"
      :visible="modal.visible"
      :id="modal.id"
      :data="modal.data"
      :tags="outboundTags"
      @close="closeModal"
    />
    <Stats
      v-model="stats.visible"
      :visible="stats.visible"
      :resource="stats.resource"
      :tag="stats.tag"
      @close="closeStats"
    />
  </div>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import HttpUtils from '@/plugins/httputil'
import { Outbound } from '@/types/outbounds'
import { computed, defineAsyncComponent, onBeforeUnmount, onMounted, ref, watch } from 'vue'

const OutboundVue = defineAsyncComponent(() => import('@/layouts/modals/Outbound.vue'))
const Stats = defineAsyncComponent(() => import('@/layouts/modals/Stats.vue'))
import {
  Plus, Edit, Delete, DataLine, Stopwatch, Loading, CircleClose, CircleCheck,
  RefreshRight, Search,
} from '@element-plus/icons-vue'

interface CheckResult {
  loading?: boolean
  success: boolean
  data?: { OK?: boolean; Delay?: number; Error?: string } | null
  errorMessage?: string
}

const checkResults = ref<Record<string, CheckResult>>({})

// 静默更新:in-place 改字段(reactivity 只重 evaluate 引用了对应 tag 的
// computed),不替换 checkResults 整对象 —— 否则每 10s 整个 table 都会
// re-render 一次,体感"在跳"。
const setCheckResult = (tag: string, r: CheckResult) => {
  checkResults.value[tag] = r
}

const checkOutbound = async (tag: string) => {
  setCheckResult(tag, { loading: true, success: false })
  const msg = await HttpUtils.get('api/checkOutbound', { tag })
  const success = msg.success && msg.obj?.OK
  const errorMessage = success ? undefined : (msg.obj?.Error ?? msg.msg ?? '')
  setCheckResult(tag, { loading: false, success, data: msg.obj ?? null, errorMessage })
}

// el-table 用稳定 key 做 row diff,id 没有就用 tag,避免 10s 一轮全 unmount
const rowKey = (row: any) => row?.id ?? row?.tag ?? ''

const testingAll = ref(false)
const checkAllOutbounds = async () => {
  const list = outbounds.value
  if (list.length === 0) return
  testingAll.value = true
  try {
    await Promise.all(list.map((o) => checkOutbound(o.tag)))
  } finally {
    testingAll.value = false
  }
}

const refreshing = ref(false)
const refresh = async () => {
  refreshing.value = true
  try {
    Data().lastLoad = 0
    await Data().loadData()
  } finally {
    refreshing.value = false
  }
}

const filter = ref('')
const outbounds = computed((): Outbound[] => <Outbound[]>(Data().outbounds ?? []))

// 系统出站(direct / block / dns)是面板自动补的"内置出口",被 rule_set
// download_detour 和路由直连规则隐式引用。它们跟用户的代理节点同框露脸
// 容易混淆,默认隐藏;开关存 localStorage 跨刷新保留。
const SYSTEM_OB_TYPES = ['direct', 'block', 'dns']
const SHOW_SYS_KEY = 'outbounds.showSystem'
const showSystem = ref<boolean>(localStorage.getItem(SHOW_SYS_KEY) === '1')
watch(showSystem, (v) => localStorage.setItem(SHOW_SYS_KEY, v ? '1' : '0'))

const isSystemOb = (o: any) => SYSTEM_OB_TYPES.includes(o?.type)
const hiddenSysCount = computed(() => showSystem.value ? 0 : outbounds.value.filter(isSystemOb).length)

const filteredOutbounds = computed(() => {
  const base = showSystem.value ? outbounds.value : outbounds.value.filter((o: any) => !isSystemOb(o))
  const q = filter.value.trim().toLowerCase()
  if (!q) return base
  return base.filter((o: any) =>
    (o.tag || '').toLowerCase().includes(q) ||
    (o.type || '').toLowerCase().includes(q) ||
    (o.server || '').toLowerCase().includes(q),
  )
})
const outboundTags = computed((): string[] => [
  ...(Data().outbounds?.map((o: Outbound) => o.tag) ?? []),
  ...(Data().endpoints?.map((e: any) => e.tag) ?? []),
])
// 注:出站场景不再展示 onlines —— 它表达"近期有流量",不是"能连通",
// 在添加测试节点时全部显示离线但其实能用,严重误导。延迟列(checkOutbound)
// 才是真实的"能拨通 + 密钥对 + 协议握手成功"。入站列表才该显示在线。
const passCount = computed(() =>
  Object.values(checkResults.value).filter((r) => !r.loading && r.success).length,
)
const failCount = computed(() =>
  Object.values(checkResults.value).filter((r) => !r.loading && !r.success && r.data !== undefined).length,
)

const latencyType = (ms: number): 'success' | 'warning' | 'danger' => {
  if (ms < 200) return 'success'
  if (ms < 600) return 'warning'
  return 'danger'
}

const modal = ref({ visible: false, id: 0, data: '' })
const showModal = (id: number) => {
  modal.value.id = id
  modal.value.data = id == 0 ? '' : JSON.stringify(outbounds.value.findLast((o) => o.id == id))
  modal.value.visible = true
}
const closeModal = () => { modal.value.visible = false }

const stats = ref({ visible: false, resource: 'outbound', tag: '' })
const delOutbound = async (tag: string) => {
  await Data().save('outbounds', 'del', tag)
}
const showStats = (tag: string) => {
  stats.value.tag = tag
  stats.value.visible = true
}
const closeStats = () => { stats.value.visible = false }

// ---------- 自动探测(在线状态 + 延迟,10s/轮) ----------
// 每轮:
//   1) 触发增量 loadData → 刷 onlines(后端 push 在线列表)
//   2) 对当前可见出站(filteredOutbounds)发 checkOutbound,4 路并发上限
//      免得机场上百节点一次打爆 sing-box dial pool。
// 暂停:autoProbe=false / 页面卸载 / 上一轮还没完(避免重叠)
const PROBE_INTERVAL_MS = 10000
const PROBE_CONCURRENCY = 4
const PROBE_KEY = 'outbounds.autoProbe'
const autoProbe = ref<boolean>(localStorage.getItem(PROBE_KEY) !== '0')
watch(autoProbe, (v) => localStorage.setItem(PROBE_KEY, v ? '1' : '0'))

const nextProbeIn = ref(PROBE_INTERVAL_MS / 1000)
let probeRunning = false
let probeTimer: number | null = null
let tickTimer: number | null = null

const probeOnce = async () => {
  if (probeRunning) return
  probeRunning = true
  try {
    // 只测延迟 — 不再调 Data().loadData()(那会整体替换 outbounds 数组,
    // 触发 el-table 全列重渲染,体感"在跳")。延迟探测走 in-place update,
    // table 行不会重建。
    const targets = filteredOutbounds.value.filter((o: any) => o?.tag && !checkResults.value[o.tag]?.loading)
    if (targets.length === 0) return
    // 4 路并发滑窗,免一次性 100+ dial 打爆 sing-box
    let i = 0
    const next = async () => {
      while (i < targets.length) {
        const idx = i++
        await checkOutbound(targets[idx].tag)
      }
    }
    await Promise.all(Array.from({ length: Math.min(PROBE_CONCURRENCY, targets.length) }, () => next()))
  } finally {
    probeRunning = false
  }
}

const startProbe = () => {
  stopProbe()
  // 立刻先来一轮,免得用户进页面要等 10s 才有数据
  probeOnce()
  nextProbeIn.value = PROBE_INTERVAL_MS / 1000
  probeTimer = window.setInterval(() => {
    probeOnce()
    nextProbeIn.value = PROBE_INTERVAL_MS / 1000
  }, PROBE_INTERVAL_MS)
  tickTimer = window.setInterval(() => {
    if (nextProbeIn.value > 0) nextProbeIn.value -= 1
  }, 1000)
}

const stopProbe = () => {
  if (probeTimer !== null) { window.clearInterval(probeTimer); probeTimer = null }
  if (tickTimer !== null) { window.clearInterval(tickTimer); tickTimer = null }
}

watch(autoProbe, (v) => { v ? startProbe() : stopProbe() })

onMounted(() => {
  if (autoProbe.value) startProbe()
})
onBeforeUnmount(stopProbe)
</script>

<style scoped>
.ob-toolbar {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 10px 14px;
  margin-bottom: 12px;
}
.ob-toolbar__search { max-width: 320px; }
.ob-toolbar__stats {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-left: auto;
  font-size: 12.5px;
  color: var(--nc-text-muted);
}
.ob-stat {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.ob-stat__num {
  font-family: var(--font-display);
  font-size: 14px;
  font-weight: 600;
  color: var(--nc-text-1);
  margin-right: 4px;
}
.ob-stat--ok { color: var(--nc-success); }
.ob-stat--err { color: var(--nc-danger); }
.ob-stat--muted { color: var(--nc-text-muted); font-style: italic; }
.ob-stat--switch { display: inline-flex; align-items: center; cursor: help; }
.ob-probe-tick { margin-left: 6px; font-size: 11px; color: var(--nc-text-muted); min-width: 28px; text-align: right; }

/* 延迟 cell:整格可点触发重测 */
.delay-cell {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 56px;
  height: 24px;
  padding: 0 4px;
  border-radius: var(--radius-sm);
  color: var(--nc-text-muted);
}
.delay-cell.is-clickable { cursor: pointer; transition: background-color 0.12s; }
.delay-cell.is-clickable:hover { background-color: var(--nc-primary-soft); color: var(--nc-primary); }

/* 操作组紧凑 */
.ob-actions { display: inline-flex; gap: 2px; justify-content: center; }
.ob-actions .el-button { padding: 4px 6px !important; height: auto !important; }

.ob-table { background: var(--nc-surface); }
.ob-table :deep(.el-table__row) td { vertical-align: middle; }

.proto-pill {
  display: inline-block;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: var(--radius-pill);
  letter-spacing: 0.04em;
  text-transform: uppercase;
  background: var(--nc-primary-soft);
  color: var(--nc-primary);
}
.proto-pill.proto-direct { color: #475569; background: #e2e8f0; }
.proto-pill.proto-block  { color: #dc2626; background: #fee2e2; }
.proto-pill.proto-dns    { color: #7c3aed; background: #ede9fe; }
.proto-pill.proto-vless,
.proto-pill.proto-vmess  { color: #2563eb; background: #dbeafe; }
.proto-pill.proto-trojan { color: #d97706; background: #fef3c7; }
.proto-pill.proto-shadowsocks { color: #16a34a; background: #dcfce7; }
.proto-pill.proto-socks,
.proto-pill.proto-http   { color: #0d9488; background: #ccfbf1; }

.ob-tag { color: var(--nc-text-1); font-weight: 500; }
.ob-port { color: var(--nc-text-muted); font-weight: 400; }
.ob-muted { color: var(--nc-text-faint); }
.ob-display-name { color: var(--nc-text-1); }

.status-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--nc-text-faint);
}
.status-dot.online {
  background: var(--nc-success);
  box-shadow: 0 0 0 2px rgba(34, 197, 94, 0.18);
}

.empty-state {
  padding: 40px 16px;
}
</style>
