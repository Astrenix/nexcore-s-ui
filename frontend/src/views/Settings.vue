<template>
  <div class="page-container">
    <div class="page-header with-actions">
      <div class="page-header-text">
        <h2 class="page-title">{{ $t('pages.settings') }}</h2>
        <p class="page-desc">{{ $t('settings.desc') }}</p>
      </div>
      <div class="page-header-actions">
        <el-button
          type="primary"
          :loading="loading"
          :disabled="!stateChange"
          @click="save"
        >
          <el-icon><Check /></el-icon>{{ $t('actions.save') }}
        </el-button>
        <el-button
          type="warning"
          plain
          :loading="loading"
          :disabled="stateChange"
          @click="restartApp"
        >
          <el-icon><RefreshRight /></el-icon>{{ $t('actions.restartApp') }}
        </el-button>
      </div>
    </div>

    <div class="nc-tabs settings-tabs">
      <el-tabs v-model="tab">
        <el-tab-pane :label="$t('setting.interface')" name="t1">
          <el-form label-position="top" class="settings-form">
            <div class="settings-grid">
              <el-form-item :label="$t('setting.ui.nodeName')">
                <el-input v-model="settings.nodeName" :placeholder="$t('setting.ui.nodeNamePlaceholder')" />
              </el-form-item>
              <el-form-item :label="$t('setting.ui.addrSourceLabel')" class="form-item--full">
                <el-radio-group v-model="settings.linkAddrSource">
                  <el-radio value="panel">{{ $t('setting.ui.addrPanel') }}</el-radio>
                  <el-radio value="ip">{{ $t('inbounds.addrSrc.ip') }}</el-radio>
                  <el-radio value="tls">{{ $t('setting.ui.addrTls') }}</el-radio>
                </el-radio-group>
                <p class="form-hint">
                  {{ $t('setting.ui.addrHint1') }}<code class="mono">add</code>{{ $t('setting.ui.addrHint2') }}<br>
                  <b>{{ $t('inbounds.addrSrc.panel') }}</b>{{ $t('setting.ui.addrHint3') }}<br>
                  <b>{{ $t('inbounds.addrSrc.ip') }}</b>{{ $t('setting.ui.addrHint4') }}<code class="mono">{{ $t('setting.ui.panelIp') }}</code>{{ $t('setting.ui.addrHint5') }}<br>
                  <b>TLS server_name</b>{{ $t('setting.ui.addrHint6') }}<br>
                  {{ $t('setting.ui.addrHint7') }}
                </p>
              </el-form-item>
              <el-form-item v-if="settings.linkAddrSource === 'ip'" :label="$t('setting.ui.panelIp')" class="form-item--full">
                <el-input v-model="settings.panelIp" :placeholder="$t('setting.ui.panelIpPlaceholder')" />
              </el-form-item>
              <el-form-item :label="$t('setting.addr')">
                <el-input v-model="settings.webListen" />
              </el-form-item>
              <el-form-item :label="$t('setting.port')">
                <el-input v-model.number="webPort" type="number" :min="1" />
              </el-form-item>
              <el-form-item :label="$t('setting.webPath')">
                <el-input v-model="settings.webPath" />
              </el-form-item>
              <el-form-item :label="$t('setting.domain')">
                <el-input v-model="settings.webDomain" />
              </el-form-item>
              <el-form-item :label="$t('setting.sslKey')">
                <el-input v-model="settings.webKeyFile" />
              </el-form-item>
              <el-form-item :label="$t('setting.sslCert')">
                <el-input v-model="settings.webCertFile" />
              </el-form-item>
              <!-- 证书状态:到期时间 + 剩余天数 + 一键续签 -->
              <el-form-item :label="$t('setting.sslStatus')" class="form-item--full">
                <div v-loading="certLoading" class="cert-status">
                  <template v-if="certInfo && certInfo.configured && !certInfo.error">
                    <div class="cert-status__row">
                      <el-tag :type="certStatusType" effect="dark" size="small">
                        <span v-if="certInfo.expired">{{ $t('setting.sslExpired') }}</span>
                        <span v-else>{{ $t('setting.sslDaysLeft', { n: certInfo.daysLeft }) }}</span>
                      </el-tag>
                      <el-button
                        type="primary"
                        size="small"
                        :loading="renewing"
                        @click="renewSsl"
                      >
                        {{ $t('setting.sslRenew') }}
                      </el-button>
                    </div>
                    <div class="cert-status__meta">
                      <span v-if="certInfo.domains && certInfo.domains.length">
                        {{ certInfo.domains.join(', ') }}
                      </span>
                      <span>{{ $t('setting.sslNotAfter') }}: {{ fmtCertDate(certInfo.notAfter) }}</span>
                      <span v-if="certInfo.issuer">{{ certInfo.issuer }}</span>
                    </div>
                  </template>
                  <span v-else-if="certInfo && certInfo.error" class="cert-status__err">
                    {{ certInfo.error }}
                  </span>
                  <span v-else class="cert-status__none">{{ $t('setting.sslNone') }}</span>
                </div>
              </el-form-item>
              <!-- CF 自动签发面板 SSL — 走跟入站 TLS 一致的 wizard 流程 -->
              <el-form-item :label="$t('setting.ui.autoHttps')" class="form-item--full">
                <div class="auto-ssl">
                  <span class="auto-ssl__hint">
                    {{ $t('setting.ui.autoHttpsHint') }}
                  </span>
                  <el-button type="primary" @click="cfPanelWizardVisible = true">
                    {{ $t('setting.ui.openWizard') }}
                  </el-button>
                </div>
              </el-form-item>
              <el-form-item :label="$t('setting.webUri')">
                <el-input v-model="settings.webURI" />
              </el-form-item>
              <el-form-item :label="`${$t('setting.sessionAge')} (${$t('date.m')})`">
                <el-input-number v-model="sessionMaxAge" :min="0" controls-position="right" style="width: 100%" />
              </el-form-item>
              <el-form-item :label="`${$t('setting.trafficAge')} (${$t('date.d')})`">
                <el-input-number v-model="trafficAge" :min="0" controls-position="right" style="width: 100%" />
              </el-form-item>
              <el-form-item :label="$t('setting.timeLoc')">
                <el-input v-model="settings.timeLocation" />
              </el-form-item>
            </div>
          </el-form>
        </el-tab-pane>

        <el-tab-pane :label="$t('setting.kernel')" name="t6">
          <p class="kernel-intro">
            {{ $t('setting.ui.kernelIntro') }}
          </p>

          <el-collapse v-model="kernelActive" class="kernel-collapse">
            <!-- 日志 -->
            <el-collapse-item name="log">
              <template #title>
                <span class="kernel-section-title">{{ $t('setting.ui.logTitle') }}</span>
                <span class="kernel-section-sub">{{ $t('setting.ui.logSub') }}</span>
              </template>
              <div class="kernel-fields">
                <div class="kernel-field">
                  <div class="kernel-field-row">
                    <label>{{ $t('basic.log.level') }}</label>
                    <el-select v-model="kernel.log.level" clearable style="width: 220px">
                      <el-option v-for="l in logLevels" :key="l" :label="l" :value="l" />
                    </el-select>
                  </div>
                  <p class="kernel-hint">{{ $t('setting.ui.logLevelHintA') }}<code>info</code>{{ $t('setting.ui.logLevelHintB') }}<code>debug</code>{{ $t('setting.ui.logLevelHintC') }}<code>trace</code>{{ $t('setting.ui.logLevelHintD') }}<code>error</code>{{ $t('setting.ui.logLevelHintE') }}</p>
                </div>
                <div class="kernel-field">
                  <div class="kernel-field-row">
                    <label>{{ $t('basic.log.output') }}</label>
                    <el-input v-model="kernel.log.output" :placeholder="$t('setting.ui.logOutputPlaceholder')" style="width: 320px" />
                  </div>
                  <p class="kernel-hint">{{ $t('setting.ui.logOutputHintA') }}<code>/var/log/sing-box.log</code>{{ $t('setting.ui.logOutputHintB') }}</p>
                </div>
                <div class="kernel-field kernel-field--inline">
                  <el-switch v-model="kernel.log.timestamp" />
                  <div>
                    <label>{{ $t('basic.log.timestamp') }}</label>
                    <p class="kernel-hint">{{ $t('setting.ui.logTimestampHint') }}</p>
                  </div>
                </div>
                <div class="kernel-field kernel-field--inline">
                  <el-switch v-model="kernel.log.disabled" />
                  <div>
                    <label>{{ $t('disable') }}</label>
                    <p class="kernel-hint">{{ $t('setting.ui.logDisableHint') }}</p>
                  </div>
                </div>
              </div>
            </el-collapse-item>

            <!-- NTP -->
            <el-collapse-item name="ntp">
              <template #title>
                <span class="kernel-section-title">{{ $t('setting.ui.ntpTitle') }}</span>
                <span class="kernel-section-sub">{{ $t('setting.ui.ntpSub') }}</span>
              </template>
              <div class="kernel-fields">
                <div class="kernel-field kernel-field--inline">
                  <el-switch v-model="kernel.ntpEnabled" />
                  <div>
                    <label>{{ $t('enable') }}</label>
                    <p class="kernel-hint">{{ $t('setting.ui.ntpEnableHintA') }}<strong>{{ $t('setting.ui.ntpEnableHintStrong') }}</strong>{{ $t('setting.ui.ntpEnableHintB') }}</p>
                  </div>
                </div>
                <template v-if="kernel.ntpEnabled">
                  <div class="kernel-field">
                    <div class="kernel-field-row">
                      <label>{{ $t('setting.ui.ntpServer') }}</label>
                      <el-input v-model="kernel.ntp.server" placeholder="time.apple.com" style="width: 240px" />
                    </div>
                    <p class="kernel-hint">{{ $t('setting.ui.ntpServerHintA') }}<code>time.apple.com</code>{{ $t('setting.ui.ntpServerHintSep') }}<code>pool.ntp.org</code>{{ $t('setting.ui.ntpServerHintSep') }}<code>ntp.aliyun.com</code>{{ $t('setting.ui.ntpServerHintEnd') }}</p>
                  </div>
                  <div class="kernel-field">
                    <div class="kernel-field-row">
                      <label>{{ $t('in.port') }}</label>
                      <el-input-number v-model="kernel.ntp.server_port" :min="1" :max="65535" controls-position="right" style="width: 160px" />
                    </div>
                    <p class="kernel-hint">{{ $t('setting.ui.ntpPortHintA') }}<code>123</code>{{ $t('setting.ui.ntpPortHintB') }}</p>
                  </div>
                  <div class="kernel-field">
                    <div class="kernel-field-row">
                      <label>{{ $t('setting.ui.ntpInterval') }}</label>
                      <el-input-number v-model="kernel.ntpIntervalMin" :min="0" controls-position="right" style="width: 160px" />
                    </div>
                    <p class="kernel-hint">{{ $t('setting.ui.ntpIntervalHintA') }}<code>30</code>{{ $t('setting.ui.ntpIntervalHintB') }}</p>
                  </div>
                </template>
              </div>
            </el-collapse-item>

            <!-- Experimental -->
            <el-collapse-item name="exp">
              <template #title>
                <span class="kernel-section-title">{{ $t('setting.ui.expTitle') }}</span>
                <span class="kernel-section-sub">{{ $t('setting.ui.expSub') }}</span>
              </template>

              <h4 class="kernel-sub">{{ $t('setting.ui.cacheTitle') }}</h4>
              <div class="kernel-fields">
                <div class="kernel-field kernel-field--inline">
                  <el-switch v-model="kernel.cacheEnabled" />
                  <div>
                    <label>{{ $t('enable') }}</label>
                    <p class="kernel-hint">{{ $t('setting.ui.cacheEnableHint') }}</p>
                  </div>
                </div>
                <template v-if="kernel.cacheEnabled">
                  <div class="kernel-field">
                    <div class="kernel-field-row">
                      <label>{{ $t('setting.ui.cachePath') }}</label>
                      <el-input v-model="kernel.cache.path" :placeholder="$t('setting.ui.cachePathPlaceholder')" style="width: 320px" />
                    </div>
                    <p class="kernel-hint">{{ $t('setting.ui.cachePathHint') }}</p>
                  </div>
                  <div class="kernel-field kernel-field--inline">
                    <el-switch v-model="kernel.cache.store_fakeip" />
                    <div>
                      <label>{{ $t('setting.ui.storeFakeip') }}</label>
                      <p class="kernel-hint">{{ $t('setting.ui.storeFakeipHint') }}</p>
                    </div>
                  </div>
                </template>
              </div>

              <h4 class="kernel-sub">Clash API</h4>
              <div class="kernel-fields">
                <div class="kernel-field kernel-field--inline">
                  <el-switch v-model="kernel.clashEnabled" />
                  <div>
                    <label>{{ $t('enable') }}</label>
                    <p class="kernel-hint">{{ $t('setting.ui.clashEnableHintA') }}<strong>{{ $t('setting.ui.clashEnableHintStrong') }}</strong>{{ $t('setting.ui.clashEnableHintB') }}</p>
                  </div>
                </div>
                <template v-if="kernel.clashEnabled">
                  <div class="kernel-field">
                    <div class="kernel-field-row">
                      <label>{{ $t('setting.ui.listen') }}</label>
                      <el-input v-model="kernel.clash.external_controller" placeholder="127.0.0.1:9090" style="width: 240px" />
                    </div>
                    <p class="kernel-hint">{{ $t('setting.ui.clashListenHintA') }}<code>127.0.0.1</code>{{ $t('setting.ui.clashListenHintB') }}<code>0.0.0.0</code>{{ $t('setting.ui.clashListenHintC') }}</p>
                  </div>
                  <div class="kernel-field">
                    <div class="kernel-field-row">
                      <label>{{ $t('setting.ui.accessSecret') }}</label>
                      <el-input v-model="kernel.clash.secret" type="password" show-password :placeholder="$t('setting.ui.stronglyRecommend')" style="width: 320px" />
                    </div>
                    <p class="kernel-hint">{{ $t('setting.ui.clashSecretHintA') }}<strong>{{ $t('setting.ui.clashSecretHintStrong') }}</strong>{{ $t('setting.ui.clashSecretHintB') }}</p>
                  </div>
                </template>
              </div>

              <h4 class="kernel-sub">V2Ray API · gRPC stats</h4>
              <div class="kernel-fields">
                <div class="kernel-field kernel-field--inline">
                  <el-switch v-model="kernel.v2rayEnabled" />
                  <div>
                    <label>{{ $t('enable') }}</label>
                    <p class="kernel-hint">{{ $t('setting.ui.v2rayEnableHintA') }}<strong>{{ $t('setting.ui.v2rayEnableHintStrong') }}</strong>{{ $t('setting.ui.v2rayEnableHintB') }}</p>
                  </div>
                </div>
                <template v-if="kernel.v2rayEnabled">
                  <div class="kernel-field">
                    <div class="kernel-field-row">
                      <label>{{ $t('setting.ui.listen') }}</label>
                      <el-input v-model="kernel.v2ray.listen" placeholder="127.0.0.1:8080" style="width: 240px" />
                    </div>
                    <p class="kernel-hint">{{ $t('setting.ui.v2rayListenHintA') }}<code>127.0.0.1</code>{{ $t('setting.ui.v2rayListenHintB') }}</p>
                  </div>
                  <div class="kernel-field kernel-field--inline">
                    <el-switch v-model="kernel.v2ray.stats.enabled" />
                    <div>
                      <label>{{ $t('setting.ui.enableStats') }}</label>
                      <p class="kernel-hint">{{ $t('setting.ui.v2rayStatsHint') }}</p>
                    </div>
                  </div>
                </template>
              </div>
            </el-collapse-item>
          </el-collapse>

          <div class="kernel-save">
            <el-button type="primary" :loading="kernelSaving" :disabled="!kernelDirty" @click="saveKernel">
              <el-icon><Check /></el-icon>{{ $t('actions.save') }}
            </el-button>
            <span v-if="kernelDirty" class="kernel-dirty-hint">{{ $t('setting.ui.unsaved') }}</span>
          </div>
        </el-tab-pane>

        <el-tab-pane :label="$t('setting.account')" name="t5">
          <el-form
            ref="accountFormRef"
            :model="account"
            :rules="accountRules"
            label-position="top"
            class="settings-form"
          >
            <div class="settings-grid">
              <el-form-item :label="$t('setting.currentUser')">
                <el-input :model-value="account.currentUsername" disabled />
              </el-form-item>
              <el-form-item :label="$t('admin.oldPass')" prop="oldPass">
                <el-input v-model="account.oldPass" type="password" show-password autocomplete="current-password" />
              </el-form-item>
              <el-form-item :label="$t('admin.newUname')" prop="newUsername">
                <el-input v-model="account.newUsername" autocomplete="username" />
              </el-form-item>
              <el-form-item :label="$t('admin.newPass')" prop="newPass">
                <el-input v-model="account.newPass" type="password" show-password autocomplete="new-password" />
              </el-form-item>
            </div>
            <div>
              <el-button type="primary" :loading="accountSaving" :disabled="!accountReady" @click="saveAccount">
                <el-icon><Check /></el-icon>{{ $t('actions.save') }}
              </el-button>
            </div>
          </el-form>
        </el-tab-pane>
      </el-tabs>
    </div>

    <CloudflareTls
      v-if="cfPanelWizardVisible"
      v-model="cfPanelWizardVisible"
      :visible="cfPanelWizardVisible"
      mode="panel"
      @close="cfPanelWizardVisible = false"
    />
  </div>
