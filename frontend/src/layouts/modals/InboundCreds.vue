<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="onClose"
    class="constrained-dialog is-large"
    :align-center="false"
    destroy-on-close
  >
    <template #header>
      <span>{{ $t('inboundCreds.title') }}</span>
      <span class="mono creds-tag">{{ inboundTag }}</span>
      <el-tag size="small" type="info" effect="plain" style="margin-left: 8px">{{ inboundType }}</el-tag>
    </template>

    <div v-if="loading" class="creds-loading">
      <el-icon class="is-loading"><Loading /></el-icon>
    </div>
    <div v-else>
      <!-- 入站元信息 banner -->
      <div class="info-banner">
        <div class="info-banner__field">
          <span class="info-banner__label">{{ $t('dns.server') }}</span>
          <code class="info-banner__value">{{ serverHost }}</code>
        </div>
        <div class="info-banner__field">
          <span class="info-banner__label">{{ $t('in.port') }}</span>
          <code class="info-banner__value">{{ listenPort }}</code>
        </div>
        <div class="info-banner__field">
          <span class="info-banner__label">{{ $t('inboundCreds.credCount') }}</span>
          <code class="info-banner__value">{{ users.length }}</code>
        </div>
        <div class="info-banner__spacer"></div>
        <el-button type="primary" size="small" @click="addOne">
          <el-icon><Plus /></el-icon>{{ $t('inboundCreds.addCred') }}
        </el-button>
      </div>

      <div v-if="!users.length" class="creds-empty">
        {{ $t('inboundCreds.empty') }}
      </div>

      <div v-for="(u, i) in users" :key="i" class="cred-card" :class="{ 'is-disabled': !u.enable }">
        <div class="cred-card__head">
          <div class="cred-card__head-left">
            <span class="cred-card__idx">#{{ i + 1 }}</span>
            <el-tag v-if="!u.enable" type="danger" size="small" effect="plain">{{ $t('inboundCreds.disabled') }}</el-tag>
            <el-tag v-else-if="isExpired(u)" type="danger" size="small" effect="plain">{{ $t('date.expired') }}</el-tag>
            <el-tag v-else-if="isOverQuota(u)" type="danger" size="small" effect="plain">{{ $t('inboundCreds.overQuota') }}</el-tag>
            <el-tag v-else type="success" size="small" effect="plain">{{ $t('inboundCreds.available') }}</el-tag>
          </div>
          <div class="cred-card__head-actions">
            <el-tooltip :content="$t('inboundCreds.toggleTip')" placement="top">
              <el-switch v-model="u.enable" size="small" />
            </el-tooltip>
            <el-tooltip :content="$t('inboundCreds.resetTip')" placement="top">
              <el-button size="small" plain @click="resetOne(i)">
                <el-icon><Refresh /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip :content="$t('inboundCreds.resetUsageTip')" placement="top">
              <el-button size="small" plain @click="resetUsage(u.username)">
                <el-icon><RefreshLeft /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip :content="$t('actions.del')" placement="top">
              <el-button size="small" plain type="danger" @click="removeOne(i)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </el-tooltip>
          </div>
        </div>

        <div class="cred-row">
          <span class="cred-row__label">{{ $t('types.un') }}</span>
          <el-input v-model="u.username" size="small" class="cred-row__input mono" placeholder="username" />
          <el-button text size="small" @click="copy(u.username, $t('types.un'))"><el-icon><CopyDocument /></el-icon></el-button>
        </div>

        <div class="cred-row">
          <span class="cred-row__label">{{ $t('types.pw') }}</span>
          <el-input
            v-model="u.password"
            size="small"
            class="cred-row__input mono"
            :type="shown[i] ? 'text' : 'password'"
            placeholder="password"
          />
          <el-button text size="small" @click="shown[i] = !shown[i]">
            <el-icon><View v-if="!shown[i]" /><Hide v-else /></el-icon>
          </el-button>
          <el-button text size="small" @click="copy(u.password, $t('types.pw'))"><el-icon><CopyDocument /></el-icon></el-button>
        </div>

        <div class="cred-row">
          <span class="cred-row__label">{{ $t('inboundCreds.connUri') }}</span>
          <code class="cred-uri mono">{{ uriOf(u) }}</code>
          <el-button text size="small" @click="copy(uriOf(u), $t('inboundCreds.connUri'))"><el-icon><CopyDocument /></el-icon></el-button>
        </div>

        <!-- 限制与用量 -->
        <div class="cred-limits">
          <div class="cred-limits__row">
            <span class="cred-row__label">{{ $t('inboundCreds.volumeLimit') }}</span>
            <el-input-number
              v-model="u.volume_limit_gb"
              :min="0"
              :step="1"
              :precision="2"
              size="small"
              controls-position="right"
              class="limit-num"
            />
            <span class="limit-unit">{{ $t('inboundCreds.gbUnlimited') }}</span>
            <span class="limit-spacer"></span>
            <span class="cred-row__label">{{ $t('date.expiry') }}</span>
            <el-date-picker
              v-model="u.expiry_date"
              type="datetime"
              size="small"
              :placeholder="$t('inboundCreds.expiryPlaceholder')"
              value-format="x"
              class="limit-date"
            />
          </div>
          <div v-if="u.volume_limit_gb > 0 || u.expiry_date" class="cred-limits__progress">
            <div v-if="u.volume_limit_gb > 0" class="usage-bar">
              <span class="usage-bar__label">{{ $t('inboundCreds.used') }} {{ formatBytes(u.used) }} / {{ u.volume_limit_gb }} GB</span>
              <el-progress
                :percentage="usagePct(u)"
                :status="usagePct(u) >= 100 ? 'exception' : usagePct(u) >= 80 ? 'warning' : 'success'"
                :stroke-width="6"
                :show-text="false"
              />
            </div>
            <div v-if="u.expiry_date" class="expiry-line" :class="{ 'is-expired': isExpired(u) }">
              {{ expiryLabel(u) }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <span v-if="dirty" class="dirty-hint">{{ $t('inboundCreds.unsaved') }}</span>
      <el-button @click="onClose">{{ $t('actions.close') }}</el-button>
      <el-button type="primary" :loading="saving" :disabled="loading || !dirty" @click="onSave">
        {{ $t('inboundCreds.saveChanges') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, computed, watch, onMounted } from 'vue'
import Data from '@/store/modules/data'
import HttpUtils from '@/plugins/httputil'
import RandomUtil from '@/plugins/randomUtil'
import { HumanReadable } from '@/plugins/utils'
import { ElMessage } from 'element-plus'
import { Loading, Plus, CopyDocument, Refresh, RefreshLeft, Delete, View, Hide } from '@element-plus/icons-vue'
import { i18n } from '@/locales'

interface CredEntry {
  username: string
  password: string
  enable: boolean
  volume_limit_gb: number  // GB, UI 直接编辑(更直观);保存时换算成 bytes 存 ext
  expiry_date: number | null  // unix ms, el-date-picker 'x' 格式;保存时换算成 unix sec
  used: number  // 已用 bytes(read-only,从 stats 拿)
}

const props = defineProps<{ visible: boolean; inboundId: number; inboundTag: string; inboundType: string }>()
const emit = defineEmits<{ close: []; 'update:modelValue': [v: boolean] }>()

const loading = ref(false)
const saving = ref(false)
const fullInbound = ref<any>(null)
const users = ref<CredEntry[]>([])
const initialJson = ref('[]')
const shown = ref<Record<number, boolean>>({})
const serverHost = ref('')
const listenPort = computed(() => fullInbound.value?.listen_port ?? '?')
const dirty = computed(() => JSON.stringify(stripUsed(users.value)) !== initialJson.value)
const stripUsed = (arr: CredEntry[]) => arr.map(({ used: _u, ...rest }) => rest)

const pickStr = (n: number) => RandomUtil.randomSeq(n)
const newCred = (): CredEntry => ({
  username: 'user_' + pickStr(4),
  password: pickStr(16),
  enable: true,
  volume_limit_gb: 0,
  expiry_date: null,
  used: 0,
})

const load = async () => {
  loading.value = true
  try {
    const arr = await Data().loadInbounds([props.inboundId])
    fullInbound.value = arr[0]
    const baseUsers: any[] = Array.isArray(fullInbound.value?.users) ? fullInbound.value.users : []
    const ext = fullInbound.value?.ext || {}
    const credsMeta = ext.creds || {}

    // 拉 stats.user 累加流量,key=username,value={up,down}
    const usageMap: Record<string, number> = {}
    try {
      const r = await HttpUtils.get('api/statsTotals', { resource: 'user' })
      if (r.success && r.obj) {
        for (const [name, t] of Object.entries(r.obj)) {
          const tt = t as { up?: number; down?: number }
          usageMap[name] = (tt.up || 0) + (tt.down || 0)
        }
      }
    } catch { /* stats 拉失败不阻塞 */ }

    users.value = baseUsers.map((u) => {
      const meta = credsMeta[u.username] || {}
      return {
        username: u.username || '',
        password: u.password || '',
        enable: meta.enable !== false, // 默认 true
        volume_limit_gb: meta.volume_limit ? Math.round((meta.volume_limit / 1073741824) * 100) / 100 : 0,
        expiry_date: meta.expiry ? meta.expiry * 1000 : null,
        used: usageMap[u.username] || 0,
      }
    })
    initialJson.value = JSON.stringify(stripUsed(users.value))
    shown.value = {}
    serverHost.value = window.location.hostname || ''
  } finally {
    loading.value = false
  }
}

const addOne = () => { users.value.push(newCred()) }
const removeOne = (i: number) => { users.value.splice(i, 1); delete shown.value[i] }
const resetOne = (i: number) => {
  const cred = newCred()
  // 重置只换 username + password,保留 enable / 限制 / 到期 / 已用
  users.value[i].username = cred.username
  users.value[i].password = cred.password
}

const resetUsage = async (username: string) => {
  if (!username) return
  const r = await HttpUtils.post('api/resetTraffic', { resource: 'user', tag: username })
  if (r.success) {
    ElMessage.success(i18n.global.t('inboundCreds.usageReset', { name: username }))
    const u = users.value.find((x) => x.username === username)
    if (u) u.used = 0
  }
}

const copy = async (text: string, label: string) => {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success(i18n.global.t('inboundCreds.copied', { label }))
  } catch {
    ElMessage.warning(i18n.global.t('inboundCreds.clipboardDenied'))
  }
}

const uriOf = (u: CredEntry): string => {
  const enc = (s: string) => encodeURIComponent(s)
  const host = serverHost.value || '<server>'
  const port = listenPort.value
  switch (props.inboundType) {
    case 'mixed':
    case 'socks':
      return `socks5://${enc(u.username)}:${enc(u.password)}@${host}:${port}`
    case 'http':
      return `http://${enc(u.username)}:${enc(u.password)}@${host}:${port}`
    case 'naive':
      return `https://${enc(u.username)}:${enc(u.password)}@${host}:${port}`
    default:
      return `${props.inboundType}://${enc(u.username)}:${enc(u.password)}@${host}:${port}`
  }
}

const formatBytes = (n: number) => HumanReadable.sizeFormat(n || 0)
const usagePct = (u: CredEntry): number => {
  if (!u.volume_limit_gb || u.volume_limit_gb <= 0) return 0
  const limit = u.volume_limit_gb * 1073741824
  return Math.min(100, Math.round((u.used / limit) * 100))
}
const isExpired = (u: CredEntry) => !!u.expiry_date && u.expiry_date < Date.now()
const isOverQuota = (u: CredEntry) => u.volume_limit_gb > 0 && u.used >= u.volume_limit_gb * 1073741824
const expiryLabel = (u: CredEntry) => {
  if (!u.expiry_date) return ''
  const d = new Date(u.expiry_date)
  const expired = u.expiry_date < Date.now()
  const fmt = d.toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false })
  return expired ? i18n.global.t('inboundCreds.expiredAt', { date: fmt }) : i18n.global.t('inboundCreds.expiresAt', { date: fmt })
}

