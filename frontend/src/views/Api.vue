<template>
  <div class="page-container">
    <div class="page-header with-actions">
      <div class="page-header-text">
        <h2 class="page-title">{{ $t('pages.api') }}</h2>
        <p class="page-desc">{{ $t('api.desc') }}</p>
      </div>
      <div class="page-header-actions">
        <el-tag type="info" effect="plain" class="api-base">
          <span class="mono">{{ apiBaseUrl }}</span>
        </el-tag>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="api-tabs">
      <el-tab-pane :label="$t('api.tab.tokens')" name="tokens">
        <div class="tab-toolbar">
          <el-input
            v-model="tokenFilter"
            :placeholder="$t('api.tokens.search')"
            clearable
            style="max-width: 280px"
          >
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-button type="primary" @click="openCreateToken">
            <el-icon><Plus /></el-icon>{{ $t('api.tokens.create') }}
          </el-button>
        </div>

        <el-alert
          v-if="latestPlainToken"
          type="success"
          show-icon
          :closable="true"
          @close="latestPlainToken = ''"
          class="token-banner"
        >
          <template #title>
            <span>{{ $t('api.tokens.created') }}</span>
          </template>
          <el-input :model-value="latestPlainToken" readonly>
            <template #append>
              <el-button @click="copyText(latestPlainToken)">
                <el-icon><DocumentCopy /></el-icon>
              </el-button>
            </template>
          </el-input>
        </el-alert>

        <el-table :data="filteredTokens" v-loading="tokensLoading" stripe size="small" class="nc-table">
          <el-table-column prop="id" label="#" width="60" />
          <el-table-column prop="desc" :label="$t('api.tokens.desc')" min-width="160" show-overflow-tooltip />
          <el-table-column prop="token" :label="$t('api.tokens.token')" min-width="140">
            <template #default="{ row }">
              <span class="mono token-mask">{{ row.token }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="$t('api.tokens.expiry')" min-width="200">
            <template #default="{ row }">
              <span class="mono expiry-text">{{ formatExpiry(row.expiry) }}</span>
              <span class="expiry-pill" :class="expiryClass(row.expiry)">{{ $t('api.tokens.status.' + expiryClass(row.expiry)) }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="$t('actions.action')" width="120" align="center">
            <template #default="{ row }">
              <el-tooltip :content="$t('api.tokens.reset')" placement="top">
                <el-popconfirm
                  :title="$t('api.tokens.resetConfirm')"
                  :confirm-button-text="$t('yes')"
                  :cancel-button-text="$t('no')"
                  width="280"
                  @confirm="resetToken(row.id)"
                >
                  <template #reference>
                    <el-button text>
                      <el-icon style="color: var(--nc-primary)"><Refresh /></el-icon>
                    </el-button>
                  </template>
                </el-popconfirm>
              </el-tooltip>
              <el-tooltip :content="$t('actions.del')" placement="top">
                <el-popconfirm
                  :title="$t('confirm')"
                  :confirm-button-text="$t('yes')"
                  :cancel-button-text="$t('no')"
                  @confirm="deleteToken(row.id)"
                >
                  <template #reference>
                    <el-button text>
                      <el-icon style="color: var(--nc-danger)"><Delete /></el-icon>
                    </el-button>
                  </template>
                </el-popconfirm>
              </el-tooltip>
            </template>
          </el-table-column>
        </el-table>

        <el-empty v-if="!tokensLoading && tokens.length === 0" :description="$t('api.tokens.empty')" />

        <el-dialog
          v-model="createTokenVisible"
          class="constrained-dialog"
          :title="$t('api.tokens.create')"
          :align-center="false"
          append-to-body
          destroy-on-close
        >
          <el-form label-position="top">
            <el-form-item :label="$t('api.tokens.desc')">
              <el-input v-model="createForm.desc" :placeholder="$t('api.tokens.descHint')" />
            </el-form-item>
            <el-form-item :label="`${$t('api.tokens.expiry')} (${$t('date.d')})`">
              <el-input-number
                v-model="createForm.expiry"
                :min="0"
                controls-position="right"
                style="width: 100%"
              />
              <p class="form-hint">{{ $t('api.tokens.expiryHint') }}</p>
            </el-form-item>
          </el-form>
          <template #footer>
            <el-button @click="createTokenVisible = false">{{ $t('actions.close') }}</el-button>
            <el-button type="primary" :loading="creating" @click="submitCreate">
              {{ $t('actions.add') }}
            </el-button>
          </template>
        </el-dialog>
      </el-tab-pane>

      <el-tab-pane :label="$t('api.tab.docs')" name="docs">
        <div class="docs-intro nc-card">
          <h3 class="docs-intro__title">{{ $t('api.docs.title') }}</h3>
          <p class="docs-intro__hint">{{ $t('api.docs.hintBearer') }}</p>
          <div class="docs-curl">
            <span class="docs-curl__label">curl</span>
            <code class="mono">
              curl -H "Authorization: Bearer &lt;TOKEN&gt;" {{ apiBaseUrl }}/me
            </code>
            <el-button text @click="copyCurlExample">
              <el-icon><DocumentCopy /></el-icon>
            </el-button>
          </div>
        </div>

        <div class="docs-intro nc-card v1-banner">
          <h3 class="docs-intro__title">
            <el-icon class="title-icon"><Connection /></el-icon>
            {{ $t('api.docs.v1Title') }}
          </h3>
          <p class="docs-intro__hint">{{ $t('api.docs.v1Hint') }}</p>
          <div class="docs-curl">
            <span class="docs-curl__label v1-tag">v1</span>
            <code class="mono">{{ apiV1BaseUrl }}</code>
            <el-button text @click="copyText(apiV1BaseUrl)">
              <el-icon><DocumentCopy /></el-icon>
            </el-button>
          </div>
          <details class="v1-list">
            <summary>{{ $t('api.docs.v1Endpoints', { n: v1Endpoints.length }) }}</summary>
            <table class="docs-table">
              <thead>
                <tr>
                  <th class="col-method">{{ $t('api.docs.col.method') }}</th>
                  <th>{{ $t('api.docs.col.path') }}</th>
                  <th>{{ $t('api.docs.col.desc') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="e in v1Endpoints" :key="e.method + e.path">
                  <td>
                    <span class="method-pill" :class="`method-${e.method.toLowerCase()}`">{{ e.method }}</span>
                  </td>
                  <td><span class="mono">{{ e.path }}</span></td>
                  <td><span class="docs-params">{{ e.note || '' }}</span></td>
                </tr>
              </tbody>
            </table>
          </details>
        </div>

        <el-input
          v-model="docsFilter"
          :placeholder="$t('api.docs.search')"
          clearable
          class="docs-search"
        >
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>

        <div class="docs-groups">
          <div v-for="group in filteredDocGroups" :key="group.name" class="docs-group nc-card">
            <h4 class="docs-group__title">{{ $t('api.docs.group.' + group.name) }}</h4>
            <table class="docs-table">
              <thead>
                <tr>
                  <th class="col-method">{{ $t('api.docs.col.method') }}</th>
                  <th class="col-path">{{ $t('api.docs.col.path') }}</th>
                  <th>{{ $t('api.docs.col.desc') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="ep in group.endpoints" :key="ep.method + ep.path">
                  <td>
                    <span class="method-pill" :class="`method-${ep.method.toLowerCase()}`">
                      {{ ep.method }}
                    </span>
                  </td>
                  <td>
                    <span class="mono path-cell">{{ ep.path }}</span>
                  </td>
                  <td>
                    <div class="docs-desc">{{ ep.desc }}</div>
                    <div v-if="ep.params" class="docs-params mono">{{ ep.params }}</div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane :label="$t('api.tab.logs')" name="logs">
        <div class="tab-toolbar">
          <el-select
            v-model="logFilter.method"
            clearable
            :placeholder="$t('api.logs.method')"
            style="width: 110px"
          >
            <el-option label="GET" value="GET" />
            <el-option label="POST" value="POST" />
          </el-select>
          <el-input
            v-model="logFilter.path"
            :placeholder="$t('api.logs.path')"
            clearable
            style="max-width: 220px"
          />
          <el-input
            v-model="logFilter.username"
            :placeholder="$t('api.logs.username')"
            clearable
            style="max-width: 180px"
          />
          <el-button @click="reloadLogs">
            <el-icon><Search /></el-icon>{{ $t('actions.search') }}
          </el-button>
          <el-button @click="resetLogFilter">{{ $t('reset') }}</el-button>
          <div class="toolbar-spacer" />
          <el-popconfirm
            :title="$t('api.logs.clearConfirm')"
            :confirm-button-text="$t('yes')"
            :cancel-button-text="$t('no')"
            @confirm="clearLogs"
          >
            <template #reference>
              <el-button>
                <el-icon><Delete /></el-icon>{{ $t('api.logs.clear') }}
              </el-button>
            </template>
          </el-popconfirm>
        </div>

        <el-table :data="logs" v-loading="logsLoading" stripe size="small" class="nc-table">
          <el-table-column :label="$t('api.logs.time')" width="170">
            <template #default="{ row }">
              <span class="mono">{{ formatTs(row.dateTime) }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="$t('api.logs.method')" width="78">
            <template #default="{ row }">
              <span class="method-pill" :class="`method-${(row.method || '').toLowerCase()}`">
                {{ row.method }}
              </span>
            </template>
          </el-table-column>
          <el-table-column prop="path" :label="$t('api.logs.path')" min-width="180" show-overflow-tooltip>
            <template #default="{ row }"><span class="mono">{{ row.path }}</span></template>
          </el-table-column>
          <el-table-column :label="$t('api.logs.status')" width="78" align="center">
            <template #default="{ row }">
              <span class="status-pill" :class="statusClass(row.status)">{{ row.status }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="$t('api.logs.latency')" width="92">
            <template #default="{ row }"><span class="mono">{{ row.latencyMs }} ms</span></template>
          </el-table-column>
          <el-table-column prop="username" :label="$t('api.logs.username')" width="120" show-overflow-tooltip />
          <el-table-column prop="tokenDesc" :label="$t('api.logs.tokenDesc')" width="120" show-overflow-tooltip />
          <el-table-column prop="remoteIp" :label="$t('api.logs.ip')" width="140" show-overflow-tooltip>
            <template #default="{ row }"><span class="mono">{{ row.remoteIp }}</span></template>
          </el-table-column>
          <el-table-column prop="err" :label="$t('api.logs.err')" min-width="160" show-overflow-tooltip />
        </el-table>

        <el-pagination
          v-model:current-page="logPage"
          :page-size="logPageSize"
          :total="logTotal"
          layout="prev, pager, next, total"
          class="logs-pagination"
          @current-change="reloadLogs"
        />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script lang="ts" setup>
import { ref, computed, onMounted, watch } from 'vue'
import { i18n } from '@/locales'
import HttpUtils from '@/plugins/httputil'
import { Search, Plus, Delete, DocumentCopy, Connection, Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import Clipboard from 'clipboard'

const activeTab = ref<'tokens' | 'docs' | 'logs'>('tokens')

// 用 location.origin + base 拼出 v2 API 前缀,告诉 SDK 用户该往哪里发请求。
// 这是文档展示的"基础 URL",非常关键 — 用户照着抄进 curl / Postman 直接能跑。
const apiBaseUrl = computed(() => {
  const base = (window as any).BASE_URL || '/'
  const norm = base.endsWith('/') ? base : base + '/'
  return `${window.location.origin}${norm}apiv2`
})

// v1 兼容层基址 — nexcore-x-ui 主控直接对接走这里。
const apiV1BaseUrl = computed(() => {
  const base = (window as any).BASE_URL || '/'
  const norm = base.endsWith('/') ? base : base + '/'
  return `${window.location.origin}${norm}api/v1`
})

// ---------- Tokens ----------

const tokens = ref<any[]>([])
const tokensLoading = ref(false)
const tokenFilter = ref('')
const filteredTokens = computed(() => {
  const q = tokenFilter.value.trim().toLowerCase()
  if (!q) return tokens.value
  return tokens.value.filter((t) =>
    (t.desc || '').toLowerCase().includes(q) || (t.token || '').toLowerCase().includes(q),
  )
})
const latestPlainToken = ref('')

const loadTokens = async () => {
  tokensLoading.value = true
  const r = await HttpUtils.get('api/tokens')
  if (r.success) tokens.value = r.obj ?? []
  tokensLoading.value = false
}

const createTokenVisible = ref(false)
const createForm = ref({ desc: '', expiry: 30 })
const creating = ref(false)
const openCreateToken = () => {
  createForm.value = { desc: '', expiry: 30 }
  latestPlainToken.value = ''
  createTokenVisible.value = true
}
const submitCreate = async () => {
  creating.value = true
  const expiry = createForm.value.expiry > 0 ? createForm.value.expiry : 0
  const r = await HttpUtils.post('api/addToken', { desc: createForm.value.desc, expiry })
  creating.value = false
  if (r.success) {
    latestPlainToken.value = r.obj
    createTokenVisible.value = false
    loadTokens()
  }
}
const deleteToken = async (id: number) => {
  const r = await HttpUtils.post('api/deleteToken', { id })
  if (r.success) loadTokens()
}

const resetToken = async (id: number) => {
  const r = await HttpUtils.post('api/resetToken', { id })
  if (r.success) {
    latestPlainToken.value = r.obj
    loadTokens()
  }
}

const formatExpiry = (expiry: number) => {
  if (!expiry) return i18n.global.t('unlimited')
  return new Date(expiry * 1000).toLocaleString()
}

const expiryClass = (expiry: number): 'permanent' | 'active' | 'expiring' | 'expired' => {
  if (!expiry) return 'permanent'
  const now = Math.floor(Date.now() / 1000)
  if (expiry <= now) return 'expired'
  if (expiry - now < 86400 * 7) return 'expiring'
  return 'active'
}

// ---------- Docs ----------

const docsFilter = ref('')

const docGroups = [
  {
    name: 'auth',
    endpoints: [
      { method: 'GET', path: '/me', desc: 'me.desc' },
    ],
  },
  {
    name: 'data',
    endpoints: [
      { method: 'GET', path: '/load', desc: 'load.desc', params: 'lu=<unix-second>' },
      { method: 'GET', path: '/inbounds', desc: 'inbounds.desc', params: 'id=<id>' },
      { method: 'GET', path: '/outbounds', desc: 'outbounds.desc' },
      { method: 'GET', path: '/endpoints', desc: 'endpoints.desc' },
      { method: 'GET', path: '/tls', desc: 'tls.desc' },
      { method: 'GET', path: '/clients', desc: 'clients.desc', params: 'id=<id>' },
      { method: 'GET', path: '/config', desc: 'config.desc' },
      { method: 'GET', path: '/users', desc: 'users.desc' },
      { method: 'GET', path: '/settings', desc: 'settings.desc' },
      { method: 'GET', path: '/stats', desc: 'stats.desc', params: 'resource=&tag=&limit=' },
      { method: 'GET', path: '/status', desc: 'status.desc' },
      { method: 'GET', path: '/onlines', desc: 'onlines.desc' },
      { method: 'GET', path: '/logs', desc: 'logs.desc', params: 'c=<count>&l=<level>' },
      { method: 'GET', path: '/changes', desc: 'changes.desc', params: 'a=<actor>&k=<key>&c=<count>' },
      { method: 'GET', path: '/keypairs', desc: 'keypairs.desc', params: 'k=<type>&o=<options>' },
      { method: 'GET', path: '/getdb', desc: 'getdb.desc', params: 'exclude=<table>' },
      { method: 'GET', path: '/checkOutbound', desc: 'checkOutbound.desc', params: 'tag=&link=' },
      { method: 'GET', path: '/singbox-config', desc: 'singboxConfig.desc' },
    ],
  },
  {
    name: 'mut',
    endpoints: [
      { method: 'POST', path: '/save', desc: 'save.desc', params: 'object=&action=&data=&initUsers=' },
      { method: 'POST', path: '/restartApp', desc: 'restartApp.desc' },
      { method: 'POST', path: '/restartSb', desc: 'restartSb.desc' },
      { method: 'POST', path: '/linkConvert', desc: 'linkConvert.desc', params: 'link=' },
      { method: 'POST', path: '/importdb', desc: 'importdb.desc', params: 'multipart/form-data: db=' },
      { method: 'POST', path: '/changePass', desc: 'changePass.desc', params: 'id=&oldPass=&newUsername=&newPass=' },
      { method: 'POST', path: '/setting', desc: 'setting.desc', params: 'port=&path=' },
    ],
  },
  {
    name: 'tokens',
    endpoints: [
      { method: 'GET', path: '/tokens', desc: 'tokens.desc' },
      { method: 'POST', path: '/addToken', desc: 'addToken.desc', params: 'desc=&expiry=<days>' },
      { method: 'POST', path: '/deleteToken', desc: 'deleteToken.desc', params: 'id=' },
    ],
  },
  {
    name: 'audit',
    endpoints: [
      { method: 'GET', path: '/apiLogs', desc: 'apiLogs.desc', params: 'method=&path=&username=&since=&until=&limit=&offset=' },
      { method: 'POST', path: '/clearApiLogs', desc: 'clearApiLogs.desc' },
    ],
  },
]

// nexcore-x-ui 兼容层 — 路径前缀是 /api/v1/*,与 /apiv2 平行。同款 Bearer Token,
// 但响应壳是 {data}/{error,code,message,details},匹配 x-ui 主控 SDK 的预期。
const v1Endpoints = [
  { method: 'GET',    path: '/health' },
  { method: 'GET',    path: '/me' },
  { method: 'GET',    path: '/server/status' },
  { method: 'GET',    path: '/xray/status', note: i18n.global.t('api.v1note.xrayStatus') },
  { method: 'POST',   path: '/xray/restart' },
  { method: 'GET',    path: '/xray/logs' },
  { method: 'GET',    path: '/xray/config' },
  { method: 'GET',    path: '/inbounds' },
  { method: 'GET',    path: '/inbounds/:id' },
  { method: 'POST',   path: '/inbounds' },
  { method: 'PUT',    path: '/inbounds/:id' },
  { method: 'DELETE', path: '/inbounds/:id' },
  { method: 'GET',    path: '/outbounds' },
  { method: 'GET',    path: '/outbounds/:id' },
  { method: 'POST',   path: '/outbounds' },
  { method: 'PUT',    path: '/outbounds/:id' },
  { method: 'DELETE', path: '/outbounds/:id' },
  { method: 'GET',    path: '/clients' },
  { method: 'GET',    path: '/clients/:identifier/traffic' },
  { method: 'POST',   path: '/clients/:identifier/reset-traffic' },
  { method: 'GET',    path: '/online-ips' },
  { method: 'GET',    path: '/online-ips/:tag' },
  { method: 'GET',    path: '/online-ips-by-email' },
  { method: 'GET',    path: '/onlines' },
  { method: 'GET',    path: '/traffic' },
  { method: 'GET',    path: '/traffic/live' },
  { method: 'GET',    path: '/access-logs' },
  { method: 'DELETE', path: '/access-logs' },
  { method: 'GET',    path: '/settings' },
  { method: 'PATCH',  path: '/settings' },
  { method: 'GET',    path: '/tokens' },
  { method: 'POST',   path: '/tokens' },
  { method: 'DELETE', path: '/tokens/:id' },
  { method: 'POST',   path: '/system/restart-panel' },
  { method: 'POST',   path: '/sui/cloudflare/zones',         note: i18n.global.t('api.v1note.suiOnly') },
  { method: 'POST',   path: '/sui/cloudflare/dns/upsert-a',  note: i18n.global.t('api.v1note.suiOnly') },
  { method: 'POST',   path: '/sui/cloudflare/tls/issue',     note: i18n.global.t('api.v1note.suiOnly') },
  { method: 'GET',    path: '/sui/singbox/raw-config',       note: i18n.global.t('api.v1note.suiOnly') },
]

const filteredDocGroups = computed(() => {
  const q = docsFilter.value.trim().toLowerCase()
  if (!q) return docGroups
  return docGroups
    .map((g) => ({
      ...g,
      endpoints: g.endpoints.filter(
        (e) => e.path.toLowerCase().includes(q) || e.method.toLowerCase().includes(q),
      ),
    }))
    .filter((g) => g.endpoints.length > 0)
})

// ---------- Logs ----------

const logs = ref<any[]>([])
const logsLoading = ref(false)
const logFilter = ref<{ method: string; path: string; username: string }>({ method: '', path: '', username: '' })
const logPage = ref(1)
const logPageSize = 50
const logTotal = ref(0)

const reloadLogs = async () => {
  logsLoading.value = true
  const offset = (logPage.value - 1) * logPageSize
  const params: any = { limit: logPageSize, offset }
  if (logFilter.value.method) params.method = logFilter.value.method
  if (logFilter.value.path) params.path = logFilter.value.path
  if (logFilter.value.username) params.username = logFilter.value.username
  const r = await HttpUtils.get('api/apiLogs', params)
  logsLoading.value = false
  if (r.success) {
    logs.value = r.obj?.logs ?? []
    logTotal.value = r.obj?.total ?? 0
  }
}
const resetLogFilter = () => {
  logFilter.value = { method: '', path: '', username: '' }
  logPage.value = 1
  reloadLogs()
}
const clearLogs = async () => {
  const r = await HttpUtils.post('api/clearApiLogs', null)
  if (r.success) reloadLogs()
}

const formatTs = (sec: number) => {
  if (!sec) return '—'
  const d = new Date(sec * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

const statusClass = (s: number) => {
  if (s >= 500) return 'status-err'
  if (s >= 400) return 'status-warn'
  if (s >= 200) return 'status-ok'
  return ''
}

const copyCurlExample = () => {
  copyText(`curl -H "Authorization: Bearer <TOKEN>" ${apiBaseUrl.value}/me`)
}

const copyText = (txt: string) => {
  const hidden = document.createElement('button')
  hidden.className = 'clipboard-copy-btn'
  document.body.appendChild(hidden)
  const cb = new Clipboard('.clipboard-copy-btn', { text: () => txt })
  cb.on('success', () => {
    cb.destroy()
    ElMessage.success(`${i18n.global.t('success')}: ${i18n.global.t('copyToClipboard')}`)
  })
  cb.on('error', () => {
    cb.destroy()
    ElMessage.error(`${i18n.global.t('failed')}: ${i18n.global.t('copyToClipboard')}`)
  })
  hidden.click()
  document.body.removeChild(hidden)
}

watch(activeTab, (v) => {
  if (v === 'tokens') loadTokens()
  if (v === 'logs') reloadLogs()
})

onMounted(() => {
  loadTokens()
})
</script>

<style scoped>
.api-base {
  font-size: 12px;
  padding: 6px 10px;
  border-radius: var(--radius-pill);
}

.api-tabs :deep(.el-tabs__header) {
  margin-bottom: 16px;
}

.tab-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  margin-bottom: 14px;
}
.toolbar-spacer { flex: 1; }

.token-banner {
  margin-bottom: 14px;
}

.token-mask { letter-spacing: 0.04em; color: var(--nc-text-1); }

.expiry-text { color: var(--nc-text-1); }
.expiry-pill {
  display: inline-block;
  margin-left: 8px;
  font-size: 11px;
  font-weight: 600;
  padding: 1px 8px;
  border-radius: var(--radius-pill);
  letter-spacing: 0.04em;
}
.expiry-pill.permanent { color: #475569; background: #e2e8f0; }
.expiry-pill.active    { color: #16a34a; background: #dcfce7; }
.expiry-pill.expiring  { color: #d97706; background: #fef3c7; }
.expiry-pill.expired   { color: #dc2626; background: #fee2e2; }

.nc-table { background: var(--nc-surface); }

.docs-intro {
  padding: 16px 18px;
  margin-bottom: 14px;
}
.docs-intro__title {
  margin: 0 0 6px;
  font-size: 15px;
  font-weight: 600;
  color: var(--nc-text-1);
}
.docs-intro__hint {
  margin: 0 0 10px;
  font-size: 13px;
  color: var(--nc-text-muted);
}
.docs-curl {
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--nc-bg-3);
  border: 1px solid var(--nc-border-soft);
  border-radius: var(--radius-md);
  padding: 8px 10px;
}
.docs-curl__label {
  font-size: 11px;
  font-weight: 600;
  color: var(--nc-primary);
  text-transform: uppercase;
  background: var(--nc-primary-soft);
  padding: 2px 8px;
  border-radius: var(--radius-pill);
  letter-spacing: 0.04em;
}
.docs-curl code {
  flex: 1;
  font-size: 12.5px;
  color: var(--nc-text-1);
  white-space: nowrap;
  overflow-x: auto;
}

.docs-search {
  max-width: 320px;
  margin-bottom: 14px;
}

.docs-groups {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.docs-group {
  padding: 14px 16px 6px;
}
.docs-group__title {
  margin: 0 0 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--nc-text-1);
}
.docs-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.docs-table thead th {
  text-align: left;
  font-weight: 500;
  color: var(--nc-text-muted);
  padding: 6px 8px;
  border-bottom: 1px solid var(--nc-border-soft);
}
.docs-table tbody td {
  padding: 8px;
  border-bottom: 1px solid var(--nc-border-soft);
  vertical-align: top;
}
.docs-table tbody tr:last-child td { border-bottom: none; }
.col-method { width: 88px; }
.col-path { width: 240px; }
.path-cell { color: var(--nc-text-1); }
.docs-desc { color: var(--nc-text-1); }
.docs-params {
  margin-top: 4px;
  font-size: 12px;
  color: var(--nc-text-muted);
}

.method-pill {
  display: inline-block;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  padding: 2px 8px;
  border-radius: var(--radius-pill);
}
.method-get { color: #2563eb; background: #dbeafe; }
.method-post { color: #7c3aed; background: #ede9fe; }

.status-pill {
  display: inline-block;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 7px;
  border-radius: var(--radius-pill);
  font-family: var(--font-mono);
}
.status-ok { color: #16a34a; background: #dcfce7; }
.status-warn { color: #d97706; background: #fef3c7; }
.status-err { color: #dc2626; background: #fee2e2; }

.form-hint {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--nc-text-muted);
}

.logs-pagination {
  margin-top: 16px;
  justify-content: flex-end;
}

.v1-banner {
  border-color: var(--nc-primary);
  border-left: 3px solid var(--nc-primary);
}
.v1-banner .title-icon {
  margin-right: 6px;
  color: var(--nc-primary);
}
.docs-curl__label.v1-tag {
  color: #fff;
  background: var(--nc-primary);
}
.v1-list {
  margin-top: 12px;
}
.v1-list > summary {
  cursor: pointer;
  font-size: 12.5px;
  color: var(--nc-text-muted);
  user-select: none;
  padding: 6px 0;
}
.v1-list > summary:hover {
  color: var(--nc-primary);
}
.method-pill.method-put { color: #f59e0b; background: #fef3c7; }
.method-pill.method-patch { color: #14b8a6; background: #ccfbf1; }
.method-pill.method-delete { color: #dc2626; background: #fee2e2; }
</style>
