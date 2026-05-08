<template>
  <div class="page-container">
    <div class="page-header with-actions">
      <div class="page-header-text">
        <h2 class="page-title">{{ $t('pages.settings') }}</h2>
        <p class="page-desc">{{ $t('settings.desc', '面板访问、订阅与外部模板配置') }}</p>
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

        <el-tab-pane :label="$t('setting.sub')" name="t2">
          <el-form label-position="top" class="settings-form">
            <div class="settings-row">
              <el-form-item>
                <el-switch v-model="subEncode" :active-text="$t('setting.subEncode')" />
              </el-form-item>
              <el-form-item>
                <el-switch v-model="subShowInfo" :active-text="$t('setting.subInfo')" />
              </el-form-item>
            </div>
            <div class="settings-grid">
              <el-form-item :label="$t('setting.addr')">
                <el-input v-model="settings.subListen" />
              </el-form-item>
              <el-form-item :label="$t('setting.port')">
                <el-input-number v-model="subPort" :min="1" controls-position="right" style="width: 100%" />
              </el-form-item>
              <el-form-item :label="$t('setting.sslKey')">
                <el-input v-model="settings.subKeyFile" />
              </el-form-item>
              <el-form-item :label="$t('setting.sslCert')">
                <el-input v-model="settings.subCertFile" />
              </el-form-item>
              <el-form-item :label="$t('setting.domain')">
                <el-input v-model="settings.subDomain" />
              </el-form-item>
              <el-form-item :label="$t('setting.path')">
                <el-input v-model="settings.subPath" />
              </el-form-item>
              <el-form-item :label="$t('setting.update')">
                <el-input-number v-model="subUpdates" :min="0" controls-position="right" style="width: 100%" />
              </el-form-item>
              <el-form-item :label="$t('setting.subUri')">
                <el-input v-model="settings.subURI" />
              </el-form-item>
            </div>
          </el-form>
        </el-tab-pane>

        <el-tab-pane :label="$t('setting.jsonSub')" name="t3" lazy>
          <SubJsonExtVue :settings="settings" />
        </el-tab-pane>

        <el-tab-pane :label="$t('setting.clashSub')" name="t4" lazy>
          <SubClashExtVue :settings="settings" />
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { Ref, computed, inject, onMounted, ref } from 'vue'
import HttpUtils from '@/plugins/httputil'
import { FindDiff } from '@/plugins/utils'
import { i18n } from '@/locales'
import SubJsonExtVue from '@/components/SubJsonExt.vue'
import SubClashExtVue from '@/components/SubClashExt.vue'
import { ElMessage } from 'element-plus'
import { Check, RefreshRight } from '@element-plus/icons-vue'

const tab = ref('t1')
const loading: Ref<boolean> = inject('loading') ?? ref(false)
const oldSettings = ref<any>({})

const settings = ref<any>({
  webListen: '',
  webDomain: '',
  webPort: '2095',
  webCertFile: '',
  webKeyFile: '',
  webPath: '/app/',
  webURI: '',
  sessionMaxAge: '0',
  trafficAge: '30',
  timeLocation: 'Asia/Tehran',
  subListen: '',
  subPort: '2096',
  subPath: '/sub/',
  subDomain: '',
  subCertFile: '',
  subKeyFile: '',
  subUpdates: '12',
  subEncode: 'true',
  subShowInfo: 'false',
  subURI: '',
  subJsonExt: '',
  subClashExt: '',
})

onMounted(async () => {
  loading.value = true
  await loadData()
  loading.value = false
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

const subEncode = computed({
  get: () => settings.value.subEncode == 'true',
  set: (v: boolean) => { settings.value.subEncode = v ? 'true' : 'false' },
})

const subShowInfo = computed({
  get: () => settings.value.subShowInfo == 'true',
  set: (v: boolean) => { settings.value.subShowInfo = v ? 'true' : 'false' },
})

const webPort = computed({
  get: () => (settings.value.webPort.length > 0 ? parseInt(settings.value.webPort) : 2095),
  set: (v: number) => { settings.value.webPort = v > 0 ? v.toString() : '2095' },
})

const sessionMaxAge = computed({
  get: () => (settings.value.sessionMaxAge.length > 0 ? parseInt(settings.value.sessionMaxAge) : 0),
  set: (v: number) => { settings.value.sessionMaxAge = v > 0 ? v.toString() : '0' },
})

const trafficAge = computed({
  get: () => (settings.value.trafficAge.length > 0 ? parseInt(settings.value.trafficAge) : 0),
  set: (v: number) => { settings.value.trafficAge = v > 0 ? v.toString() : '0' },
})

const subPort = computed({
  get: () => (settings.value.subPort.length > 0 ? parseInt(settings.value.subPort) : 2096),
  set: (v: number) => { settings.value.subPort = v > 0 ? v.toString() : '2096' },
})

const subUpdates = computed({
  get: () => (settings.value.subUpdates.length > 0 ? parseInt(settings.value.subUpdates) : 12),
  set: (v: number) => { settings.value.subUpdates = v > 0 ? v.toString() : '12' },
})

const stateChange = computed(() => !FindDiff.deepCompare(settings.value, oldSettings.value))
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

.settings-row {
  display: flex;
  flex-wrap: wrap;
  gap: 24px;
  margin-bottom: 4px;
}
</style>
