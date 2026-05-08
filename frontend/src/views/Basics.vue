<template>
  <div class="page-container">
    <div class="page-header with-actions">
      <div class="page-header-text">
        <h2 class="page-title">{{ $t('pages.basics') }}</h2>
        <p class="page-desc">{{ $t('basics.desc', 'Sing-Box 基础参数:Log / NTP / Experimental') }}</p>
      </div>
      <div class="page-header-actions">
        <el-button type="warning" plain :loading="loading" :disabled="!stateChange" @click="saveConfig">
          <el-icon><Check /></el-icon>{{ $t('actions.save') }}
        </el-button>
      </div>
    </div>

    <el-collapse v-model="active">
      <el-collapse-item :title="$t('basic.log.title')" name="log">
        <el-form v-if="appConfig.log" label-position="top">
          <div class="form-grid">
            <el-form-item :label="$t('disable')">
              <el-switch v-model="appConfig.log.disabled" />
            </el-form-item>
            <el-form-item :label="$t('basic.log.level')">
              <el-select v-model="appConfig.log.level" clearable>
                <el-option v-for="l in levels" :key="l" :label="l" :value="l" />
              </el-select>
            </el-form-item>
            <el-form-item :label="$t('basic.log.output')">
              <el-input v-model="appConfig.log.output" />
            </el-form-item>
            <el-form-item :label="$t('basic.log.timestamp')">
              <el-switch v-model="appConfig.log.timestamp" />
            </el-form-item>
          </div>
        </el-form>
      </el-collapse-item>

      <el-collapse-item title="NTP" name="ntp">
        <el-form label-position="top">
          <div class="form-grid">
            <el-form-item :label="$t('enable')">
              <el-switch v-model="enableNtp" />
            </el-form-item>
            <template v-if="appConfig.ntp?.enabled">
              <el-form-item :label="$t('out.addr')">
                <el-input v-model="appConfig.ntp.server" />
              </el-form-item>
              <el-form-item :label="$t('out.port')">
                <el-input-number v-model="appConfig.ntp.server_port" :min="1" controls-position="right" style="width: 100%" />
              </el-form-item>
              <el-form-item :label="`${$t('ruleset.interval')} (${$t('date.m')})`">
                <el-input-number v-model="ntpInterval" :min="0" controls-position="right" style="width: 100%" />
              </el-form-item>
            </template>
          </div>
        </el-form>
      </el-collapse-item>

      <el-collapse-item v-if="appConfig.experimental" title="Experimental" name="exp">
        <div class="nc-divider"><span>Cache File</span></div>
        <el-form label-position="top">
          <div class="form-grid">
            <el-form-item :label="$t('enable')">
              <el-switch v-model="enableCacheFile" />
            </el-form-item>
            <template v-if="appConfig.experimental.cache_file">
              <el-form-item :label="$t('transport.path')">
                <el-input v-model="appConfig.experimental.cache_file.path" />
              </el-form-item>
              <el-form-item label="Cache ID">
                <el-input v-model="appConfig.experimental.cache_file.cache_id" />
              </el-form-item>
              <el-form-item :label="$t('basic.exp.storeFakeIp')">
                <el-switch v-model="appConfig.experimental.cache_file.store_fakeip" />
              </el-form-item>
            </template>
          </div>
        </el-form>

        <div class="nc-divider"><span>Clash API</span></div>
        <el-form label-position="top">
          <div class="form-grid">
            <el-form-item :label="$t('enable')">
              <el-switch v-model="enableClashApi" />
            </el-form-item>
            <template v-if="appConfig.experimental.clash_api">
              <el-form-item :label="$t('basic.exp.extController')">
                <el-input v-model="appConfig.experimental.clash_api.external_controller" />
              </el-form-item>
              <el-form-item :label="$t('basic.exp.secret')">
                <el-input v-model="appConfig.experimental.clash_api.secret" />
              </el-form-item>
              <el-form-item :label="$t('basic.exp.extUi')">
                <el-input v-model="appConfig.experimental.clash_api.external_ui" />
              </el-form-item>
              <el-form-item :label="$t('basic.exp.extUiDownloadUrl')">
                <el-input v-model="appConfig.experimental.clash_api.external_ui_download_url" />
              </el-form-item>
              <el-form-item :label="$t('basic.exp.extUiDownloadDetour')">
                <el-select v-model="appConfig.experimental.clash_api.external_ui_download_detour" clearable filterable>
                  <el-option v-for="t in outboundTags" :key="t" :label="t" :value="t" />
                </el-select>
              </el-form-item>
              <el-form-item :label="$t('basic.exp.defaultMode')">
                <el-input v-model="appConfig.experimental.clash_api.default_mode" />
              </el-form-item>
              <el-form-item :label="`${$t('basic.exp.allowOrigin')} ${$t('commaSeparated')}`">
                <el-input v-model="origin" />
              </el-form-item>
              <el-form-item :label="$t('basic.exp.allowPrivate')">
                <el-switch v-model="appConfig.experimental.clash_api.access_control_allow_private_network" />
              </el-form-item>
            </template>
          </div>
        </el-form>

        <div class="nc-divider"><span>V2Ray API</span></div>
        <el-form label-position="top">
          <div class="form-grid">
            <el-form-item :label="$t('enable')">
              <el-switch v-model="enableV2rayApi" />
            </el-form-item>
            <template v-if="appConfig.experimental.v2ray_api">
              <el-form-item :label="$t('objects.listen')">
                <el-input v-model="appConfig.experimental.v2ray_api.listen" />
              </el-form-item>
              <el-form-item :label="$t('stats.enable')">
                <el-switch v-model="appConfig.experimental.v2ray_api.stats.enabled" />
              </el-form-item>
            </template>
          </div>
          <template v-if="appConfig.experimental.v2ray_api?.stats?.enabled">
            <el-form-item :label="$t('pages.inbounds')">
              <el-select v-model="appConfig.experimental.v2ray_api.stats.inbounds" multiple collapse-tags>
                <el-option v-for="t in inboundTags" :key="t" :label="t" :value="t" />
              </el-select>
            </el-form-item>
            <el-form-item :label="$t('pages.outbounds')">
              <el-select v-model="appConfig.experimental.v2ray_api.stats.outbounds" multiple collapse-tags>
                <el-option v-for="t in outboundTags" :key="t" :label="t" :value="t" />
              </el-select>
            </el-form-item>
            <el-form-item :label="$t('pages.clients')">
              <el-select v-model="appConfig.experimental.v2ray_api.stats.users" multiple collapse-tags filterable>
                <el-option v-for="c in clientNames" :key="c" :label="c" :value="c" />
              </el-select>
            </el-form-item>
          </template>
        </el-form>
      </el-collapse-item>
    </el-collapse>
  </div>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import { computed, ref, onBeforeMount } from 'vue'
