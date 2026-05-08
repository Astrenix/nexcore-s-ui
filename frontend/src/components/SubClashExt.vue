<template>
  <div class="sub-ext-wrap">
    <p class="sub-ext-hint">
      Clash 订阅扩展(YAML)。完整规则配置请通过下方编辑器调整。
    </p>

    <el-button type="primary" @click="enableEditor = true">
      <el-icon><Edit /></el-icon>{{ $t('editor') }} - {{ $t('setting.clashSub') }}
    </el-button>

    <el-input
      v-model="settings.subClashExt"
      type="textarea"
      :rows="14"
      spellcheck="false"
      class="yaml-textarea mono"
      style="margin-top: 12px"
    />

    <Editor
      v-model="enableEditor"
      :data="settings.subClashExt"
      :visible="enableEditor"
      :title="$t('editor') + ' - ' + $t('setting.clashSub')"
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
  props.settings.subClashExt = data
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

.yaml-textarea :deep(.el-textarea__inner) {
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.6;
  background: #f8fafc;
  border-color: var(--nc-border);
}
</style>