</template>

<script lang="ts" setup>
import { Ref, computed, defineAsyncComponent, inject, onMounted, ref } from 'vue'

const CloudflareTls = defineAsyncComponent(() => import('@/layouts/modals/CloudflareTls.vue'))
import HttpUtils from '@/plugins/httputil'
import { FindDiff } from '@/plugins/utils'
import { i18n } from '@/locales'
import Data from '@/store/modules/data'
import { ElMessage } from 'element-plus'
import { Check, RefreshRight } from '@element-plus/icons-vue'
import type { FormInstance, FormRules } from 'element-plus'

const tab = ref('t1')
const loading: Ref<boolean> = inject('loading') ?? ref(false)
const oldSettings = ref<any>({})

// 账号修改
const accountFormRef = ref<FormInstance>()
const accountSaving = ref(false)
const account = ref({
  id: 0,
  currentUsername: localStorage.getItem('admin_username') ?? '',
  oldPass: '',
  newUsername: '',
  newPass: '',
})
const accountReady = computed(() =>
  account.value.id > 0 &&
  account.value.oldPass.length > 0 &&
  account.value.newUsername.length > 0 &&
  account.value.newPass.length > 0,
)
const accountRules: FormRules = {
  oldPass:     [{ required: true, message: () => i18n.global.t('login.pwRules'), trigger: 'blur' }],
  newUsername: [{ required: true, message: () => i18n.global.t('login.unRules'), trigger: 'blur' }],
  newPass:     [{ required: true, message: () => i18n.global.t('login.pwRules'), trigger: 'blur' }],
}

