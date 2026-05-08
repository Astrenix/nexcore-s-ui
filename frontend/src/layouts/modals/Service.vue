<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="closeModal"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="$t('actions.' + title) + ' ' + $t('objects.service')"
    destroy-on-close
  >
    <el-form label-position="top">
      <div class="form-grid">
        <el-form-item :label="$t('type')">
          <el-select v-model="srv.type" filterable @change="changeType">
            <el-option v-for="(v, k) in SrvTypes" :key="k" :label="k" :value="v" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('objects.tag')">
          <el-input v-model="srv.tag" />
        </el-form-item>
        <el-form-item :label="$t('in.addr')">
          <el-input v-model="(srv as any).listen" placeholder="::" />
        </el-form-item>
        <el-form-item :label="$t('in.port')">
          <el-input-number v-model="(srv as any).listen_port" :min="0" :max="65535" controls-position="right" style="width: 100%" />
        </el-form-item>
      </div>
      <JsonEditorBlock :data="srv" @update:data="(v) => (srv = v)" />
    </el-form>

    <template #footer>
      <el-button @click="closeModal">{{ $t('actions.close') }}</el-button>
      <el-button type="primary" :loading="loading" @click="saveChanges">{{ $t('actions.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, watch } from 'vue'
import { Srv, SrvTypes, createSrv } from '@/types/services'
import RandomUtil from '@/plugins/randomUtil'
import Data from '@/store/modules/data'
import JsonEditorBlock from '@/components/JsonEditorBlock.vue'

const props = defineProps<{
  visible: boolean
  id: number
  data: string
  inTags: string[]
  tsTags: string[]
  ssTags: string[]
  tlsConfigs: any[]
}>()
const emit = defineEmits<{ close: []; 'update:modelValue': [v: boolean] }>()
void props.inTags; void props.tsTags; void props.ssTags; void props.tlsConfigs

const srv = ref<Srv>(createSrv('resolved', { tag: '' } as any))
const title = ref<'add' | 'edit'>('add')
const loading = ref(false)

const updateData = (id: number) => {
  if (id > 0) {
    srv.value = JSON.parse(props.data)
    title.value = 'edit'
  } else {
    srv.value = createSrv('resolved', { tag: 'resolved-' + RandomUtil.randomSeq(3) } as any)
    title.value = 'add'
  }
}

const changeType = () => {
  const tag = props.id > 0 ? srv.value.tag : srv.value.type + '-' + RandomUtil.randomSeq(3)
  srv.value = createSrv(srv.value.type, { tag, listen: (srv.value as any).listen, listen_port: (srv.value as any).listen_port } as any)
}

const closeModal = () => emit('close')

const saveChanges = async () => {
  if (!props.visible) return
  if (Data().checkTag('service', props.id, srv.value.tag)) return
  loading.value = true
  const success = await Data().save('services', props.id == 0 ? 'new' : 'edit', srv.value)
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
