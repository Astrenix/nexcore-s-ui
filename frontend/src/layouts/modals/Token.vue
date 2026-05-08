<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="$emit('close')"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="$t('admin.api.title')"
    destroy-on-close
  >
    <el-alert v-if="newToken.token.length > 0" type="success" :closable="false" show-icon class="token-alert">
      <template #title>{{ $t('admin.api.msg') }}</template>
      <el-input :model-value="newToken.token" readonly>
        <template #append>
          <el-button @click="copyToClipboard(newToken.token)"><el-icon><DocumentCopy /></el-icon></el-button>
        </template>
      </el-input>
    </el-alert>

    <el-table :data="tokens" v-loading="loading" size="small" stripe>
      <el-table-column prop="id" label="#" width="60" />
      <el-table-column prop="token" :label="$t('admin.api.token')" min-width="180" show-overflow-tooltip />
      <el-table-column prop="desc" :label="$t('client.desc')" min-width="120" show-overflow-tooltip />
      <el-table-column :label="$t('date.expiry')" width="120">
        <template #default="{ row }">
          <span class="mono">{{ dateFormatted(row.expiry) }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="$t('actions.del')" width="80" align="center">
        <template #default="{ row }">
          <el-popconfirm
            :title="$t('confirm')"
            :confirm-button-text="$t('yes')"
            :cancel-button-text="$t('no')"
            @confirm="deleteToken(row.id)"
          >
            <template #reference>
              <el-button text>
                <el-icon style="color: var(--nc-danger)"><Delete /></el-icon>
              </el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <div class="token-add">
      <el-button type="primary" @click="showAddToken">
        <el-icon><Plus /></el-icon>{{ $t('actions.add') }}
      </el-button>
    </div>

    <el-dialog
      v-model="showNewToken"
      class="constrained-dialog"
      :title="$t('admin.api.token')"
      :align-center="false"
      append-to-body
    >
      <el-form label-position="top">
        <el-form-item :label="$t('client.desc')">
          <el-input v-model="newToken.desc" />
        </el-form-item>
        <el-form-item :label="`${$t('date.expiry')} (${$t('date.d')})`">
          <el-input-number v-model="newToken.expiry" :min="0" controls-position="right" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showNewToken = false">{{ $t('actions.close') }}</el-button>
        <el-button type="primary" @click="addToken">{{ $t('actions.add') }}</el-button>
      </template>
    </el-dialog>

    <template #footer>
      <el-button @click="$emit('close')">{{ $t('actions.close') }}</el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, watch, computed } from 'vue'
import { i18n } from '@/locales'
import HttpUtils from '@/plugins/httputil'
import Clipboard from 'clipboard'
import { ElMessage } from 'element-plus'
import { DocumentCopy, Delete, Plus } from '@element-plus/icons-vue'

const props = defineProps<{ visible: boolean }>()
defineEmits<{ close: []; 'update:modelValue': [v: boolean] }>()

const loading = ref(false)
const tokens = ref<any[]>([])
const showNewToken = ref(false)
const newToken = ref({ desc: '', token: '', expiry: 0 })

const locale = computed(() => {
  const l = i18n.global.locale.value
  if (l === 'zhHans') return 'zh-cn'
  if (l === 'zhHant') return 'zh-tw'
  return l
})

const loadData = async () => {
  loading.value = true
  const data = await HttpUtils.get('api/tokens')
  if (data.success) tokens.value = data.obj ?? []
  loading.value = false
}

const resetNewToken = () => { newToken.value = { desc: '', token: '', expiry: 30 } }
const showAddToken = () => { resetNewToken(); showNewToken.value = true }

const addToken = async () => {
  loading.value = true
  newToken.value.expiry = newToken.value.expiry > 0 ? newToken.value.expiry : 0
  const r = await HttpUtils.post('api/addToken', { desc: newToken.value.desc, expiry: newToken.value.expiry })
  if (r.success) {
    newToken.value.token = r.obj
    loadData()
    showNewToken.value = false
  }
  loading.value = false
}

const deleteToken = async (id: number) => {
  loading.value = true
  const r = await HttpUtils.post('api/deleteToken', { id })
  if (r.success) loadData()
  loading.value = false
}

const dateFormatted = (expiry: number) => {
  if (expiry === 0) return i18n.global.t('unlimited')
  return new Date(expiry * 1000).toLocaleString(locale.value, { year: 'numeric', month: '2-digit', day: '2-digit' })
}

const copyToClipboard = (txt: string) => {
  const hidden = document.createElement('button')
  hidden.className = 'clipboard-btn'
  document.body.appendChild(hidden)
  const cb = new Clipboard('.clipboard-btn', { text: () => txt })
  cb.on('success', () => {
    cb.destroy()
    ElMessage.success(`${i18n.global.t('success')}: ${i18n.global.t('copyToClipboard')}`)
  })
  cb.on('error', () => {
    cb.destroy()
    ElMessage.error(`${i18n.global.t('failed')}: ${i18n.global.t('copyToClipboard')}`)
  })
  hidden.click()
  document.body.removeChild(hidden)
}

watch(() => props.visible, (v) => {
  if (v) {
    resetNewToken()
    loadData()
  }
})
</script>

<style scoped>
.token-alert {
  margin-bottom: 14px;
}

.token-add {
  margin-top: 12px;
}
</style>
