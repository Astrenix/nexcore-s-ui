<template>
  <div class="page-container">
    <div class="page-header with-actions">
      <div class="page-header-text">
        <h2 class="page-title">{{ $t('pages.subPools') }}</h2>
        <p class="page-desc">{{ $t('subpool.desc') }}</p>
      </div>
      <div class="page-header-actions">
        <el-button @click="loadAll" :loading="loading">
          <el-icon><Refresh /></el-icon>{{ $t('subpool.refreshView') }}
        </el-button>
        <el-popconfirm
          :title="$t('subpool.resetConfirm')"
          :confirm-button-text="$t('subpool.resetConfirmBtn')"
          :cancel-button-text="$t('actions.cancel')"
          @confirm="resetAll"
          width="380"
        >
          <template #reference>
            <el-button :disabled="subs.length === 0 && pools.length === 0">
              <el-icon><Delete /></el-icon>{{ $t('subpool.cleanup') }}
            </el-button>
          </template>
        </el-popconfirm>
        <el-button type="primary" @click="openSubEdit(null)">
          <el-icon><Plus /></el-icon>{{ $t('subpool.addSub') }}
        </el-button>
      </div>
    </div>

    <div class="nc-card">
      <div class="section-title">{{ $t('subpool.subSources') }}</div>
      <el-table :data="subs" stripe>
        <el-table-column :label="$t('enable')" width="78">
          <template #default="{ row }">
            <el-switch :model-value="row.enable" @change="(val: boolean) => toggleEnable(row, val)" />
          </template>
        </el-table-column>
        <el-table-column prop="name" :label="$t('subpool.name')" min-width="160" show-overflow-tooltip />
        <el-table-column prop="url" label="URL" min-width="280" show-overflow-tooltip>
          <template #default="{ row }"><span class="mono">{{ row.url }}</span></template>
        </el-table-column>
        <el-table-column :label="$t('subpool.refreshInterval')" width="120">
          <template #default="{ row }">{{ $t('subpool.everyNMinutes', { n: row.refresh_interval }) }}</template>
        </el-table-column>
        <el-table-column :label="$t('subpool.lastSync')" width="200">
          <template #default="{ row }">
            <span v-if="row.last_synced_at && !isZeroTime(row.last_synced_at)">{{ formatTime(row.last_synced_at) }}</span>
            <span v-else class="muted">{{ $t('subpool.neverSynced') }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('subpool.status')" width="120">
          <template #default="{ row }">
            <el-tag v-if="row.last_status === 'ok'" type="success" size="small">{{ $t('subpool.okNodes', { n: row.last_node_count }) }}</el-tag>
            <el-tag v-else-if="row.last_status === 'failed'" type="danger" size="small">{{ $t('failed') }}</el-tag>
            <el-tag v-else type="info" size="small" effect="plain">{{ $t('subpool.notSynced') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('actions.action')" width="180" align="center">
          <template #default="{ row }">
            <el-tooltip :content="$t('subpool.refreshNow')" placement="top">
              <el-button text :loading="refreshingId === row.id" @click="refreshSub(row)">
                <el-icon><RefreshRight /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip :content="$t('actions.edit')" placement="top">
              <el-button text @click="openSubEdit(row)">
                <el-icon><Edit /></el-icon>
              </el-button>
            </el-tooltip>
            <el-popconfirm
              :title="$t('subpool.delSubConfirm')"
              @confirm="delSub(row.id)"
            >
              <template #reference>
                <el-button text><el-icon><Delete /></el-icon></el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="subs.length === 0 && !loading" :description="$t('subpool.emptySubs')" />
      <div v-if="lastRefresh" class="muted refresh-banner">
        <el-icon><InfoFilled /></el-icon>
        {{ $t('subpool.refreshResult') }} <b>{{ lastRefresh.parsed }}</b>/{{ lastRefresh.total }}{{ $t('subpool.probeAlive') }} <b>{{ lastRefresh.alive }}</b>
        <span v-if="lastRefresh.error" class="err"> · {{ lastRefresh.error }}</span>
      </div>
    </div>

    <div class="nc-card" style="margin-top:16px">
      <div class="section-title">{{ $t('subpool.countryPoolTitle') }}</div>
      <el-table :data="pools" stripe row-key="country" @expand-change="onExpandPool">
        <el-table-column type="expand" width="50">
          <template #default="{ row }">
            <div class="pool-detail">
              <el-table v-if="detailLoaded[row.country]" :data="detailNodes[row.country]" size="small">
                <el-table-column label="" width="36">
                  <template #default="{ row: n }">
                    <span v-if="n.alive" class="dot dot-ok" title="alive" />
                    <span v-else class="dot dot-bad" title="dead" />
                  </template>
                </el-table-column>
                <el-table-column :label="$t('subpool.nodeName')" min-width="180" show-overflow-tooltip>
                  <template #default="{ row: n }"><span>{{ n.remark || $t('subpool.noName') }}</span></template>
                </el-table-column>
                <el-table-column :label="$t('subpool.entry')" min-width="220" show-overflow-tooltip>
                  <template #default="{ row: n }"><span class="mono">{{ n.server }}:{{ n.server_port }}</span></template>
                </el-table-column>
                <el-table-column :label="$t('protocol')" width="100">
                  <template #default="{ row: n }"><span class="muted">{{ n.type }}</span></template>
                </el-table-column>
                <el-table-column :label="$t('subpool.exitIp')" width="140">
                  <template #default="{ row: n }">
                    <span v-if="n.exit_ip" class="mono">{{ n.exit_ip }}</span>
                    <span v-else class="muted">—</span>
                  </template>
                </el-table-column>
                <el-table-column :label="$t('out.delay')" width="80">
                  <template #default="{ row: n }">
                    <span v-if="n.alive" :class="latencyClass(n.latency_ms)">{{ n.latency_ms }}ms</span>
                    <span v-else class="muted">—</span>
                  </template>
                </el-table-column>
                <el-table-column :label="$t('subpool.statusFailReason')" min-width="260">
                  <template #default="{ row: n }">
                    <span v-if="n.alive" class="ok">OK</span>
                    <el-tooltip v-else :content="n.last_error || $t('subpool.notProbed')" placement="top" :show-after="200">
                      <span class="err mono">{{ shortError(n.last_error) }}</span>
                    </el-tooltip>
                  </template>
                </el-table-column>
                <el-table-column :label="$t('subpool.lastProbe')" width="170">
                  <template #default="{ row: n }">
                    <span class="muted">{{ formatTime(n.last_check_at) }}</span>
                  </template>
                </el-table-column>
              </el-table>
              <div v-else class="muted" style="padding:8px 12px">{{ $t('loading') }}</div>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="$t('subpool.country')" width="100">
          <template #default="{ row }">
            <span class="cc-badge">{{ row.country }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('subpool.outboundTag')" width="160">
          <template #default="{ row }">
            <span v-if="row.outbound_tag" class="mono">{{ row.outbound_tag }}</span>
            <span v-else class="muted">{{ $t('subpool.notCreated') }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('subpool.currentWinner')" min-width="320">
          <template #default="{ row }">
            <div v-if="row.winner">
              <div><b>{{ row.winner.remark || row.winner.server }}</b> <span class="muted">· {{ row.winner.type }}</span></div>
              <div class="muted mono">{{ row.winner.server }}:{{ row.winner.server_port }} → exit {{ row.winner.exit_ip }}</div>
            </div>
            <span v-else class="warn">{{ $t('subpool.noAvailableNode') }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('out.delay')" width="90">
          <template #default="{ row }">
            <span v-if="row.winner" :class="latencyClass(row.winner.latency_ms)">{{ row.winner.latency_ms }}ms</span>
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('subpool.poolSize')" width="110">
          <template #default="{ row }">
            <span class="muted">{{ row.alive }}/{{ row.total }} alive</span>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="pools.length === 0 && !loading" :description="$t('subpool.emptyPools')" />
    </div>

    <div class="nc-card" style="margin-top:16px">
      <div class="section-title">{{ $t('subpool.poolOutboundsTitle') }}</div>
      <el-table :data="poolOutbounds" stripe size="small">
        <el-table-column :label="$t('subpool.country')" width="100">
          <template #default="{ row }"><span class="cc-badge">{{ row.country }}</span></template>
        </el-table-column>
        <el-table-column :label="$t('subpool.outboundTag')" width="160">
          <template #default="{ row }"><span class="mono">{{ row.tag }}</span></template>
        </el-table-column>
        <el-table-column :label="$t('subpool.displayNameCol')" min-width="220">
          <template #default="{ row }">
            <span>{{ row.display_name }}</span>
            <el-button text size="small" style="margin-left:6px" @click="openEditDisplayName(row)">
              <el-icon><Edit /></el-icon>
            </el-button>
          </template>
        </el-table-column>
        <el-table-column :label="$t('protocol')" width="100">
          <template #default="{ row }"><span class="muted">{{ row.type }}</span></template>
        </el-table-column>
        <el-table-column :label="$t('subpool.currentWinnerNode')" width="120">
          <template #default="{ row }">
            <span v-if="row.winner_node_id" class="mono">#{{ row.winner_node_id }}</span>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('subpool.winnerLatency')" width="120">
          <template #default="{ row }">
            <span v-if="row.winner_latency" :class="latencyClass(row.winner_latency)">{{ row.winner_latency }}ms</span>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('subpool.lastUpdate')" width="180">
          <template #default="{ row }"><span class="muted">{{ formatTime(row.updated_at) }}</span></template>
        </el-table-column>
      </el-table>
      <el-empty v-if="poolOutbounds.length === 0 && !loading" :description="$t('subpool.emptyPoolOutbounds')" />
      <div class="muted" style="font-size:12px; margin-top:8px">
        {{ $t('subpool.poolOutboundsHint') }}
      </div>
    </div>

    <!-- 编辑订阅池出站「显示名」-->
    <el-dialog v-model="poolDisplayDialog" :title="$t('subpool.editDisplayNameTitle')" width="480px">
      <el-form :model="editingPool" label-width="100px">
        <el-form-item :label="$t('subpool.country')">
          <span class="cc-badge">{{ editingPool.country }}</span>
          <span class="mono muted" style="margin-left:8px">{{ editingPool.tag }}</span>
        </el-form-item>
        <el-form-item :label="$t('subpool.displayName')">
          <el-input v-model="editingPool.display_name" :placeholder="$t('subpool.displayNamePlaceholder')" />
          <div class="muted" style="font-size:12px; margin-top:4px">
            {{ $t('subpool.displayNameHint') }}<code>{{ $t('subpool.displayNameFormat') }}</code>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="poolDisplayDialog = false">{{ $t('actions.cancel') }}</el-button>
        <el-button type="primary" :loading="savingPool" @click="savePoolDisplayName">{{ $t('actions.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 订阅新增/编辑 -->
    <el-dialog v-model="subDialog" :title="editing.id ? $t('subpool.editSubTitle') : $t('subpool.addSubTitle')" width="540px">
      <el-form :model="editing" label-width="100px">
        <el-form-item :label="$t('subpool.name')">
          <el-input v-model="editing.name" :placeholder="$t('subpool.namePlaceholder')" />
        </el-form-item>
        <el-form-item :label="$t('subpool.subUrl')">
          <el-input v-model="editing.url" placeholder="https://api.example.com/subscribe?token=..." />
        </el-form-item>
        <el-form-item :label="$t('subpool.refreshIntervalMin')">
          <el-input-number v-model="editing.refresh_interval" :min="0" :max="1440" :step="10" />
          <span class="muted" style="margin-left:8px">{{ $t('subpool.manualOnly') }}</span>
        </el-form-item>
        <el-form-item :label="$t('enable')">
          <el-switch v-model="editing.enable" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="subDialog = false">{{ $t('actions.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="saveSub">{{ $t('actions.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script lang="ts" setup>
import { ref, reactive, onMounted } from 'vue'
import HttpUtils from '@/plugins/httputil'
import { ElMessage } from 'element-plus'
import { Refresh, RefreshRight, Plus, Edit, Delete, InfoFilled } from '@element-plus/icons-vue'
import { i18n } from '@/locales'

interface Sub {
  id?: number
  name: string
  url: string
  enable: boolean
  refresh_interval: number
  last_synced_at?: string
  last_status?: string
  last_error?: string
  last_node_count?: number
}

interface WinnerRow {
  id: number
  remark: string
  type: string
  server: string
  server_port: number
  exit_ip: string
  latency_ms: number
}

interface PoolRow {
  country: string
  total: number
  alive: number
  winner?: WinnerRow
  outbound_id?: number
  outbound_tag?: string
}

interface SubNode {
  id: number
  remark: string
  type: string
  server: string
  server_port: number
  exit_ip: string
  latency_ms: number
  alive: boolean
  last_error: string
  last_check_at: string
}

interface PoolOutbound {
  id: number
  tag: string
  country: string
  type: string
  display_name: string
  winner_node_id: number
  winner_latency: number
  updated_at: string
}

const subs = ref<Sub[]>([])
const pools = ref<PoolRow[]>([])
const poolOutbounds = ref<PoolOutbound[]>([])
const loading = ref(false)
const subDialog = ref(false)
const saving = ref(false)
const refreshingId = ref<number | null>(null)
const lastRefresh = ref<any>(null)
const poolDisplayDialog = ref(false)
const savingPool = ref(false)
const editingPool = ref<PoolOutbound>({} as PoolOutbound)

// 展开行的节点明细缓存:country → 节点列表;detailLoaded 用于显示 loading 占位
const detailNodes = reactive<Record<string, SubNode[]>>({})
const detailLoaded = reactive<Record<string, boolean>>({})

const editing = ref<Sub>(emptySub())

function emptySub(): Sub {
  return { name: '', url: '', enable: true, refresh_interval: 60 }
}

const isZeroTime = (s: string) => !s || s.startsWith('0001-01-01')

const formatTime = (s: string) => {
  if (!s) return '—'
  try {
    return new Date(s).toLocaleString()
  } catch { return s }
}

const latencyClass = (ms: number) => {
  if (ms < 100) return 'lat-good'
  if (ms < 300) return 'lat-mid'
  return 'lat-bad'
}

// 失败原因往往很长(完整 Go err 字符串带堆栈/包路径),表格里截短显示,tooltip 看全文
const shortError = (err: string) => {
  if (!err) return i18n.global.t('subpool.notProbed')
  // 命中常见错误关键词,转更易懂的简短提示
  if (/i\/o timeout/.test(err)) return i18n.global.t('subpool.errTcpTimeout')
  if (/EOF/.test(err)) return i18n.global.t('subpool.errEof')
  if (/connection refused/.test(err)) return i18n.global.t('subpool.errRefused')
  if (/no route to host/.test(err)) return i18n.global.t('subpool.errNoRoute')
  if (/tls/i.test(err) && /handshake/i.test(err)) return i18n.global.t('subpool.errTlsHandshake')
  if (/http 4\d\d/i.test(err)) return err.match(/http \d+/)?.[0] ?? i18n.global.t('subpool.errHttp')
  if (/http 5\d\d/i.test(err)) return err.match(/http \d+/)?.[0] ?? i18n.global.t('subpool.errHttp')
  if (/ctx done/.test(err)) return i18n.global.t('subpool.errTimeout')
  // fallback:取最后一段(常是 root cause)
  return err.length > 60 ? err.slice(-60) : err
}

// 展开某国家行 → 拉该国全部节点
const onExpandPool = async (row: PoolRow, expandedRows: PoolRow[]) => {
  const open = expandedRows.some(r => r.country === row.country)
  if (!open) return
  if (detailLoaded[row.country]) return // 已加载过,沿用缓存
  detailLoaded[row.country] = false
  try {
    const r = await HttpUtils.get(`api/subNodes?country=${row.country}`)
    if (r.success) {
      detailNodes[row.country] = (r.obj || []) as SubNode[]
    } else {
      detailNodes[row.country] = []
    }
  } finally {
    detailLoaded[row.country] = true
  }
}

const loadAll = async () => {
  loading.value = true
  try {
    const [r1, r2, r3] = await Promise.all([
      HttpUtils.get('api/subs'),
      HttpUtils.get('api/subPools'),
      HttpUtils.get('api/poolOutbounds'),
    ])
    if (r1.success) subs.value = r1.obj || []
    if (r2.success) pools.value = r2.obj || []
    if (r3.success) poolOutbounds.value = r3.obj || []
  } finally {
    loading.value = false
  }
}

const openSubEdit = (row: Sub | null) => {
  editing.value = row ? { ...row } : emptySub()
  subDialog.value = true
}

const saveSub = async () => {
  if (!editing.value.url.trim()) { ElMessage.error(i18n.global.t('subpool.urlRequired')); return }
  saving.value = true
  try {
    const fd = new FormData()
    Object.entries(editing.value).forEach(([k, v]) => fd.append(k, String(v ?? '')))
    const res = await HttpUtils.post('api/subSave', fd as any)
    if (res.success) {
      ElMessage.success(editing.value.id ? i18n.global.t('subpool.updated') : i18n.global.t('subpool.added'))
      subDialog.value = false
      await loadAll()
    }
  } finally {
    saving.value = false
  }
}

const delSub = async (id: number) => {
  const fd = new FormData(); fd.append('id', String(id))
  const res = await HttpUtils.post('api/subDelete', fd as any)
  if (res.success) {
    // 后端 Delete 已级联清节点 + 重选 + 删孤儿 pool;前端只需重拉视图
    ElMessage.success(i18n.global.t('subpool.delSubSuccess'))
    await loadAll()
  }
}

const resetAll = async () => {
  const res = await HttpUtils.post('api/poolReset', {} as any)
  if (res.success) {
    ElMessage.success(i18n.global.t('subpool.resetSuccess'))
    await loadAll()
  } else {
    ElMessage.error(i18n.global.t('subpool.resetFailed') + (res.msg || ''))
  }
}

const toggleEnable = async (row: Sub, val: boolean) => {
  const fd = new FormData()
  Object.entries({ ...row, enable: val }).forEach(([k, v]) => fd.append(k, String(v ?? '')))
  const res = await HttpUtils.post('api/subSave', fd as any)
  if (res.success) { row.enable = val } else { ElMessage.error(i18n.global.t('subpool.toggleFailed')) }
}

const refreshSub = async (row: Sub) => {
  refreshingId.value = row.id ?? null
  lastRefresh.value = null
  try {
    const fd = new FormData(); fd.append('id', String(row.id))
    const res = await HttpUtils.post('api/subRefresh', fd as any)
    if (res.success && res.obj) {
      lastRefresh.value = res.obj
      ElMessage.success(i18n.global.t('subpool.refreshDone', { alive: res.obj.alive, parsed: res.obj.parsed }))
    } else {
      ElMessage.error(i18n.global.t('subpool.refreshFailed') + (res.msg || i18n.global.t('subpool.unknown')))
    }
    await loadAll()
  } finally {
    refreshingId.value = null
  }
}

const openEditDisplayName = (row: PoolOutbound) => {
  editingPool.value = { ...row }
  poolDisplayDialog.value = true
}

const savePoolDisplayName = async () => {
  if (!editingPool.value.id) return
  savingPool.value = true
  try {
    const fd = new FormData()
    fd.append('id', String(editingPool.value.id))
    fd.append('display_name', editingPool.value.display_name || '')
    const res = await HttpUtils.post('api/poolOutboundSave', fd as any)
    if (res.success) {
      ElMessage.success(i18n.global.t('subpool.displayNameSaved'))
      poolDisplayDialog.value = false
      await loadAll()
    }
  } finally {
    savingPool.value = false
  }
}

onMounted(loadAll)
</script>

<style scoped>
.section-title {
  font-size: 14px;
  font-weight: 600;
  margin: 0 0 12px;
  color: var(--nc-text-strong);
}
.mono { font-family: ui-monospace, 'SF Mono', Menlo, Consolas, monospace; font-size: 12px; }
.muted { color: var(--nc-text-muted); }
.warn { color: #e6a23c; }
.err { color: #f56c6c; }
.cc-badge {
  display: inline-block;
  padding: 2px 10px;
  background: var(--nc-primary-50, #eff6ff);
  color: var(--nc-primary, #3b82f6);
  border-radius: 4px;
  font-weight: 600;
  font-size: 12px;
}
.lat-good { color: #67c23a; font-weight: 600; }
.lat-mid  { color: #e6a23c; font-weight: 600; }
.lat-bad  { color: #f56c6c; font-weight: 600; }
.refresh-banner {
  margin-top: 8px;
  font-size: 12px;
  display: flex; align-items: center; gap: 6px;
}
.pool-detail {
  background: var(--nc-bg-soft, #f8fafc);
  padding: 4px 12px 12px;
  border-radius: 6px;
}
.dot {
  display: inline-block;
  width: 8px; height: 8px; border-radius: 50%;
  margin-left: 8px;
}
.dot-ok  { background: #67c23a; }
.dot-bad { background: #c0c4cc; }
.ok { color: #67c23a; font-weight: 600; }
</style>
