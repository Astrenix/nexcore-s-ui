<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="closeModal"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="$t('actions.' + title) + ' ' + $t('objects.ruleset')"
    destroy-on-close
  >
    <el-form label-position="top">
      <div class="form-grid">
        <el-form-item :label="$t('type')">
          <el-select v-model="ruleset.type">
            <el-option label="local" value="local" />
            <el-option label="remote" value="remote" />
            <el-option label="inline" value="inline" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('objects.tag')">
          <el-input v-model="ruleset.tag" />
        </el-form-item>
        <el-form-item :label="$t('ruleset.format')">
          <el-select v-model="ruleset.format">
            <el-option label="binary" value="binary" />
            <el-option label="source" value="source" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="ruleset.type === 'remote'" :label="$t('actions.update')">
          <el-input v-model="ruleset.update_interval" placeholder="1d" />
        </el-form-item>
        <el-form-item v-if="ruleset.type === 'remote'" :label="$t('objects.outbound')">
          <el-select v-model="ruleset.download_detour" clearable filterable>
            <el-option v-for="t in outTags" :key="t" :label="t" :value="t" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="ruleset.type === 'remote'" label="URL">
          <el-input v-model="ruleset.url" />
        </el-form-item>
        <el-form-item v-if="ruleset.type === 'local'" :label="$t('transport.path')">
          <el-input v-model="ruleset.path" />
        </el-form-item>
      </div>
      <JsonEditorBlock :data="ruleset" @update:data="(v) => (ruleset = v)" />
    </el-form>

    <template #footer>
      <el-button @click="closeModal">{{ $t('actions.close') }}</el-button>
      <el-button type="primary" @click="saveChanges">{{ $t('actions.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, watch } from 'vue'
import JsonEditorBlock from '@/components/JsonEditorBlock.vue'
import RandomUtil from '@/plugins/randomUtil'

const props = defineProps<{ visible: boolean; index: number; data: string; outTags: string[] }>()
const emit = defineEmits<{ close: []; save: [data: any]; 'update:modelValue': [v: boolean] }>()
void props.outTags

const ruleset = ref<any>({ type: 'remote', tag: '', format: 'binary', url: '', update_interval: '1d' })
const title = ref<'add' | 'edit'>('add')

const updateData = () => {
  if (props.index >= 0) {
    ruleset.value = JSON.parse(props.data || '{}')
    title.value = 'edit'
  } else {
    ruleset.value = { type: 'remote', tag: 'rs-' + RandomUtil.randomSeq(3), format: 'binary', url: '', update_interval: '1d' }
    title.value = 'add'
  }
}

const closeModal = () => emit('close')
const saveChanges = () => emit('save', ruleset.value)

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
