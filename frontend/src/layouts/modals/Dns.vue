<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="closeModal"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="$t('actions.' + title) + ' ' + $t('objects.dnsserver')"
    destroy-on-close
  >
    <el-form label-position="top">
      <div class="form-grid">
        <el-form-item :label="$t('type')">
          <el-select v-model="server.type" filterable @change="changeType">
            <el-option v-for="(v, k) in DnsTypes" :key="k" :label="k" :value="v" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('objects.tag')">
          <el-input v-model="server.tag" />
        </el-form-item>
      </div>
      <JsonEditorBlock :data="server" @update:data="(v) => (server = v)" />
    </el-form>

    <template #footer>
      <el-button @click="closeModal">{{ $t('actions.close') }}</el-button>
      <el-button type="primary" @click="saveChanges">{{ $t('actions.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, watch } from 'vue'
import { DnsTypes, createDnsServer } from '@/types/dns'
import RandomUtil from '@/plugins/randomUtil'
import JsonEditorBlock from '@/components/JsonEditorBlock.vue'

const props = defineProps<{
  visible: boolean
  index: number
  data: string
  tsTags: string[]
  rslvdTags: string[]
}>()
const emit = defineEmits<{ close: []; save: [data: any]; 'update:modelValue': [v: boolean] }>()
void props.tsTags; void props.rslvdTags

const server = ref<any>({ type: 'tcp', tag: '' })
const title = ref<'add' | 'edit'>('add')

const updateData = () => {
  if (props.index >= 0) {
    server.value = JSON.parse(props.data || '{}')
    title.value = 'edit'
  } else {
    server.value = createDnsServer('tcp', { tag: 'dns-' + RandomUtil.randomSeq(3) } as any)
    title.value = 'add'
  }
}

const changeType = () => {
  const tag = props.index >= 0 ? server.value.tag : server.value.type + '-' + RandomUtil.randomSeq(3)
  server.value = createDnsServer(server.value.type, { tag } as any)
}

const closeModal = () => emit('close')
const saveChanges = () => emit('save', server.value)

watch(() => props.visible, (v) => { if (v) updateData() })
</script>

<style scoped>
.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 6px 16px;
  margin-bottom: 12px;
}
</style>
