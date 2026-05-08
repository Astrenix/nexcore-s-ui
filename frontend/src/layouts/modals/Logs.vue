<template>
  <el-dialog
    :model-value="control.visible"
    @update:model-value="(v) => (control.visible = v)"
    @close="control.visible = false"
    class="constrained-dialog is-wide"
    :align-center="false"
    :title="$t('basic.log.title')"
    destroy-on-close
  >
    <div class="logs-toolbar">
      <el-form-item :label="$t('basic.log.level')" class="logs-toolbar__item">
        <el-select v-model="logLevel" @change="loadData">
          <el-option v-for="l in logLevels" :key="l.value" :label="l.title" :value="l.value" />
        </el-select>
      </el-form-item>
      <el-form-item :label="$t('count')" class="logs-toolbar__item">
        <el-select v-model.number="logCount" @change="loadData">
          <el-option v-for="n in [10, 20, 30, 50, 100]" :key="n" :label="n" :value="n" />
        </el-select>
      </el-form-item>
      <el-button :loading="loading" @click="loadData">
        <el-icon><Refresh /></el-icon>{{ $t('actions.update') }}
      </el-button>
    </div>

    <div class="logs-output mono" dir="ltr">
      <div v-for="(line, i) in lines" :key="i" v-html="line"></div>
      <div v-if="lines.length === 0 && !loading" class="logs-empty">{{ $t('noData') }}</div>
    </div>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, watch } from 'vue'
import HttpUtils from '@/plugins/httputil'
import { Refresh } from '@element-plus/icons-vue'

const props = defineProps<{ control: any; visible: boolean }>()

const loading = ref(false)
const lines = ref<string[]>([])
const logLevel = ref('info')
const logCount = ref(10)

const logLevels = [
  { title: 'DEBUG', value: 'debug' },
  { title: 'INFO', value: 'info' },
  { title: 'WARNING', value: 'warning' },
  { title: 'ERROR', value: 'err' },
]

const loadData = async () => {
  loading.value = true
  const data = await HttpUtils.get('api/logs', { c: logCount.value, l: logLevel.value })
  if (data.success) lines.value = data.obj ?? []
  loading.value = false
}

watch(() => props.visible, (v) => {
  lines.value = []
  logLevel.value = 'info'
  logCount.value = 10
  if (v) loadData()
})
</script>

<style scoped>
.logs-toolbar {
  display: flex;
  align-items: flex-end;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.logs-toolbar__item {
  flex: 0 0 180px;
  margin: 0 !important;
}

.logs-output {
  background: #0f172a;
  color: #e2e8f0;
  border-radius: var(--radius-md);
  padding: 12px 16px;
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.6;
  max-height: 50vh;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
}

.logs-empty {
  color: var(--nc-text-faint);
  text-align: center;
  padding: 24px 0;
  font-family: var(--font-body);
}
</style>
