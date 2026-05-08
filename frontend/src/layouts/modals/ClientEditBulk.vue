<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="closeModal"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="$t('actions.editbulk')"
    destroy-on-close
  >
    <div class="nc-card section-card">
      <h4 class="section-card__title">{{ $t('actions.action') }}</h4>
      <el-form label-position="top">
        <el-form-item :label="$t('actions.action')" class="action-select-item">
          <el-select v-model="actionMode" @change="onActionChange">
            <el-option v-for="m in actionModes" :key="m.value" :label="m.title" :value="m.value" />
          </el-select>
        </el-form-item>

        <div v-if="actionMode === 'change_limits'" class="form-grid">
          <el-form-item :label="$t('bulk.addDays')">
            <el-input-number v-model="editData.addDays" controls-position="right" style="width: 100%" />
          </el-form-item>
          <el-form-item :label="$t('bulk.addVolume')">
            <el-input-number v-model="editData.addVolume" controls-position="right" style="width: 100%" />
          </el-form-item>
          <el-form-item :label="$t('enable')">
            <el-switch v-model="editData.enable" />
          </el-form-item>
        </div>

        <el-form-item
          v-if="actionMode === 'add_inbounds' || actionMode === 'remove_inbounds'"
          :label="$t('client.inboundTags')"
        >
          <el-select v-model="editData.inboundTags" multiple collapse-tags collapse-tags-tooltip>
            <el-option v-for="t in inboundTags" :key="t.value" :label="t.title" :value="t.value" />
          </el-select>
        </el-form-item>
      </el-form>
    </div>

    <Users :clients="clients" :data="selectedClients" />

    <template #footer>
      <el-button @click="closeModal">{{ $t('actions.close') }}</el-button>
      <el-button
        type="primary"
        :loading="loading"
        :disabled="selectedClients.values.length === 0 && selectedClients.model !== 'all'"
        @click="saveChanges"
      >{{ $t('actions.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { reactive, ref, watch } from 'vue'
import Users from '@/components/Users.vue'
import { i18n } from '@/locales'
import Data from '@/store/modules/data'
import { Client } from '@/types/clients'

const props = defineProps<{ visible: boolean; clients: any[]; inboundTags: any[] }>()
const emit = defineEmits<{ close: []; 'update:modelValue': [v: boolean] }>()

const loading = ref(false)
const actionMode = ref('change_limits')

const actionModes = [
  { title: i18n.global.t('bulk.changeLimits'),  value: 'change_limits' },
  { title: i18n.global.t('bulk.addInbounds'),   value: 'add_inbounds' },
  { title: i18n.global.t('bulk.removeInbounds'), value: 'remove_inbounds' },
  { title: i18n.global.t('actions.delbulk'),     value: 'delete_bulk' },
]

const editData = reactive({
  enable: true,
  addDays: 0,
  addVolume: 0,
  inboundTags: <number[]>[],
})

const selectedClients = reactive({ model: 'none', values: <any[]>[] })

const onActionChange = () => { editData.inboundTags = [] }
const closeModal = () => emit('close')

const getTargetClients = (): Client[] => {
  const list = props.clients ?? []
  switch (selectedClients.model) {
    case 'all':
      return list
    case 'group':
      return list.filter((c: any) => selectedClients.values.includes(c.group))
    case 'client':
      return list.filter((c: any) => selectedClients.values.includes(c.id))
    default:
      return []
  }
}

const saveChanges = async () => {
  loading.value = true
  const targets = getTargetClients()
  switch (actionMode.value) {
    case 'change_limits':
      targets.forEach((c: Client) => {
        if (editData.addVolume !== 0 && c.volume > 0) c.volume += editData.addVolume * 1024 ** 3
        if (editData.addDays !== 0 && c.expiry > 0) c.expiry += editData.addDays * 24 * 60 * 60
        if (editData.enable)
          c.enable = (c.volume === 0 || c.up + c.down < c.volume) && (c.expiry === 0 || c.expiry > Date.now() / 1000)
      })
      break
    case 'add_inbounds':
      targets.forEach((c: Client) => {
        editData.inboundTags.forEach((t: number) => {
          if (!c.inbounds.includes(t)) c.inbounds.push(t)
        })
        c.inbounds = [...c.inbounds].sort()
      })
      break
    case 'remove_inbounds':
      targets.forEach((c: Client) => {
        c.inbounds = c.inbounds.filter((i: number) => !editData.inboundTags.includes(i))
      })
      break
    case 'delete_bulk': {
      const success = await Data().save('clients', 'delbulk', targets.map((c: Client) => c.id))
      if (success) closeModal()
      loading.value = false
      return
    }
  }
  const success = await Data().save('clients', 'editbulk', targets)
  if (success) closeModal()
  loading.value = false
}

watch(() => props.visible, (v) => {
  if (v) {
    actionMode.value = 'change_limits'
    Object.assign(editData, { enable: true, addDays: 0, addVolume: 0, inboundTags: [] })
    Object.assign(selectedClients, { model: 'none', values: [] })
  }
})
</script>

<style scoped>
.section-card {
  margin-bottom: 16px;
  padding: 14px 18px;
}

.section-card__title {
  font-size: 11.5px;
  font-weight: 600;
  color: var(--nc-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  margin-bottom: 12px;
}

.action-select-item {
  max-width: 280px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 6px 16px;
}
</style>