const loadAccount = async () => {
  const msg = await HttpUtils.get('api/users')
  if (!msg.success || !Array.isArray(msg.obj)) return
  const stored = localStorage.getItem('admin_username')
  const matched = stored ? msg.obj.find((u: any) => u.username === stored) : null
  const u = matched ?? msg.obj[0]
  if (u) {
    account.value.id = u.id
    account.value.currentUsername = u.username
    account.value.newUsername = u.username
  }
}

const saveAccount = async () => {
  if (!accountFormRef.value) return
  await accountFormRef.value.validate(async (valid) => {
    if (!valid) return
    accountSaving.value = true
    const r = await HttpUtils.post('api/changePass', {
      id: account.value.id,
      oldPass: account.value.oldPass,
      newUsername: account.value.newUsername,
      newPass: account.value.newPass,
    })
    accountSaving.value = false
    if (r.success) {
      ElMessage.success(i18n.global.t('success'))
      localStorage.setItem('admin_username', account.value.newUsername)
      account.value.currentUsername = account.value.newUsername
      account.value.oldPass = ''
      account.value.newPass = ''
    }
  })
}

const settings = ref<any>({
  webListen: '',
  webDomain: '',
  webPort: '3095',
  webCertFile: '',
  webKeyFile: '',
  webPath: '/app/',
  webURI: '',
  sessionMaxAge: '0',
  trafficAge: '30',
  timeLocation: 'Asia/Tehran',
  nodeName: '',
  linkAddrSource: 'panel',
  panelIp: '',
})

