<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="closeModal"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="`${$t('actions.addbulk')} ${$t('objects.outbound')}`"
    destroy-on-close
  >
    <template v-if="outbounds.length === 0">
      <el-form label-position="top">
        <el-form-item :label="$t('client.sub')">
          <el-input
            v-model="link"
            placeholder="http[s]://<domain>[:]<port>/<path>"
            dir="ltr"
          />
        </el-form-item>
        <el-form-item>
          <el-checkbox v-model="addUrlTest">{{ $t('out.addUrlTest') }}</el-checkbox>
        </el-form-item>
        <div class="actions-center">
          <el-button type="primary" :loading="loading" @click="linkCheck">{{ $t('submit') }}</el-button>
        </div>
      </el-form>
    </template>

    <el-table v-else :data="outbounds" v-loading="loading" size="small" stripe>
      <el-table-column width="50" align="center">
        <template #default="{ $index }">
          <el-icon v-if="outChecks[$index] === 1" style="color: var(--nc-success)"><CircleCheck /></el-icon>
          <el-icon v-else-if="outChecks[$index] === 2" style="color: var(--nc-danger)"><CircleClose /></el-icon>
          <el-icon v-else-if="outChecks[$index] === 3" class="is-loading"><Loading /></el-icon>
          <el-icon v-else style="color: var(--nc-text-faint)"><QuestionFilled /></el-icon>
        </template>
      </el-table-column>
      <el-table-column prop="type" :label="$t('type')" width="110" />
      <el-table-column prop="tag" :label="$t('objects.tag')" min-width="120" />
      <el-table-column :label="$t('out.addr')" min-width="180">
        <template #default="{ row }">
          <span class="mono">{{ row.server }}{{ row.server_port ? ':' + row.server_port : '' }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('objects.tls')" width="80">
        <template #default="{ row }">
          {{ Object.hasOwn(row, 'tls') ? $t(row.tls?.enabled ? 'enable' : 'disable') : '—' }}
        </template>
      </el-table-column>
    </el-table>

    <template #footer>
      <el-button @click="closeModal">{{ $t('actions.close') }}</el-button>
      <el-button
        type="primary"
        :loading="loading"
        :disabled="outbounds.length === 0"
        @click="saveChanges"
      >{{ $t('actions.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, watch } from 'vue'
import HttpUtils from '@/plugins/httputil'
import RandomUtil from '@/plugins/randomUtil'
import Data from '@/store/modules/data'
import { createOutbound, Outbound } from '@/types/outbounds'
import { CircleCheck, CircleClose, Loading, QuestionFilled } from '@element-plus/icons-vue'

const props = defineProps<{ visible: boolean; outboundTags: string[] }>()
const emit = defineEmits<{ close: []; 'update:modelValue': [v: boolean] }>()

const loading = ref(false)
const link = ref('')
const outbounds = ref<Outbound[]>([])
const outChecks = ref<number[]>([])
const addUrlTest = ref(false)

const resetData = () => {
  outbounds.value = []
  outChecks.value = []
  link.value = ''
  addUrlTest.value = false
  loading.value = false
}

const closeModal = () => {
  resetData()
  emit('close')
}

const linkCheck = async () => {
  loading.value = true
  outbounds.value = []
  const msg = await HttpUtils.post('api/subConvert', { link: link.value })
  if (msg.success && msg.obj?.length > 0) {
    msg.obj.forEach((o: any, idx: number) => {
      if (outbounds.value.map((x) => x.tag).includes(o.tag)) o.tag = o.tag + '-' + (idx + 1)
      outbounds.value.push(createOutbound(o.type, o))
      outChecks.value.push(0)
    })
    if (addUrlTest.value) {
      const utTag = 'urltest-' + RandomUtil.randomSeq(3)
      outbounds.value.push(createOutbound('urltest', {
        tag: utTag,
        outbounds: outbounds.value.map((o: Outbound) => o.tag),
        interrupt_exist_connections: false,
        interval: '30s',
      }))
      outChecks.value.push(0)
    }
  }
  loading.value = false
}

const saveChanges = async () => {
  if (!props.visible) return
  outbounds.value.forEach((o: Outbound, idx: number) => {
    const dup = Data().checkTag('outbound', 0, o.tag)
    outChecks.value[idx] = dup ? 2 : 0
  })
  loading.value = true
  await Promise.all(outbounds.value.map(async (o: Outbound, idx: number) => {
    if (outChecks.value[idx] === 2) return
    outChecks.value[idx] = 3
    const success = await Data().save('outbounds', 'new', o)
    outChecks.value[idx] = success ? 1 : 2
  }))
  loading.value = false
}

watch(() => props.visible, (v) => { if (v) resetData() })
</script>

<style scoped>
.actions-center {
  display: flex;
  justify-content: center;
  margin-top: 8px;
}
</style>
