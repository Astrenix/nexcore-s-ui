<template>
  <div class="page-container">
    <div class="page-header with-actions">
      <div class="page-header-text">
        <h2 class="page-title">{{ $t('pages.subPools') }}</h2>
        <p class="page-desc">导入机场订阅链接,系统自动探测每条节点的真落地 IP + 延迟,按国家分组,只挑最快一条作为出站 winner;入站可直接绑「订阅池(选国家)」,节点更新对入站透明。</p>
      </div>
      <div class="page-header-actions">
        <el-button @click="loadAll" :loading="loading">
          <el-icon><Refresh /></el-icon>刷新视图
        </el-button>
        <el-button type="primary" @click="openSubEdit(null)">
          <el-icon><Plus /></el-icon>新增订阅
        </el-button>
      </div>
    </div>

    <div class="nc-card">
      <div class="section-title">订阅源</div>
      <el-table :data="subs" stripe>
        <el-table-column label="启用" width="78">
          <template #default="{ row }">
            <el-switch :model-value="row.enable" @change="(val: boolean) => toggleEnable(row, val)" />
          </template>
        </el-table-column>
        <el-table-column prop="name" label="名称" min-width="160" show-overflow-tooltip />
        <el-table-column prop="url" label="URL" min-width="280" show-overflow-tooltip>
          <template #default="{ row }"><span class="mono">{{ row.url }}</span></template>
        </el-table-column>
        <el-table-column label="刷新周期" width="120">
          <template #default="{ row }">每 {{ row.refresh_interval }} 分钟</template>
        </el-table-column>
        <el-table-column label="上次同步" width="200">
          <template #default="{ row }">
            <span v-if="row.last_synced_at && !isZeroTime(row.last_synced_at)">{{ formatTime(row.last_synced_at) }}</span>
            <span v-else class="muted">从未同步</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag v-if="row.last_status === 'ok'" type="success" size="small">OK · {{ row.last_node_count }} 节点</el-tag>
            <el-tag v-else-if="row.last_status === 'failed'" type="danger" size="small">失败</el-tag>
            <el-tag v-else type="info" size="small" effect="plain">未同步</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" align="center">
          <template #default="{ row }">
            <el-tooltip content="立即刷新(同步探测,可能耗时 30s+)" placement="top">
              <el-button text :loading="refreshingId === row.id" @click="refreshSub(row)">
                <el-icon><RefreshRight /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip content="编辑" placement="top">
              <el-button text @click="openSubEdit(row)">
                <el-icon><Edit /></el-icon>
              </el-button>
            </el-tooltip>
            <el-popconfirm
              title="删除订阅 + 节点池里这家的全部节点?(已选 winner 的国家会重选,如池空则保留旧 winner)"
              @confirm="delSub(row.id)"
            >
              <template #reference>
                <el-button text><el-icon><Delete /></el-icon></el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="subs.length === 0 && !loading" description="还没有订阅源 — 点「新增订阅」从机场链接导入" />
      <div v-if="lastRefresh" class="muted refresh-banner">
        <el-icon><InfoFilled /></el-icon>
        刷新结果:解析 <b>{{ lastRefresh.parsed }}</b>/{{ lastRefresh.total }},探测存活 <b>{{ lastRefresh.alive }}</b>
        <span v-if="lastRefresh.error" class="err"> · {{ lastRefresh.error }}</span>
      </div>
    </div>

    <div class="nc-card" style="margin-top:16px">
      <div class="section-title">国家池(winner 选举状态)— 点行展开看全部节点</div>
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
                <el-table-column label="节点名" min-width="180" show-overflow-tooltip>
                  <template #default="{ row: n }"><span>{{ n.remark || '(无名)' }}</span></template>
                </el-table-column>
                <el-table-column label="入口" min-width="220" show-overflow-tooltip>
                  <template #default="{ row: n }"><span class="mono">{{ n.server }}:{{ n.server_port }}</span></template>
                </el-table-column>
                <el-table-column label="协议" width="100">
                  <template #default="{ row: n }"><span class="muted">{{ n.type }}</span></template>
                </el-table-column>
                <el-table-column label="落地 IP" width="140">
                  <template #default="{ row: n }">
                    <span v-if="n.exit_ip" class="mono">{{ n.exit_ip }}</span>
                    <span v-else class="muted">—</span>
                  </template>
                </el-table-column>
                <el-table-column label="延迟" width="80">
                  <template #default="{ row: n }">
                    <span v-if="n.alive" :class="latencyClass(n.latency_ms)">{{ n.latency_ms }}ms</span>
                    <span v-else class="muted">—</span>
                  </template>
                </el-table-column>
                <el-table-column label="状态 / 失败原因" min-width="260">
                  <template #default="{ row: n }">
                    <span v-if="n.alive" class="ok">OK</span>
                    <el-tooltip v-else :content="n.last_error || '未探测'" placement="top" :show-after="200">
                      <span class="err mono">{{ shortError(n.last_error) }}</span>
                    </el-tooltip>
                  </template>
                </el-table-column>
                <el-table-column label="最后探测" width="170">
                  <template #default="{ row: n }">
                    <span class="muted">{{ formatTime(n.last_check_at) }}</span>
                  </template>
                </el-table-column>
              </el-table>
              <div v-else class="muted" style="padding:8px 12px">加载中…</div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="国家" width="100">
          <template #default="{ row }">
            <span class="cc-badge">{{ row.country }}</span>
          </template>
        </el-table-column>
        <el-table-column label="出站 tag" width="160">
          <template #default="{ row }">
            <span v-if="row.outbound_tag" class="mono">{{ row.outbound_tag }}</span>
            <span v-else class="muted">未创建</span>
          </template>
        </el-table-column>
        <el-table-column label="当前 Winner" min-width="320">
          <template #default="{ row }">
            <div v-if="row.winner">
              <div><b>{{ row.winner.remark || row.winner.server }}</b> <span class="muted">· {{ row.winner.type }}</span></div>
              <div class="muted mono">{{ row.winner.server }}:{{ row.winner.server_port }} → exit {{ row.winner.exit_ip }}</div>
            </div>
            <span v-else class="warn">⚠ 池里无可用节点</span>
          </template>
        </el-table-column>
        <el-table-column label="延迟" width="90">
          <template #default="{ row }">
            <span v-if="row.winner" :class="latencyClass(row.winner.latency_ms)">{{ row.winner.latency_ms }}ms</span>
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column label="池规模" width="110">
          <template #default="{ row }">
            <span class="muted">{{ row.alive }}/{{ row.total }} alive</span>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="pools.length === 0 && !loading" description="还没有国家池 — 添加订阅并刷新后,国家会自动出现" />
    </div>

    <div class="nc-card" style="margin-top:16px">
      <div class="section-title">订阅池出站 — 协议字段自动维护,「显示名」(中转名称)可改</div>
      <el-table :data="poolOutbounds" stripe size="small">
        <el-table-column label="国家" width="100">
          <template #default="{ row }"><span class="cc-badge">{{ row.country }}</span></template>
        </el-table-column>
        <el-table-column label="出站 tag" width="160">
          <template #default="{ row }"><span class="mono">{{ row.tag }}</span></template>
        </el-table-column>
        <el-table-column label="显示名(中转名称)" min-width="220">
          <template #default="{ row }">
            <span>{{ row.display_name }}</span>
            <el-button text size="small" style="margin-left:6px" @click="openEditDisplayName(row)">
              <el-icon><Edit /></el-icon>
            </el-button>
          </template>
        </el-table-column>
        <el-table-column label="协议" width="100">
          <template #default="{ row }"><span class="muted">{{ row.type }}</span></template>
        </el-table-column>
        <el-table-column label="当前 winner node" width="120">
          <template #default="{ row }">
            <span v-if="row.winner_node_id" class="mono">#{{ row.winner_node_id }}</span>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column label="winner 延迟" width="120">
          <template #default="{ row }">
            <span v-if="row.winner_latency" :class="latencyClass(row.winner_latency)">{{ row.winner_latency }}ms</span>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column label="最后更新" width="180">
          <template #default="{ row }"><span class="muted">{{ formatTime(row.updated_at) }}</span></template>
        </el-table-column>
      </el-table>
      <el-empty v-if="poolOutbounds.length === 0 && !loading" description="还没有订阅池出站 — 添加订阅并刷新,winner 选出后自动创建" />
      <div class="muted" style="font-size:12px; margin-top:8px">
        💡 入站绑这些 tag(`pool-jp` / `pool-hk` …)即可走对应国家;sing-box 看到的是
        「出站管理」+「订阅池出站」union 后的完整列表
      </div>
    </div>

    <!-- 编辑订阅池出站「显示名」-->
    <el-dialog v-model="poolDisplayDialog" title="编辑订阅池出站 · 显示名" width="480px">
      <el-form :model="editingPool" label-width="100px">
        <el-form-item label="国家">
          <span class="cc-badge">{{ editingPool.country }}</span>
          <span class="mono muted" style="margin-left:8px">{{ editingPool.tag }}</span>
        </el-form-item>
        <el-form-item label="显示名">
          <el-input v-model="editingPool.display_name" placeholder="例如:日本机场池、HK-Premium…" />
          <div class="muted" style="font-size:12px; margin-top:4px">
            分享链接(vless/vmess 链接的 ps 字段)中转模式时取这里的值,格式 <code>&lt;显示名&gt;-&lt;客户端名&gt;</code>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="poolDisplayDialog = false">{{ $t('actions.cancel') }}</el-button>
        <el-button type="primary" :loading="savingPool" @click="savePoolDisplayName">{{ $t('actions.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 订阅新增/编辑 -->
    <el-dialog v-model="subDialog" :title="editing.id ? '编辑订阅源' : '新增订阅源'" width="540px">
      <el-form :model="editing" label-width="100px">
        <el-form-item label="名称">
          <el-input v-model="editing.name" placeholder="e.g. 2N订阅" />
        </el-form-item>
        <el-form-item label="订阅 URL">
          <el-input v-model="editing.url" placeholder="https://api.example.com/subscribe?token=..." />
        </el-form-item>
        <el-form-item label="刷新周期(分)">
          <el-input-number v-model="editing.refresh_interval" :min="0" :max="1440" :step="10" />
          <span class="muted" style="margin-left:8px">0 = 仅手动刷新</span>
        </el-form-item>
        <el-form-item label="启用">
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
  if (!err) return '未探测'
  // 命中常见错误关键词,转更易懂的简短提示
  if (/i\/o timeout/.test(err)) return 'TCP timeout(网络不可达)'
  if (/EOF/.test(err)) return 'session EOF(机场端拒绝)'
  if (/connection refused/.test(err)) return '连接被拒(端口关闭)'
  if (/no route to host/.test(err)) return '无路由(机房黑洞)'
  if (/tls/i.test(err) && /handshake/i.test(err)) return 'TLS 握手失败'
  if (/http 4\d\d/i.test(err)) return err.match(/http \d+/)?.[0] ?? 'http 错误'
  if (/http 5\d\d/i.test(err)) return err.match(/http \d+/)?.[0] ?? 'http 错误'
  if (/ctx done/.test(err)) return '总超时'
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
  if (!editing.value.url.trim()) { ElMessage.error('URL 必填'); return }
  saving.value = true
  try {
    const fd = new FormData()
    Object.entries(editing.value).forEach(([k, v]) => fd.append(k, String(v ?? '')))
    const res = await HttpUtils.post('api/subSave', fd as any)
    if (res.success) {
      ElMessage.success(editing.value.id ? '已更新' : '已新增 — 点立即刷新拉取节点')
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
  if (res.success) { ElMessage.success('已删除'); await loadAll() }
}

const toggleEnable = async (row: Sub, val: boolean) => {
  const fd = new FormData()
  Object.entries({ ...row, enable: val }).forEach(([k, v]) => fd.append(k, String(v ?? '')))
  const res = await HttpUtils.post('api/subSave', fd as any)
  if (res.success) { row.enable = val } else { ElMessage.error('切换失败') }
}

const refreshSub = async (row: Sub) => {
  refreshingId.value = row.id ?? null
  lastRefresh.value = null
  try {
    const fd = new FormData(); fd.append('id', String(row.id))
    const res = await HttpUtils.post('api/subRefresh', fd as any)
    if (res.success && res.obj) {
      lastRefresh.value = res.obj
      ElMessage.success(`刷新完成:存活 ${res.obj.alive} / 解析 ${res.obj.parsed}`)
    } else {
      ElMessage.error('刷新失败:' + (res.msg || '未知'))
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
      ElMessage.success('已保存 — 分享链接下次生成时会用新名称')
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
