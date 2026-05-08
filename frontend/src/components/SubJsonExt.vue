<template>
  <div class="sub-ext-wrap">
    <p class="sub-ext-hint">
      JSON 订阅扩展配置。完整字段(log / dns / route 规则等)请通过下方 JSON 编辑器调整。
    </p>

    <el-button type="primary" @click="enableEditor = true">
      <el-icon><Edit /></el-icon>{{ $t('editor') }} - {{ $t('setting.jsonSub') }}
    </el-button>

    <el-input
      v-model="settings.subJsonExt"
      type="textarea"
      :rows="14"
      spellcheck="false"
      class="json-textarea mono"
      style="margin-top: 12px"
    />

    <Editor
      v-model="enableEditor"
      :data="settings.subJsonExt"
      :visible="enableEditor"
      :title="$t('editor') + ' - ' + $t('setting.jsonSub')"
      @close="enableEditor = false"
      @save="saveEditor"
    />
  </div>
</template>

<script lang="ts" setup>
import { ref } from 'vue'
import Editor from './Editor.vue'
import { Edit } from '@element-plus/icons-vue'

const props = defineProps<{ settings: any }>()

const enableEditor = ref(false)

const saveEditor = (data: string) => {
  props.settings.subJsonExt = data
  enableEditor.value = false
}
</script>

<style scoped>
.sub-ext-wrap {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.sub-ext-hint {
  font-size: 12px;
  color: var(--nc-text-muted);
  margin: 0 0 8px;
}

.json-textarea :deep(.el-textarea__inner) {
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.6;
  background: #f8fafc;
  border-color: var(--nc-border);
}
</style>