onMounted(async () => {
  loading.value = true
  await Promise.all([loadData(), loadAccount(), loadKernel()])
  loading.value = false
  // 证书信息独立拉,不阻塞主设置加载
  loadCertInfo()
})

const loadData = async () => {
  loading.value = true
  const msg = await HttpUtils.get('api/settings')
  loading.value = false
  if (msg.success) setData(msg.obj)
}

const setData = (data: any) => {
  settings.value = data
  oldSettings.value = { ...data }
}

const save = async () => {
  loading.value = true
  const msg = await HttpUtils.post('api/save', {
    object: 'settings',
    action: 'set',
    data: JSON.stringify(settings.value),
  })
  if (msg.success) {
    ElMessage.success(`${i18n.global.t('success')}: ${i18n.global.t('actions.set')} ${i18n.global.t('pages.settings')}`)
    setData(msg.obj.settings)
  }
  loading.value = false
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))

// 面板 SSL 签发 wizard — 跟入站 TLS 一致的多步流程,复用 CloudflareTls.vue
const cfPanelWizardVisible = ref(false)

// ---- 面板 HTTPS 证书状态(到期时间 + 一键续签)----
interface CertInfo {
  configured: boolean
  domains?: string[]
  issuer?: string
  notBefore?: number
  notAfter?: number
  daysLeft: number
  expired: boolean
  error?: string
}
const certInfo = ref<CertInfo | null>(null)
const certLoading = ref(false)
const renewing = ref(false)