import { Config, Ntp } from '@/types/config'
import { FindDiff } from '@/plugins/utils'
import { Check } from '@element-plus/icons-vue'

const oldConfig = ref({})
const loading = ref(false)
const active = ref(['log'])

const appConfig = computed((): Config => <Config>Data().config)

onBeforeMount(async () => {
  loading.value = true
  while (Data().lastLoad === 0) {
    await new Promise((r) => setTimeout(r, 100))
  }
  // 防御性兜底:确保关键路径存在,避免 v-model 解构 undefined
  const cfg = Data().config as any
  if (!cfg.log) cfg.log = { disabled: false, level: 'info', output: '', timestamp: false }
  if (!cfg.experimental) cfg.experimental = {}
  oldConfig.value = JSON.parse(JSON.stringify(Data().config))
  loading.value = false
})

const stateChange = computed(() => !FindDiff.deepCompare(appConfig.value, oldConfig.value))

const saveConfig = async () => {
  loading.value = true
  const success = await Data().save('config', 'set', appConfig.value)
  if (success) {
    oldConfig.value = JSON.parse(JSON.stringify(Data().config))
  }
  loading.value = false
}

const inboundTags = computed(() => [
  ...(Data().inbounds?.map((i: any) => i.tag) ?? []),
  ...(Data().endpoints?.filter((e: any) => e.listen_port > 0).map((e: any) => e.tag) ?? []),
])
const clientNames = computed(() => (<any[]>Data().clients)?.map((c) => c.name) ?? [])
const outboundTags = computed(() => [
  ...(Data().outbounds?.map((o: any) => o.tag) ?? []),
  ...(Data().endpoints?.map((e: any) => e.tag) ?? []),
])

const levels = ['trace', 'debug', 'info', 'warn', 'error', 'fatal', 'panic']

const enableNtp = computed({
  get: () => appConfig.value.ntp?.enabled ?? false,
  set: (v: boolean) => {
    if (v) appConfig.value.ntp = <Ntp>{ enabled: true, server: 'time.apple.com', server_port: 123, interval: '30m' }
    else appConfig.value.ntp = <Ntp>{}
  },
})

const ntpInterval = computed({
  get: (): any => (appConfig.value.ntp?.interval ? parseInt(appConfig.value.ntp.interval.replace('m', '')) : null),
  set: (v: number) => {
    if (appConfig.value.ntp) {
      if (v > 0) appConfig.value.ntp.interval = v + 'm'
      else delete appConfig.value.ntp.interval
    }
  },
})

const ensureExperimental = () => {
  if (!appConfig.value.experimental) appConfig.value.experimental = {}
  return appConfig.value.experimental
}

const enableCacheFile = computed({
  get: () => appConfig.value.experimental?.cache_file?.enabled ?? false,
  set: (v: boolean) => {
    const e = ensureExperimental()
    if (v) e.cache_file = { enabled: true }
    else delete e.cache_file
  },
})

const enableClashApi = computed({
  get: () => appConfig.value.experimental?.clash_api != undefined,
  set: (v: boolean) => {
    const e = ensureExperimental()
    e.clash_api = v ? { external_controller: '127.0.0.1:9090' } : undefined
  },
})

const enableV2rayApi = computed({
  get: () => appConfig.value.experimental?.v2ray_api != undefined,
  set: (v: boolean) => {
    const e = ensureExperimental()
    e.v2ray_api = v
      ? { listen: '127.0.0.1:8080', stats: { enabled: false, inbounds: [], outbounds: [], users: [] } }
      : undefined
  },
})

const origin = computed({
  get: () =>
    appConfig.value.experimental?.clash_api?.access_control_allow_origin?.length
      ? appConfig.value.experimental.clash_api.access_control_allow_origin.join(',')
      : '',
  set: (v: string) => {
    if (appConfig.value.experimental?.clash_api) {
      appConfig.value.experimental.clash_api.access_control_allow_origin = v.length > 0 ? v.split(',') : undefined
    }
  },
})
</script>

<style scoped>
.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 6px 16px;
}

:deep(.el-collapse) {
  background: #fff;
  border: 1px solid var(--nc-border);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

:deep(.el-collapse-item__header) {
  padding: 0 18px;
  font-size: 13.5px;
  font-weight: 600;
  background: #f8fafc;
  border-bottom: 1px solid var(--nc-border);
  height: 44px;
}

:deep(.el-collapse-item__wrap) {
  border-bottom: 1px solid var(--nc-border-soft);
  padding: 16px 18px;
}

:deep(.el-collapse-item:last-child .el-collapse-item__wrap) {
  border-bottom: none;
}
</style>
