<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:visible', $event)"
    @close="$emit('update:visible', false)"
    class="constrained-dialog"
    :align-center="false"
    :title="$t('main.stats.title')"
    destroy-on-close
  >
    <template #header>
      <div class="usage-header">
        <span class="el-dialog__title">{{ $t('main.stats.title') }}</span>
        <el-tooltip :content="$t('actions.update')" placement="top">
          <el-button text :loading="loading" @click="refresh">
            <el-icon><Refresh /></el-icon>
          </el-button>
        </el-tooltip>
      </div>
    </template>

    <el-descriptions :column="1" size="small" border>
      <el-descriptions-item v-for="row in tableRows" :key="row.key">
        <template #label>
          <span class="usage-label">
            <el-icon class="usage-icon" :style="{ color: row.color }"><component :is="row.icon" /></el-icon>
            {{ row.label }}
          </span>
        </template>
        <span class="usage-value mono">{{ row.value }}</span>
      </el-descriptions-item>
    </el-descriptions>
  </el-dialog>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue'
import HttpUtils from '@/plugins/httputil'
import { HumanReadable } from '@/plugins/utils'
import { i18n } from '@/locales'
import { markRaw } from 'vue'
import {
  Refresh, User, Download, Upload, Operation, PriceTag, DataAnalysis, MagicStick,
} from '@element-plus/icons-vue'

const props = defineProps<{ visible: boolean }>()
defineEmits<{ 'update:visible': [v: boolean] }>()

const loading = ref(false)
const info = ref<{
  clients?: number; inbounds?: number; outbounds?: number; services?: number; endpoints?: number
  clientUp?: number; clientDown?: number
}>({})

const clientUp = computed(() => HumanReadable.sizeFormat(info.value.clientUp ?? 0))
const clientDown = computed(() => HumanReadable.sizeFormat(info.value.clientDown ?? 0))
const totalUsage = computed(() => HumanReadable.sizeFormat((info.value.clientUp ?? 0) + (info.value.clientDown ?? 0)))

const tableRows = computed(() => {
  const t = (key: string) => i18n.global.t(key)
  return [
    { key: 'clients',   icon: markRaw(User),         label: t('pages.clients'),   value: info.value.clients ?? 0,   color: 'var(--nc-text-muted)' },
    { key: 'inbounds',  icon: markRaw(Download),     label: t('pages.inbounds'),  value: info.value.inbounds ?? 0,  color: 'var(--nc-text-muted)' },
    { key: 'outbounds', icon: markRaw(Upload),       label: t('pages.outbounds'), value: info.value.outbounds ?? 0, color: 'var(--nc-text-muted)' },
    { key: 'services',  icon: markRaw(Operation),    label: t('pages.services'),  value: info.value.services ?? 0,  color: 'var(--nc-text-muted)' },
    { key: 'endpoints', icon: markRaw(PriceTag),     label: t('pages.endpoints'), value: info.value.endpoints ?? 0, color: 'var(--nc-text-muted)' },
    { key: 'clientUp',  icon: markRaw(Upload),       label: t('stats.upload'),    value: clientUp.value,            color: 'var(--nc-warning)' },
    { key: 'clientDown',icon: markRaw(Download),     label: t('stats.download'),  value: clientDown.value,          color: 'var(--nc-success)' },
    { key: 'total',     icon: markRaw(DataAnalysis), label: t('main.stats.totalUsage'), value: totalUsage.value,    color: 'var(--nc-primary)' },
  ]
})

const refresh = async () => {
  loading.value = true
  const data = await HttpUtils.get('api/status', { r: 'db' })
  if (data.success && data.obj) info.value = data.obj.db ?? data.obj
  loading.value = false
}

watch(() => props.visible, (v) => {
  if (v) refresh()
})

void MagicStick
</script>

<style scoped>
.usage-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.usage-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--nc-text-3);
}

.usage-icon {
  font-size: 14px;
}

.usage-value {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
  font-size: 13px;
  font-weight: 600;
  color: var(--nc-text-1);
}

:deep(.el-descriptions__cell) {
  padding: 10px 14px !important;
}
</style>
