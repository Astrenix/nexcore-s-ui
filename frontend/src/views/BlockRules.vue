<template>
  <div class="page-container">
    <div class="page-header with-actions">
      <div class="page-header-text">
        <h2 class="page-title">{{ $t('pages.blockRules') }}</h2>
        <p class="page-desc">{{ $t('blockrule.desc') }}</p>
      </div>
      <div class="page-header-actions">
        <el-button @click="loadList" :loading="loading">
          <el-icon><Refresh /></el-icon>{{ $t('actions.refresh') }}
        </el-button>
        <el-button @click="presetsVisible = true">
          <el-icon><MagicStick /></el-icon>{{ $t('blockrule.applyPresets') }}
        </el-button>
        <el-button type="primary" @click="openEdit(null)">
          <el-icon><Plus /></el-icon>{{ $t('actions.add') }}
        </el-button>
      </div>
    </div>

    <div class="nc-card">
      <el-table :data="rules" stripe>
        <el-table-column :label="$t('enable')" width="80">
          <template #default="{ row }">
            <el-switch
              :model-value="row.enable"
              :disabled="isManaged(row) || togglingId === row.id"
              @change="(val: boolean) => toggleEnable(row, val)"
            />
          </template>
        </el-table-column>
        <el-table-column prop="type" :label="$t('type')" width="110">
          <template #default="{ row }">
            <span class="type-pill">{{ row.type }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="value" :label="$t('objects.value')" min-width="220" show-overflow-tooltip>
          <template #default="{ row }"><span class="mono">{{ row.value }}</span></template>
        </el-table-column>
        <el-table-column prop="remark" :label="$t('in.remark')" min-width="180">
          <template #default="{ row }">
            <span v-if="isManaged(row)" class="managed-tag">{{ $t('blockrule.managed') }}</span>{{ stripNexcorePrefix(row.remark) }}
          </template>
        </el-table-column>
        <el-table-column prop="inboundTag" :label="$t('blockrule.inboundTag')" width="140">
          <template #default="{ row }">
            <span v-if="row.inboundTag" class="mono">{{ row.inboundTag }}</span>
            <span v-else class="muted">{{ $t('blockrule.global') }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="createdAt" :label="$t('blockrule.createdAt')" width="170">
          <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
        </el-table-column>
        <el-table-column :label="$t('actions.action')" width="120" align="center">
          <template #default="{ row }">
            <el-tooltip :content="$t('actions.edit')" placement="top">
              <el-button text :disabled="isManaged(row)" @click="openEdit(row)">
                <el-icon><Edit /></el-icon>
              </el-button>
            </el-tooltip>
            <el-popconfirm
              :title="$t('blockrule.delConfirm')"
              :confirm-button-text="$t('yes')"
              :cancel-button-text="$t('no')"
              @confirm="delRule(row.id!)"
            >
              <template #reference>
                <el-button text :disabled="isManaged(row)">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="rules.length === 0 && !loading" :description="$t('blockrule.empty')" />
    </div>

    <el-dialog v-model="dialogVisible" :title="editing.id ? $t('blockrule.editTitle') : $t('blockrule.addTitle')" width="540px">
      <el-form :model="editing" label-width="86px">
        <el-form-item :label="$t('type')" required>
          <el-select v-model="editing.type">
            <el-option v-for="t in typeOptions" :key="t.value" :label="t.label" :value="t.value" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('objects.value')" required>
          <el-input v-model="editing.value" :placeholder="$t('blockrule.valuePlaceholder')" />
          <span class="form-hint">{{ valueHint }}</span>
        </el-form-item>
        <el-form-item :label="$t('in.remark')">
          <el-input v-model="editing.remark" :placeholder="$t('blockrule.remarkPlaceholder')" />
        </el-form-item>
        <el-form-item :label="$t('blockrule.inboundTag')">
          <el-input v-model="editing.inboundTag" :placeholder="$t('blockrule.inboundTagPlaceholder')" />
          <span class="form-hint">{{ $t('blockrule.inboundTagHint') }}</span>
        </el-form-item>
        <el-form-item :label="$t('enable')">
          <el-switch v-model="editing.enable" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ $t('actions.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="saveRule">{{ $t('actions.save') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="presetsVisible" :title="$t('blockrule.presetsTitle')" width="560px">
      <p class="preset-tip">{{ $t('blockrule.presetsTip') }}</p>
      <div class="presets-grid">
        <div v-for="p in presets" :key="p.key" class="preset-card">
          <h4>{{ p.name }}</h4>
          <p>{{ p.description }}</p>
          <el-button type="primary" plain :loading="presetApplying === p.key" @click="applyPreset(p)">
            <el-icon><Plus /></el-icon>{{ $t('blockrule.import') }}
          </el-button>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script lang="ts" setup>
import { ref, computed, onMounted } from 'vue'
import { Plus, Edit, Delete, Refresh, MagicStick } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import HttpUtils from '@/plugins/httputil'
import { i18n } from '@/locales'

interface BlockRule {
  id?: number
  type: string
  value: string
  remark: string
  inboundTag: string
  enable: boolean
  createdAt?: number
}

interface Preset {
  key: string
  name: string
  description: string
  rules: BlockRule[]
}

// 主控的命名空间 — 跟 service/proxy_host_xui_client.go 的 [NexCore] 前缀对齐。
const MANAGED_PREFIX = '[NexCore]'

const rules = ref<BlockRule[]>([])
const loading = ref(false)
const saving = ref(false)
const togglingId = ref<number | null>(null)
const presetApplying = ref<string | null>(null)
const dialogVisible = ref(false)
const presetsVisible = ref(false)
const editing = ref<BlockRule>(emptyRule())

const typeOptions = [
  { value: 'domain',   label: i18n.global.t('blockrule.type.domain') },
  { value: 'ip',       label: i18n.global.t('blockrule.type.ip') },
  { value: 'geosite',  label: i18n.global.t('blockrule.type.geosite') },
  { value: 'geoip',    label: i18n.global.t('blockrule.type.geoip') },
  { value: 'port',     label: i18n.global.t('blockrule.type.port') },
  { value: 'protocol', label: i18n.global.t('blockrule.type.protocol') },
  { value: 'source',   label: i18n.global.t('blockrule.type.source') },
]

// 预置跟 v1 后端的 listBlockRulePresets 对齐(database/model/block_rule.go 注释)
const presets: Preset[] = [
  {
    key: 'ads',
    name: i18n.global.t('blockrule.preset.adsName'),
    description: i18n.global.t('blockrule.preset.adsDesc'),
    rules: [{ type: 'geosite', value: 'category-ads-all', remark: i18n.global.t('blockrule.preset.adsRemark'), inboundTag: '', enable: true }],
  },
  {
    key: 'tracker',
    name: i18n.global.t('blockrule.preset.trackerName'),
    description: i18n.global.t('blockrule.preset.trackerDesc'),
    rules: [{ type: 'geosite', value: 'category-public-tracker', remark: i18n.global.t('blockrule.preset.trackerRemark'), inboundTag: '', enable: true }],
  },
  {
    key: 'porn',
    name: i18n.global.t('blockrule.preset.pornName'),
    description: i18n.global.t('blockrule.preset.pornDesc'),
    rules: [{ type: 'geosite', value: 'category-porn', remark: i18n.global.t('blockrule.preset.pornRemark'), inboundTag: '', enable: true }],
  },
]

const valueHint = computed(() => {
  switch (editing.value.type) {
    case 'domain':   return i18n.global.t('blockrule.hint.domain')
    case 'ip':       return i18n.global.t('blockrule.hint.ip')
    case 'geosite':  return i18n.global.t('blockrule.hint.geosite')
    case 'geoip':    return i18n.global.t('blockrule.hint.geoip')
    case 'port':     return i18n.global.t('blockrule.hint.port')
    case 'protocol': return i18n.global.t('blockrule.hint.protocol')
    case 'source':   return i18n.global.t('blockrule.hint.source')
    default:         return ''
  }
})

function emptyRule(): BlockRule {
  return { type: 'domain', value: '', remark: '', inboundTag: '', enable: true }
}

const isManaged = (r: BlockRule) => (r.remark || '').startsWith(MANAGED_PREFIX)

const stripNexcorePrefix = (s: string) => (s || '').replace(/^\[NexCore\]\s*/, '')

const formatTime = (ms?: number) => {
  if (!ms) return '—'
  const d = new Date(ms)
  return d.toLocaleString()
}

const loadList = async () => {
  loading.value = true
  try {
    // sui 内部 API 走 LoadPartialData("block-rules"),响应是 {success, obj:{"block-rules":[...]}}
    const res = await HttpUtils.get('api/block-rules')
    if (res.success && res.obj) {
      rules.value = (res.obj['block-rules'] as BlockRule[]) || []
    }
  } finally {
    loading.value = false
  }
}

const openEdit = (r: BlockRule | null) => {
  editing.value = r ? { ...r } : emptyRule()
  dialogVisible.value = true
}

// saveAction 走 sui 内部 /api/save 路径,跟 inbounds/outbounds 同套
const saveAction = async (action: 'new' | 'edit' | 'del', data: any): Promise<boolean> => {
  const fd = new FormData()
  fd.append('object', 'block-rules')
  fd.append('action', action)
  fd.append('data', JSON.stringify(data))
  const res = await HttpUtils.post('api/save', fd as any)
  return !!res.success
}

const saveRule = async () => {
  if (!editing.value.value.trim()) return ElMessage.error(i18n.global.t('blockrule.valueRequired'))
  saving.value = true
  try {
    const ok = await saveAction(editing.value.id ? 'edit' : 'new', editing.value)
    if (ok) {
      ElMessage.success(editing.value.id ? i18n.global.t('blockrule.updated') : i18n.global.t('blockrule.added'))
      dialogVisible.value = false
      await loadList()
    }
  } finally {
    saving.value = false
  }
}

const delRule = async (id: number) => {
  if (await saveAction('del', [id])) {
    ElMessage.success(i18n.global.t('blockrule.deleted'))
    await loadList()
  }
}

const toggleEnable = async (r: BlockRule, val: boolean) => {
  togglingId.value = r.id ?? null
  try {
    const ok = await saveAction('edit', { ...r, enable: val })
    if (ok) {
      r.enable = val
      ElMessage.success(val ? i18n.global.t('blockrule.enabled') : i18n.global.t('blockrule.disabled'))
    }
  } finally {
    togglingId.value = null
  }
}

const applyPreset = async (p: Preset) => {
  presetApplying.value = p.key
  try {
    let okCount = 0
    for (const r of p.rules) {
      if (await saveAction('new', r)) okCount++
    }
    ElMessage.success(i18n.global.t('blockrule.imported', { n: okCount }))
    presetsVisible.value = false
    await loadList()
  } finally {
    presetApplying.value = null
  }
}

onMounted(loadList)
</script>

<style scoped>
.muted { color: var(--nc-text-muted); }
.mono { font-family: var(--font-mono, ui-monospace, SFMono-Regular, Menlo, monospace); }
.type-pill {
  display: inline-block;
  padding: 1px 8px;
  background: var(--nc-border-soft);
  border-radius: 4px;
  font-size: 12px;
  font-family: var(--font-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  color: var(--nc-text-2);
}
.managed-tag {
  display: inline-block;
  padding: 1px 6px;
  background: var(--nc-primary-soft);
  color: var(--nc-primary-deep);
  border-radius: 4px;
  font-size: 11px;
  margin-right: 6px;
  font-weight: 600;
}
.form-hint {
  display: block;
  color: var(--nc-text-muted);
  font-size: 12px;
  margin-top: 4px;
  line-height: 1.4;
}
.preset-tip { margin: 0 0 12px; color: var(--nc-text-muted); font-size: 13px; }
.presets-grid {
  display: grid;
  gap: 12px;
}
.preset-card {
  padding: 14px 16px;
  border: 1px solid var(--nc-border);
  border-radius: var(--radius-md);
}
.preset-card h4 { margin: 0 0 4px; font-size: 14px; }
.preset-card p { margin: 0 0 10px; color: var(--nc-text-muted); font-size: 13px; line-height: 1.5; }
</style>
