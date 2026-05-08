<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="closeModal"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="$t('actions.' + title) + ' ' + $t('objects.endpoint')"
    destroy-on-close
  >
    <el-form label-position="top">
      <div class="form-grid">
        <el-form-item :label="$t('type')">
          <el-select v-model="endpoint.type" filterable @change="changeType">
            <el-option v-for="(v, k) in EpTypes" :key="k" :label="k" :value="v" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('objects.tag')">
          <el-input v-model="endpoint.tag" />
        </el-form-item>
      </div>
      <JsonEditorBlock :data="endpoint" @update:data="(v) => (endpoint = v)" />
    </el-form>

    <template #footer>
      <el-button @click="closeModal">{{ $t('actions.close') }}</el-button>
      <el-button type="primary" :loading="loading" @click="saveChanges">{{ $t('actions.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, watch } from 'vue'
import { Endpoint, EpTypes, createEndpoint } from '@/types/endpoints'
import RandomUtil from '@/plugins/randomUtil'
import Data from '@/store/modules/data'
import JsonEditorBlock from '@/components/JsonEditorBlock.vue'

const props = defineProps<{ visible: boolean; id: number; data: string; tags: string[] }>()
const emit = defineEmits<{ close: []; 'update:modelValue': [v: boolean] }>()
void props.tags

const endpoint = ref<Endpoint>(createEndpoint('wireguard', { tag: '' } as any))
const title = ref<'add' | 'edit'>('add')
const loading = ref(false)

const updateData = (id: number) => {
  if (id > 0) {
    endpoint.value = JSON.parse(props.data)
    title.value = 'edit'
  } else {
    endpoint.value = createEndpoint('wireguard', { tag: 'wg-' + RandomUtil.randomSeq(3) } as any)
    title.value = 'add'
  }
}

const changeType = () => {
  const tag = props.id > 0 ? endpoint.value.tag : endpoint.value.type + '-' + RandomUtil.randomSeq(3)
  endpoint.value = createEndpoint(endpoint.value.type, { tag } as any)
}

const closeModal = () => emit('close')

const saveChanges = async () => {
  if (!props.visible) return
  if (Data().checkTag('endpoint', props.id, endpoint.value.tag)) return
  loading.value = true
  const success = await Data().save('endpoints', props.id == 0 ? 'new' : 'edit', endpoint.value)
  if (success) closeModal()
  loading.value = false
}

watch(() => props.visible, (v) => { if (v) updateData(props.id) })
</script>

<style scoped>
.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 6px 16px;
  margin-bottom: 12px;
}
</style>