const loadCertInfo = async () => {
  certLoading.value = true
  const msg = await HttpUtils.get('api/panelCertInfo')
  certLoading.value = false
  if (msg.success) certInfo.value = msg.obj as CertInfo
}

// 到期时间格式化(本地时区)
const fmtCertDate = (unix?: number) => {
  if (!unix) return '-'
  return new Date(unix * 1000).toLocaleString()
}
// 剩余天数配色:>30 天绿、7~30 天橙、<7 天/已过期红
const certStatusType = computed(() => {
  const c = certInfo.value
  if (!c || !c.configured) return 'info'
  if (c.expired || c.daysLeft < 7) return 'danger'
  if (c.daysLeft < 30) return 'warning'
  return 'success'
})

const renewSsl = async () => {
  renewing.value = true
  const tip = ElMessage({
    type: 'info',
    duration: 0,
    showClose: false,
    message: i18n.global.t('setting.ui.renewing'),
  })
  try {
    const r = await HttpUtils.post('api/panelSslRenew', {})
    if (r.success) {
      ElMessage.success(i18n.global.t('setting.ui.renewSuccess'))
      await sleep(4000)
      window.location.reload()
    }
  } finally {
    tip.close()
    renewing.value = false
  }
}

const restartApp = async () => {
  loading.value = true
  const msg = await HttpUtils.post('api/restartApp', {})
  if (msg.success) {
    let url = settings.value.webURI
    if (url !== '') {
      const isTLS = settings.value.webCertFile !== '' || settings.value.webKeyFile !== ''
      url = buildURL(settings.value.webDomain, settings.value.webPort.toString(), isTLS, settings.value.webPath)
    }
    await sleep(3000)
    window.location.replace(url)
  }
  loading.value = false
}

