<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="closeModal"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="$t('actions.' + title) + ' ' + $t('objects.tls')"
    destroy-on-close
  >
    <el-form label-position="top">
      <div class="form-grid">
        <el-form-item :label="$t('client.name')">
          <el-input v-model="config.name" />
        </el-form-item>
        <el-form-item label="ACME">
          <el-switch v-model="hasAcme" />
        </el-form-item>
        <el-form-item label="Reality">
          <el-switch v-model="hasReality" />
        </el-form-item>
        <el-form-item label="ECH">
          <el-switch v-model="hasEch" />
        </el-form-item>
      </div>
      <JsonEditorBlock :data="config" :rows="20" @update:data="(v) => (config = v)" />
    </el-form>

    <template #footer>
      <el-button @click="closeModal">{{ $t('actions.close') }}</el-button>
      <el-button type="primary" :loading="loading" @click="saveChanges">{{ $t('actions.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, watch, computed } from 'vue'
import type { tls } from '@/types/tls'
import JsonEditorBlock from '@/components/JsonEditorBlock.vue'

const props = defineProps<{ visible: boolean; id: number; data: string }>()
const emit = defineEmits<{ close: []; save: [data: tls]; 'update:modelValue': [v: boolean] }>()

const config = ref<any>({ name: '', server: { server_name: '' }, client: {} })
const title = ref<'add' | 'edit'>('add')
const loading = ref(false)

const hasAcme = computed({
  get: () => !!config.value.server?.acme,
  set: (v: boolean) => {
    if (!config.value.server) config.value.server = {}
    if (v) config.value.server.acme = { domain: [], email: '' }
    else delete config.value.server.acme
  },
})
const hasReality = computed({
  get: () => !!config.value.server?.reality,
  set: (v: boolean) => {
    if (!config.value.server) config.value.server = {}
    if (v) config.value.server.reality = { handshake: { server: '', server_port: 443 }, private_key: '', short_id: [] }
    else delete config.value.server.reality
  },
})
const hasEch = computed({
  get: () => !!config.value.server?.ech,
  set: (v: boolean) => {
    if (!config.value.server) config.value.server = {}
    if (v) config.value.server.ech = {}
    else delete config.value.server.ech
  },
})

const updateData = (id: number) => {
  if (id > 0) {
    config.value = JSON.parse(props.data || '{}')
    title.value = 'edit'
  } else {
    config.value = { name: 'tls-' + Math.random().toString(36).slice(2, 6), server: { server_name: '' } }
    title.value = 'add'
  }
}

const closeModal = () => emit('close')

const saveChanges = async () => {
  loading.value = true
  emit('save', config.value as tls)
  loading.value = false
}

watch(() => props.visible, (v) => { if (v) updateData(props.id) })
</script>

<style scoped>
.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 6px 16px;
  margin-bottom: 12px;
}
</style>
