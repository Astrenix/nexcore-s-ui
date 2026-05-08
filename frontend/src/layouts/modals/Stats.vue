<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="$emit('close')"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="$t('stats.graphTitle')"
    destroy-on-close
  >
    <div class="stats-target">{{ $t('objects.' + resource) }} : {{ tag }}</div>

    <el-radio-group v-model="limit" size="small" class="stats-period" @change="loadData">
      <el-radio-button v-for="p in periods" :key="p.value" :label="p.value">{{ p.title }}</el-radio-button>
    </el-radio-group>

    <div class="stats-chart">
      <div v-if="loading" class="stats-loading">
        <el-icon class="is-loading"><Loading /></el-icon>
      </div>
      <el-empty v-else-if="alert" :description="$t('noData')" :image-size="80" />
      <Line v-else-if="loaded" :data="usage as any" :options="options as any" />
    </div>
  </el-dialog>
</template>

<script lang="ts" setup>
import { onBeforeUnmount, ref, watch } from 'vue'
import HttpUtils from '@/plugins/httputil'
import { HumanReadable } from '@/plugins/utils'
import { i18n } from '@/locales'
import { Loading } from '@element-plus/icons-vue'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js'
import { Line } from 'vue-chartjs'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Legend, Filler)
ChartJS.defaults.font.family = 'system-ui, -apple-system, sans-serif'
ChartJS.defaults.color = '#64748b'

const props = defineProps<{ visible: boolean; resource: string; tag: string }>()
defineEmits<{ close: []; 'update:modelValue': [v: boolean] }>()

const loading = ref(false)
const loaded = ref(false)
const alert = ref(false)
const limit = ref(1)

const periods = [
  { value: 1, title: '1H' },
  { value: 6, title: '6H' },
  { value: 12, title: '12H' },
  { value: 24, title: '1D' },
  { value: 48, title: '2D' },
  { value: 240, title: '10D' },
  { value: 720, title: '30D' },
  { value: 1440, title: '60D' },
]

const usage = ref<any>({})
let intervalId: ReturnType<typeof setInterval> | null = null

const options = {
  responsive: true,
  maintainAspectRatio: false,
  interaction: { intersect: false, mode: 'index' as const },
  elements: { point: { pointStyle: 'crossRot' } },
  plugins: {
    legend: { labels: { boxWidth: 10, usePointStyle: true, font: { size: 11 } } },
    tooltip: {
      callbacks: {
        footer: (items: any[]) => HumanReadable.sizeFormat(items.reduce((acc, c) => acc + c.raw, 0)),
      },
    },
  },
  scales: {
    y: {
      grid: { color: '#e2e8f0' },
      beginAtZero: true,
      ticks: {
        callback: (label: any) => (label == 0 ? 0 : HumanReadable.sizeFormat(label, 0)),
        count: 6,
        font: { size: 10 },
      },
    },
    x: {
      grid: { display: false },
      ticks: { font: { size: 10 } },
    },
  },
}

const loadData = async () => {
  loading.value = true
  const data = await HttpUtils.get('api/stats', {
    resource: props.resource,
    tag: props.tag,
    limit: limit.value,
  })
  if (data.success && data.obj) {
    const obj = <any[]>data.obj
    const l = String(i18n.global.locale.value) === 'fa' ? 'fa-IR' : 'en-US'
    const oneStep = (limit.value * 3600 * 1000) / 360
    const now = new Date().getTime()
    const steps: number[] = []
    for (let i = 360; i >= 0; i--) steps.push(now - oneStep * i)
    const labels: string[] = []
    const uplink: number[] = []
    const downlink: number[] = []
    for (let i = 1; i < 360; i++) {
      labels.push(genLabel(steps[i], l))
      const upTraffics = obj
        .filter((o) => o.direction && o.dateTime * 1000 < steps[i] && o.dateTime * 1000 > steps[i - 1])
        .map((o: any) => o.traffic)
      const upSum = upTraffics.length > 0 ? upTraffics.reduce((u: number) => u) : (null as any)
      const downTraffics = obj
        .filter((o) => !o.direction && o.dateTime * 1000 < steps[i] && o.dateTime * 1000 > steps[i - 1])
        .map((o: any) => o.traffic)
      const downSum = downTraffics.length > 0 ? downTraffics.reduce((d: number) => d) : (null as any)
      uplink.push(upSum)
      downlink.push(downSum)
    }
    usage.value = {
      labels,
      datasets: [
        {
          label: i18n.global.t('stats.upload'),
          backgroundColor: 'rgba(59, 130, 246, 0.18)',
          borderColor: '#3b82f6',
          borderWidth: 1.5,
          fill: true,
          tension: 0.25,
          data: uplink,
        },
        {
          label: i18n.global.t('stats.download'),
          backgroundColor: 'rgba(100, 116, 139, 0.16)',
          borderColor: '#64748b',
          borderWidth: 1.5,
          fill: true,
          tension: 0.25,
          data: downlink,
        },
      ],
    }
    loaded.value = true
    alert.value = false
  } else {
    alert.value = true
    loaded.value = false
  }
  loading.value = false
}

const genLabel = (step: number, l: string) =>
  new Date(step).toLocaleString(l, {
    month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false,
  })

watch(() => props.visible, (v) => {
  if (v) {
    limit.value = 1
    loadData()
    intervalId = setInterval(loadData, 10000)
  } else {
    loaded.value = false
    alert.value = false
    if (intervalId) {
      clearInterval(intervalId)
      intervalId = null
    }
  }
})

onBeforeUnmount(() => {
  if (intervalId) clearInterval(intervalId)
})
</script>

<style scoped>
.stats-target {
  text-align: center;
  font-size: 13px;
  color: var(--nc-text-muted);
  margin-bottom: 12px;
  font-family: var(--font-mono);
}

.stats-period {
  display: flex;
  justify-content: center;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.stats-chart {
  height: 280px;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
}

.stats-loading {
  font-size: 28px;
  color: var(--nc-primary);
}
</style>