const buildURL = (host: string, port: string, isTLS: boolean, path: string) => {
  if (!host || host.length == 0) host = window.location.hostname
  if (!port || port.length == 0) port = window.location.port
  const protocol = isTLS ? 'https:' : 'http:'
  if (port === '' || (isTLS && port === '443') || (!isTLS && port === '80')) port = ''
  else port = `:${port}`
  return `${protocol}//${host}${port}${path}settings`
}

const webPort = computed({
  get: () => (settings.value.webPort.length > 0 ? parseInt(settings.value.webPort) : 3095),
  set: (v: number) => { settings.value.webPort = v > 0 ? v.toString() : '3095' },
})

const sessionMaxAge = computed({
  get: () => (settings.value.sessionMaxAge.length > 0 ? parseInt(settings.value.sessionMaxAge) : 0),
  set: (v: number) => { settings.value.sessionMaxAge = v > 0 ? v.toString() : '0' },
})

const trafficAge = computed({
  get: () => (settings.value.trafficAge.length > 0 ? parseInt(settings.value.trafficAge) : 0),
  set: (v: number) => { settings.value.trafficAge = v > 0 ? v.toString() : '0' },
})

const stateChange = computed(() => !FindDiff.deepCompare(settings.value, oldSettings.value))

// 内核(sing-box)参数 — Log / NTP / Experimental
const logLevels = ['trace', 'debug', 'info', 'warn', 'error', 'fatal', 'panic']
const kernelActive = ref<string[]>(['log'])
const kernelSaving = ref(false)
const kernelOriginal = ref<string>('{}')
const kernel = ref<any>({
  log: { disabled: false, level: 'info', output: '', timestamp: false },
  ntpEnabled: false,
  ntp: { server: 'time.apple.com', server_port: 123 },
  ntpIntervalMin: 30,
  cacheEnabled: false,
  cache: { path: '', store_fakeip: false },
  clashEnabled: false,
  clash: { external_controller: '127.0.0.1:9090', secret: '' },
  v2rayEnabled: false,
  v2ray: { listen: '127.0.0.1:8080', stats: { enabled: false, inbounds: [], outbounds: [], users: [] } },
})