const onSave = async () => {
  if (!fullInbound.value) return
  for (const u of users.value) {
    if (!u.username.trim()) {
      ElMessage.error(i18n.global.t('inboundCreds.unameEmpty'))
      return
    }
  }
  const seen = new Set<string>()
  for (const u of users.value) {
    if (seen.has(u.username)) {
      ElMessage.error(i18n.global.t('inboundCreds.unameDup', { name: u.username }))
      return
    }
    seen.add(u.username)
  }
  saving.value = true
  try {
    // inbound.users = 仅 username + password(给 sing-box)
    fullInbound.value.users = users.value.map((u) => ({ username: u.username, password: u.password }))
    // inbound.ext.creds = 元数据(给 cronjob)
    const creds: Record<string, any> = {}
    for (const u of users.value) {
      creds[u.username] = {
        enable: u.enable,
        volume_limit: u.volume_limit_gb > 0 ? Math.round(u.volume_limit_gb * 1073741824) : 0,
        expiry: u.expiry_date ? Math.floor(u.expiry_date / 1000) : 0,
      }
    }
    fullInbound.value.ext = { ...(fullInbound.value.ext || {}), creds }

    const ok = await Data().save('inbounds', 'edit', fullInbound.value)
    if (ok) {
      ElMessage.success(i18n.global.t('inboundCreds.saved', { tag: props.inboundTag, n: users.value.length }))
      initialJson.value = JSON.stringify(stripUsed(users.value))
      onClose()
    }
  } finally {
    saving.value = false
  }
}

