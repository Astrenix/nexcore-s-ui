<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="$emit('close')"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="title"
    destroy-on-close
  >
    <el-input
      v-model="content"
      type="textarea"
      :rows="18"
      spellcheck="false"
      class="code-textarea mono"
    />

    <template #footer>
      <el-button @click="$emit('close')">{{ $t('actions.close') }}</el-button>
      <el-button type="primary" @click="$emit('save', content)">{{ $t('actions.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, watch } from 'vue'

const props = defineProps<{ visible: boolean; data: string; title: string }>()
defineEmits<{ close: []; save: [data: string]; 'update:modelValue': [v: boolean] }>()

const content = ref('')

watch(() => props.visible, (v) => {
  if (v) content.value = props.data ?? ''
})

watch(() => props.data, (d) => {
  if (props.visible) content.value = d ?? ''
})
</script>

<style scoped>
.code-textarea :deep(.el-textarea__inner) {
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.6;
  background: #f8fafc;
  border-color: var(--nc-border);
  min-height: 280px;
}
</style>
