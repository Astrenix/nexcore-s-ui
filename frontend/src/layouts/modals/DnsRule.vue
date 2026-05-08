<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="closeModal"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="$t('actions.' + title) + ' ' + $t('objects.dnsrule')"
    destroy-on-close
  >
    <el-form label-position="top">
      <div class="form-grid">
        <el-form-item :label="$t('rule.mode')">
          <el-select v-model="ruleMode" @change="onModeChange">
            <el-option :label="$t('rule.simple')" value="simple" />
            <el-option :label="$t('rule.logical')" value="logical" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('admin.action')">
          <el-input v-model="rule.action" />
        </el-form-item>
        <el-form-item :label="$t('dns.server')">
          <el-select v-model="rule.server" clearable filterable>
            <el-option v-for="t in serverTags" :key="t" :label="t" :value="t" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('rule.invert')">
          <el-switch v-model="rule.invert" />
        </el-form-item>
      </div>
      <JsonEditorBlock :data="rule" @update:data="(v) => (rule = v)" />
    </el-form>

    <template #footer>
      <el-button @click="closeModal">{{ $t('actions.close') }}</el-button>
      <el-button type="primary" @click="saveChanges">{{ $t('actions.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, watch, computed } from 'vue'
import JsonEditorBlock from '@/components/JsonEditorBlock.vue'

const props = defineProps<{
  visible: boolean
  index: number
  data: string
  clients: string[]
  inTags: string[]
  serverTags: string[]
  ruleSets: string[]
}>()
const emit = defineEmits<{ close: []; save: [data: any]; 'update:modelValue': [v: boolean] }>()
void props.clients; void props.inTags; void props.ruleSets

const rule = ref<any>({ action: 'route', server: '' })
const title = ref<'add' | 'edit'>('add')

const ruleMode = computed({
  get: () => (rule.value.type === 'logical' ? 'logical' : 'simple'),
  set: (v: string) => {
    if (v === 'logical') {
      rule.value = { type: 'logical', mode: 'and', rules: [], action: rule.value.action ?? 'route', server: rule.value.server ?? '' }
    } else {
      rule.value = { action: rule.value.action ?? 'route', server: rule.value.server ?? '' }
    }
  },
})

const onModeChange = () => { /* mode change handled by computed */ }

const updateData = () => {
  if (props.index >= 0) {
    rule.value = JSON.parse(props.data || '{}')
    title.value = 'edit'
  } else {
    rule.value = { action: 'route', server: '' }
    title.value = 'add'
  }
}

const closeModal = () => emit('close')
const saveChanges = () => emit('save', rule.value)

watch(() => props.visible, (v) => { if (v) updateData() })
</script>

<style scoped>
.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 6px 16px;
  margin-bottom: 12px;
}
</style>