const onClose = () => {
  emit('close')
  emit('update:modelValue', false)
}

watch(() => props.visible, (v) => { if (v && props.inboundId > 0) load() })
onMounted(() => { if (props.visible && props.inboundId > 0) load() })
</script>

<style scoped>
.creds-tag { font-weight: 600; color: var(--nc-text-1); }
.creds-loading { display: flex; justify-content: center; padding: 60px 0; font-size: 28px; color: var(--nc-primary); }

.info-banner {
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 12px 14px;
  background: linear-gradient(90deg, var(--nc-primary-soft), transparent);
  border: 1px solid var(--nc-border-soft);
  border-radius: var(--radius-md);
  margin-bottom: 14px;
}
.info-banner__field { display: flex; flex-direction: column; gap: 2px; }
.info-banner__label { font-size: 11px; color: var(--nc-text-muted); text-transform: uppercase; letter-spacing: 0.05em; }
.info-banner__value { font-size: 14px; font-weight: 600; color: var(--nc-text-1); font-family: var(--font-mono); }
.info-banner__spacer { flex: 1; }

.creds-empty {
  padding: 32px 0;
  text-align: center;
  color: var(--nc-text-muted);
  font-size: 13px;
  background: var(--nc-bg-2, #f8fafc);
  border: 1px dashed var(--nc-border-soft);
  border-radius: var(--radius-md);
}

.cred-card {
  border: 1px solid var(--nc-border-soft);
  border-radius: var(--radius-md);
  padding: 12px 14px;
  margin-bottom: 10px;
  background: var(--nc-surface, #fff);
  transition: border-color 0.15s, box-shadow 0.15s, opacity 0.15s;
}
.cred-card:hover { border-color: var(--nc-primary); box-shadow: 0 2px 8px rgba(59, 130, 246, 0.06); }
.cred-card.is-disabled { opacity: 0.55; background: var(--nc-bg-2, #f8fafc); }

.cred-card__head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; }
.cred-card__head-left { display: flex; align-items: center; gap: 8px; }
.cred-card__head-actions { display: flex; gap: 6px; align-items: center; }
.cred-card__idx {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px; height: 24px;
  border-radius: 50%;
  background: var(--nc-primary-soft);
  color: var(--nc-primary);
  font-size: 12px;
  font-weight: 700;
}

.cred-row { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
.cred-row__label { flex-shrink: 0; width: 64px; font-size: 12px; color: var(--nc-text-muted); }
.cred-row__input { flex: 1; min-width: 0; }
.cred-uri {
  flex: 1; min-width: 0; padding: 6px 10px; font-size: 12px; color: var(--nc-text-1);
  background: var(--nc-bg-2, #f8fafc); border: 1px solid var(--nc-border-soft);
  border-radius: var(--radius-sm); white-space: nowrap; overflow-x: auto;
  font-family: var(--font-mono);
}

.cred-limits {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px dashed var(--nc-border-soft);
}
.cred-limits__row { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.limit-num { width: 110px !important; }
.limit-date { width: 200px; }
.limit-unit { font-size: 11.5px; color: var(--nc-text-muted); }
.limit-spacer { width: 16px; }

.cred-limits__progress { margin-top: 8px; display: flex; flex-direction: column; gap: 4px; }
.usage-bar { display: flex; flex-direction: column; gap: 3px; }
.usage-bar__label { font-size: 11px; color: var(--nc-text-muted); font-family: var(--font-mono); }
.expiry-line { font-size: 11px; color: var(--nc-text-muted); }
.expiry-line.is-expired { color: var(--nc-danger); font-weight: 600; }

.dirty-hint { margin-right: 12px; font-size: 12px; color: var(--nc-warning); }
</style>
