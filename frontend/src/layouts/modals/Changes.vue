<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="$emit('close')"
    class="constrained-dialog is-wide"
    :align-center="false"
    :title="$t('admin.changes')"
    destroy-on-close
  >
    <div class="changes-toolbar">
      <el-form-item :label="$t('admin.actor')" class="changes-toolbar__item">
        <el-select v-model="user" clearable @change="loadData">
          <el-option label="DepleteJob" value="DepleteJob" />
          <el-option v-for="a in admins" :key="a" :label="a" :value="a" />
        </el-select>
      </el-form-item>
      <el-form-item :label="$t('admin.key')" class="changes-toolbar__item">
        <el-select v-model="key" clearable @change="loadData">
          <el-option v-for="k in keys" :key="k" :label="k" :value="k" />
        </el-select>
      </el-form-item>
      <el-form-item :label="$t('count')" class="changes-toolbar__item">
        <el-select v-model.number="chngCount" @change="loadData">
          <el-option v-for="n in [10, 20, 30, 50, 100]" :key="n" :label="n" :value="n" />
        </el-select>
      </el-form-item>
      <el-button :loading="loading" @click="loadData">
        <el-icon><Refresh /></el-icon>
      </el-button>
    </div>

    <el-table :data="changes" v-loading="loading" size="small" stripe>
      <el-table-column type="expand">
        <template #default="{ row }">
          <div class="changes-detail mono">
            <div v-if="row.index > 0" class="changes-detail__row">Index: {{ row.index }}</div>
            <pre class="changes-detail__obj">{{ row.obj }}</pre>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column :label="`${$t('admin.date')} - ${$t('admin.time')}`" width="150">
        <template #default="{ row }">
          <span class="mono">{{ dateFormatted(row.dateTime) }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="Actor" :label="$t('admin.actor')" min-width="120" />
      <el-table-column prop="key" :label="$t('admin.key')" min-width="100" />
      <el-table-column :label="$t('admin.action')" width="100">
        <template #default="{ row }">
          <el-tag size="small" effect="plain">{{ $t('actions.' + row.action) }}</el-tag>
        </template>
      </el-table-column>
    </el-table>
  </el-dialog>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue'
import { i18n } from '@/locales'
import HttpUtils from '@/plugins/httputil'
import { Refresh } from '@element-plus/icons-vue'

const props = defineProps<{ admins: string[]; actor: string; visible: boolean }>()
defineEmits<{ close: []; 'update:modelValue': [v: boolean] }>()

const loading = ref(false)
const changes = ref<any[]>([])
const user = ref('')
const key = ref('')
const chngCount = ref(10)
const keys = ['inbounds', 'outbounds', 'clients', 'route', 'tls', 'experimental']

const localeStr = computed(() => {
  const l = i18n.global.locale.value
  if (l === 'zhHans') return 'zh-cn'
  if (l === 'zhHant') return 'zh-tw'
  return l
})

const loadData = async () => {
  loading.value = true
  const data = await HttpUtils.get('api/changes', { a: user.value, k: key.value, c: chngCount.value })
  if (data.success) changes.value = data.obj ?? []
  loading.value = false
}

const dateFormatted = (dt: number) => new Date(dt * 1000).toLocaleString(localeStr.value)

watch(() => props.visible, (v) => {
  changes.value = []
  user.value = props.actor || ''
  key.value = ''
  chngCount.value = 10
  if (v) loadData()
})
</script>

<style scoped>
.changes-toolbar {
  display: flex;
  align-items: flex-end;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.changes-toolbar__item {
  flex: 0 0 180px;
  margin: 0 !important;
}

.changes-detail {
  background: #f8fafc;
  padding: 12px 16px;
  border-radius: var(--radius-md);
  font-size: 11.5px;
}

.changes-detail__obj {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  color: var(--nc-text-3);
}
</style>