const kernelDirty = computed(() => JSON.stringify(kernel.value) !== kernelOriginal.value)

const snapshotKernel = () => { kernelOriginal.value = JSON.stringify(kernel.value) }

const loadKernel = async () => {
  while (Data().lastLoad === 0) await new Promise((r) => setTimeout(r, 100))
  const cfg: any = Data().config ?? {}

  if (cfg.log) {
    kernel.value.log = {
      disabled: !!cfg.log.disabled,
      level: cfg.log.level || 'info',
      output: cfg.log.output || '',
      timestamp: !!cfg.log.timestamp,
    }
  }

  if (cfg.ntp?.enabled) {
    kernel.value.ntpEnabled = true
    kernel.value.ntp.server = cfg.ntp.server || 'time.apple.com'
    kernel.value.ntp.server_port = cfg.ntp.server_port || 123
    kernel.value.ntpIntervalMin = cfg.ntp.interval ? parseInt(String(cfg.ntp.interval).replace(/m$/, '')) || 30 : 30
  }

  const exp: any = cfg.experimental ?? {}
  if (exp.cache_file?.enabled) {
    kernel.value.cacheEnabled = true
    kernel.value.cache.path = exp.cache_file.path || ''
    kernel.value.cache.store_fakeip = !!exp.cache_file.store_fakeip
  }
  if (exp.clash_api) {
    kernel.value.clashEnabled = true
    kernel.value.clash.external_controller = exp.clash_api.external_controller || '127.0.0.1:9090'
    kernel.value.clash.secret = exp.clash_api.secret || ''
  }
  if (exp.v2ray_api) {
    kernel.value.v2rayEnabled = true
    kernel.value.v2ray.listen = exp.v2ray_api.listen || '127.0.0.1:8080'
    kernel.value.v2ray.stats = exp.v2ray_api.stats || { enabled: false, inbounds: [], outbounds: [], users: [] }
  }

  snapshotKernel()
}

const saveKernel = async () => {
  kernelSaving.value = true
  // 深拷贝 store.config 再改,避免就地污染 Pinia store —— 否则未保存/保存失败时
  // 脏字段已经进了 store,还会被别的页面的整份保存顺带持久化("幽灵保存")。
  const cfg: any = JSON.parse(JSON.stringify(Data().config ?? {}))
  cfg.log = { ...kernel.value.log }

  if (kernel.value.ntpEnabled) {
    cfg.ntp = {
      enabled: true,
      server: kernel.value.ntp.server,
      server_port: kernel.value.ntp.server_port,
      interval: (kernel.value.ntpIntervalMin > 0 ? kernel.value.ntpIntervalMin : 30) + 'm',
    }
  } else {
    delete cfg.ntp
  }

  if (!cfg.experimental) cfg.experimental = {}
  if (kernel.value.cacheEnabled) {
    cfg.experimental.cache_file = {
      enabled: true,
      ...(kernel.value.cache.path ? { path: kernel.value.cache.path } : {}),
      store_fakeip: !!kernel.value.cache.store_fakeip,
    }
  } else {
    delete cfg.experimental.cache_file
  }
  if (kernel.value.clashEnabled) {
    cfg.experimental.clash_api = {
      external_controller: kernel.value.clash.external_controller,
      ...(kernel.value.clash.secret ? { secret: kernel.value.clash.secret } : {}),
    }
  } else {
    delete cfg.experimental.clash_api
  }
  if (kernel.value.v2rayEnabled) {
    cfg.experimental.v2ray_api = {
      listen: kernel.value.v2ray.listen,
      stats: kernel.value.v2ray.stats,
    }
  } else {
    delete cfg.experimental.v2ray_api
  }
  if (Object.keys(cfg.experimental).length === 0) delete cfg.experimental

  const ok = await Data().save('config', 'set', cfg)
  kernelSaving.value = false
  if (ok) snapshotKernel()
}
</script>

