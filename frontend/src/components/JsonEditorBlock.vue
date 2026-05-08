<template>
  <div class="advanced-section">
    <div class="advanced-section__head">
      <span class="advanced-section__title">{{ title || `配置 (JSON)` }}</span>
      <el-tooltip :content="$t('actions.update')" placement="top">
        <el-button text @click="syncFromObject"><el-icon><RefreshRight /></el-icon></el-button>
      </el-tooltip>
    </div>
    <p v-if="hint" class="advanced-section__hint">{{ hint }}</p>
    <p v-else class="advanced-section__hint">完整字段编辑界面在阶段 4 重写;当前可通过 JSON 直接调整。</p>
    <el-input
      v-model="json"
      type="textarea"
      :rows="rows"
      spellcheck="false"
      class="json-editor mono"
      @change="onJsonEdit"
    />
    <p v-if="error" class="json-error">{{ error }}</p>
  </div>
</template>

<script lang="ts" setup>
import { ref, watch } from 'vue'
import { RefreshRight } from '@element-plus/icons-vue'

const props = defineProps<{ data: any; rows?: number; title?: string; hint?: string }>()
const emit = defineEmits<{ 'update:data': [v: any] }>()

const json = ref(JSON.stringify(props.data, null, 2))
const error = ref('')

const syncFromObject = () => {
  json.value = JSON.stringify(props.data, null, 2)
  error.value = ''
}

const onJsonEdit = () => {
  try {
    const parsed = JSON.parse(json.value)
    if (typeof parsed === 'object' && parsed !== null) {
      emit('update:data', parsed)
      error.value = ''
    }
  } catch (e: any) {
    error.value = `JSON: ${e.message}`
  }
}

watch(() => props.data, (v) => {
  if (JSON.stringify(v, null, 2) !== json.value) {
    json.value = JSON.stringify(v, null, 2)
  }
}, { deep: true })
</script>

<style scoped>
.advanced-section {
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px dashed var(--nc-border);
}

.advanced-section__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.advanced-section__title {
  font-size: 11.5px;
  font-weight: 600;
  color: var(--nc-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.advanced-section__hint {
  font-size: 11.5px;
  color: var(--nc-text-faint);
  margin: 4px 0 8px;
}

.json-editor :deep(.el-textarea__inner) {
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.6;
  background: #f8fafc;
  border-color: var(--nc-border);
}

.json-error {
  margin-top: 6px;
  font-size: 11.5px;
  color: var(--nc-danger);
  font-family: var(--font-mono);
}
</style>
