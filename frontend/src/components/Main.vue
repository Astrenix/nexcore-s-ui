<template>
  <div class="dashboard">
    <div class="dashboard__hero nc-card">
      <img src="@/assets/logo.svg" alt="S-UI" class="dashboard__logo" />
      <div>
        <h3 class="dashboard__title">S-UI</h3>
        <p class="dashboard__subtitle">{{ $t('main.welcome', 'Sing-Box panel · 仪表盘') }}</p>
      </div>
    </div>

    <div class="dashboard__actions">
      <el-button @click="logModal.visible = true">
        <el-icon><Document /></el-icon>{{ $t('basic.log.title') }}
      </el-button>
      <el-button @click="backupModal.visible = true">
        <el-icon><DocumentCopy /></el-icon>{{ $t('main.backup.title') }}
      </el-button>
      <el-button @click="usageStatsModal.visible = true">
        <el-icon><DataAnalysis /></el-icon>{{ $t('main.stats.title') }}
      </el-button>
      <el-button v-if="sbdRunning" type="warning" plain :loading="restarting" @click="restartSingbox">
        <el-icon><Refresh /></el-icon>{{ $t('actions.restartSb') }}
      </el-button>
    </div>

    <div class="dashboard__status">
      <div class="stat-card primary">
        <span class="stat-label">{{ $t('main.info.sbd') }}</span>
        <span class="stat-value">
          <span class="status-dot" :class="sbdRunning ? 'online' : 'offline'"></span>
          {{ sbdRunning ? $t('yes') : $t('no') }}
        </span>
        <span class="stat-meta" v-if="tilesData.sbd?.stats?.Uptime">
          {{ $t('main.info.uptime') }}: {{ HumanReadable.formatSecond(tilesData.sbd.stats.Uptime) }}
        </span>
      </div>

      <div class="stat-card info">
        <span class="stat-label">{{ $t('main.info.sys') }}</span>
        <span class="stat-value">{{ tilesData.sys?.cpuCount ?? '—' }} {{ $t('main.info.core') }}</span>
        <span class="stat-meta" v-if="tilesData.sys?.hostName">{{ tilesData.sys.hostName }}</span>
      </div>

      <div class="stat-card success">
        <span class="stat-label">{{ $t('online') }}</span>
        <span class="stat-value">{{ onlineCount }}</span>
        <span class="stat-meta">{{ $t('pages.clients') }}</span>
      </div>

      <div class="stat-card warning">
        <span class="stat-label">{{ $t('version') }}</span>
        <span class="stat-value">{{ tilesData.sys?.appVersion ? `v${tilesData.sys.appVersion}` : '—' }}</span>
      </div>
    </div>

    <div v-if="tilesData.sbd?.stats" class="dashboard__sbd nc-card">
      <h4 class="nc-card-title">{{ $t('main.info.sbd') }}</h4>
      <div class="dashboard__sbd-grid">
        <div class="kv">
          <span class="kv__label">{{ $t('main.info.memory') }}</span>
          <span class="kv__value mono">{{ HumanReadable.sizeFormat(tilesData.sbd.stats.Alloc ?? 0) }}</span>
        </div>
        <div class="kv">
          <span class="kv__label">{{ $t('main.info.threads') }}</span>
          <span class="kv__value mono">{{ tilesData.sbd.stats.NumGoroutine ?? '—' }}</span>
        </div>
        <div class="kv">
          <span class="kv__label">{{ $t('pages.inbounds') }}</span>
          <span class="kv__value mono">{{ Data().onlines.inbound?.length ?? 0 }}</span>
        </div>
        <div class="kv">
          <span class="kv__label">{{ $t('pages.outbounds') }}</span>
          <span class="kv__value mono">{{ Data().onlines.outbound?.length ?? 0 }}</span>
        </div>
      </div>
    </div>

    <Logs v-model="logModal.visible" :control="logModal" :visible="logModal.visible" />
    <Backup v-model="backupModal.visible" :control="backupModal" :visible="backupModal.visible" />
    <UsageStats v-model:visible="usageStatsModal.visible" />
  </div>
</template>

<script lang="ts" setup>
import { computed, defineAsyncComponent, onBeforeUnmount, onMounted, ref } from 'vue'
import HttpUtils from '@/plugins/httputil'
import { HumanReadable } from '@/plugins/utils'
import Data from '@/store/modules/data'
import { Document, DocumentCopy, DataAnalysis, Refresh } from '@element-plus/icons-vue'

const Logs = defineAsyncComponent(() => import('@/layouts/modals/Logs.vue'))
const Backup = defineAsyncComponent(() => import('@/layouts/modals/Backup.vue'))
const UsageStats = defineAsyncComponent(() => import('@/layouts/modals/UsageStats.vue'))

const tilesData = ref<any>({})

const sbdRunning = computed(() => !!tilesData.value?.sbd?.running)
const onlineCount = computed(() => Data().onlines?.user?.length ?? 0)

let intervalId: ReturnType<typeof setInterval> | null = null

const reloadData = async () => {
  const data = await HttpUtils.get('api/status', { r: 'sys,sbd' })
  if (data.success) tilesData.value = data.obj
}

const restarting = ref(false)
const restartSingbox = async () => {
  restarting.value = true
  await HttpUtils.post('api/restartSb', {})
  restarting.value = false
}

onMounted(async () => {
  await reloadData()
  intervalId = setInterval(reloadData, 5000)
})
onBeforeUnmount(() => {
  if (intervalId) clearInterval(intervalId)
})

const logModal = ref({ visible: false })
const backupModal = ref({ visible: false })
const usageStatsModal = ref({ visible: false })
</script>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.dashboard__hero {
  display: flex;
  align-items: center;
  gap: 16px;
}

.dashboard__logo {
  width: 44px;
  height: 44px;
}

.dashboard__title {
  font-size: 18px;
  font-weight: 700;
  color: var(--nc-text-1);
  margin: 0;
}

.dashboard__subtitle {
  margin: 4px 0 0;
  font-size: 13px;
  color: var(--nc-text-muted);
}

.dashboard__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.dashboard__status {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
}

.dashboard__sbd-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 14px;
}

.kv {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.kv__label {
  font-size: 11.5px;
  color: var(--nc-text-muted);
  font-weight: 500;
}

.kv__value {
  font-size: 16px;
  font-weight: 600;
  color: var(--nc-text-1);
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}
</style>