<style scoped>
.settings-tabs :deep(.el-tabs__header) {
  margin: 0;
  padding: 0 20px;
  background: #f8fafc;
  border-bottom: 1px solid var(--nc-border);
}
.settings-tabs :deep(.el-tabs__nav-wrap::after) { display: none; }
.settings-tabs :deep(.el-tabs__item) {
  height: 44px;
  font-size: 13px;
  color: var(--nc-text-muted);
}
.settings-tabs :deep(.el-tabs__item.is-active) {
  color: var(--nc-primary);
  font-weight: 600;
}
.settings-tabs :deep(.el-tabs__active-bar) {
  background-color: var(--nc-primary);
  height: 2px;
}
.settings-tabs :deep(.el-tabs__content) {
  padding: 20px;
}
.settings-tabs {
  background: #fff;
  border: 1px solid var(--nc-border);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.settings-form {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.settings-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 8px 16px;
}
/* 跨行的项(如 CF 自动 SSL 那条)用 grid-column: 1/-1 拉到一行 */
.settings-grid :deep(.form-item--full) { grid-column: 1 / -1; }
.auto-ssl {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding: 10px 12px;
  background: linear-gradient(90deg, rgba(124, 58, 237, 0.06), transparent);
  border: 1px dashed rgba(124, 58, 237, 0.4);
  border-radius: var(--radius-md);
  width: 100%;
}
.auto-ssl__hint { font-size: 12.5px; color: var(--nc-text-1); flex: 1; min-width: 200px; }
.auto-ssl__warn { font-size: 12px; color: var(--nc-warning); margin: 4px 0 0; }

.cert-status { width: 100%; }
.cert-status__row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.cert-status__meta {
  display: flex;
  gap: 14px;
  flex-wrap: wrap;
  margin-top: 6px;
  font-size: 12px;
  color: var(--nc-text-2);
}
.cert-status__err { font-size: 12.5px; color: var(--nc-danger); }
.cert-status__none { font-size: 12.5px; color: var(--nc-text-2); }

.settings-row {
  display: flex;
  flex-wrap: wrap;
  gap: 24px;
  margin-bottom: 4px;
}

/* 内核 tab */
.kernel-intro {
  margin: 0 0 14px;
  font-size: 12.5px;
  color: var(--nc-text-muted);
  padding: 10px 12px;
  background: var(--nc-primary-soft);
  border-left: 3px solid var(--nc-primary);
  border-radius: var(--radius-md);
}
.kernel-collapse {
  background: #fff;
  border: 1px solid var(--nc-border);
  border-radius: var(--radius-lg);
  overflow: hidden;
}
.kernel-collapse :deep(.el-collapse-item__header) {
  padding: 0 16px;
  height: 48px;
  background: #f8fafc;
  border-bottom: 1px solid var(--nc-border-soft);
  font-size: 13px;
  display: flex;
  align-items: center;
  gap: 12px;
}
.kernel-collapse :deep(.el-collapse-item__wrap) {
  border: none;
  padding: 14px 16px 16px;
}
.kernel-section-title { font-weight: 600; color: var(--nc-text-1); }
.kernel-section-sub { font-size: 12px; color: var(--nc-text-muted); font-weight: 400; }
.kernel-sub {
  margin: 14px 0 8px;
  font-size: 12.5px;
  font-weight: 600;
  color: var(--nc-text-1);
  padding-bottom: 4px;
  border-bottom: 1px dashed var(--nc-border-soft);
}
.kernel-fields { display: flex; flex-direction: column; gap: 14px; }
.kernel-field { display: flex; flex-direction: column; gap: 4px; }
.kernel-field-row { display: flex; align-items: center; gap: 12px; }
.kernel-field-row label { font-size: 13px; font-weight: 500; color: var(--nc-text-1); min-width: 120px; }
.kernel-field--inline { flex-direction: row; align-items: flex-start; gap: 12px; }
.kernel-field--inline > div { flex: 1; }
.kernel-field--inline label { display: block; font-size: 13px; font-weight: 500; color: var(--nc-text-1); margin-bottom: 2px; }
.kernel-hint { margin: 0; font-size: 12px; color: var(--nc-text-muted); line-height: 1.55; }
.kernel-hint code {
  font-family: var(--font-mono);
  font-size: 11.5px;
  background: var(--nc-bg-3);
  padding: 1px 6px;
  border-radius: 4px;
  color: var(--nc-text-1);
}
.kernel-hint strong { color: var(--nc-text-1); font-weight: 600; }
.kernel-save {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 16px;
}
.kernel-dirty-hint {
  font-size: 12px;
  color: var(--nc-warning, #d97706);
}
</style>
