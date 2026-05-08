<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="closeModal"
    @opened="updateData(id)"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="$t('actions.' + title) + ' ' + $t('objects.inbound')"
    destroy-on-close
  >
    <div v-if="loading" class="modal-loading">
      <el-icon class="is-loading"><Loading /></el-icon>
    </div>

    <el-form v-else label-position="top">
      <div class="form-grid">
        <el-form-item :label="$t('type')">
          <el-select v-model="inbound.type" filterable @change="changeType">
            <el-option
              v-for="(v, k) in InTypes"
              :key="k"
              :label="k"
              :value="v"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('objects.tag')">
          <el-input v-model="inbound.tag" />
        </el-form-item>
        <template v-if="inbound.type !== InTypes.Tun">
          <el-form-item :label="$t('in.addr')">
            <el-input v-model="inbound.listen" placeholder="::" />
          </el-form-item>
          <el-form-item :label="$t('in.port')">
            <el-input-number v-model="inbound.listen_port" :min="1" :max="65535" controls-position="right" style="width: 100%" />
          </el-form-item>
        </template>
        <el-form-item v-if="hasTls" :label="$t('objects.tls')">
          <el-select v-model="inbound.tls_id" clearable>
            <el-option :value="0" :label="$t('disable')" />
            <el-option v-for="t in tlsConfigs" :key="t.id" :value="t.id" :label="t.name" />
          </el-select>
        </el-form-item>
      </div>

      <Users v-if="hasUser" :clients="clientList" :data="initUsers" />

      <div class="advanced-section">
        <div class="advanced-section__head">
          <span class="advanced-section__title">{{ $t('client.config') }} (JSON)</span>
          <el-tooltip :content="$t('actions.update')" placement="top">
            <el-button text @click="syncFromJson">
              <el-icon><RefreshRight /></el-icon>
            </el-button>
          </el-tooltip>
        </div>
        <p class="advanced-section__hint">
          完整协议 / TLS / Transport 字段编辑界面在阶段 4 重写;当前可通过 JSON 直接调整。
        </p>
        <el-input
          v-model="inboundJson"
          type="textarea"
          :rows="14"
          spellcheck="false"
          class="json-editor mono"
          @change="onJsonEdit"
        />
        <p v-if="jsonError" class="json-error">{{ jsonError }}</p>
      </div>
    </el-form>

    <template #footer>
      <el-button @click="closeModal">{{ $t('actions.close') }}</el-button>
      <el-button type="primary" :loading="loading" :disabled="!validate" @click="saveChanges">
        {{ $t('actions.save') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue'
import { InTypes, createInbound, ShadowTLS } from '@/types/inbounds'
import RandomUtil from '@/plugins/randomUtil'
import Data from '@/store/modules/data'
import Users from '@/components/Users.vue'
import { Loading, RefreshRight } from '@element-plus/icons-vue'

const props = defineProps<{
  visible: boolean
  id: number
  inTags: string[]
  tlsConfigs: any[]
}>()
const emit = defineEmits<{ close: []; 'update:modelValue': [v: boolean] }>()
void props.inTags

const inbound = ref<any>(createInbound('direct', { id: 0, tag: '' }))
const title = ref<'add' | 'edit'>('add')
const loading = ref(false)
const inboundJson = ref('{}')
const jsonError = ref('')

const inboundWithUsers = ['mixed', 'socks', 'http', 'shadowsocks', 'vmess', 'trojan', 'naive', 'hysteria', 'shadowtls', 'tuic', 'hysteria2', 'vless', 'anytls']
const HasTls = [InTypes.HTTP, InTypes.VMess, InTypes.Trojan, InTypes.Naive, InTypes.Hysteria, InTypes.TUIC, InTypes.Hysteria2, InTypes.VLESS, InTypes.AnyTls]
const OnlyTLS = [InTypes.Hysteria, InTypes.Hysteria2, InTypes.TUIC, InTypes.Naive, InTypes.AnyTls]
const HasInData = [InTypes.SOCKS, InTypes.HTTP, InTypes.Mixed, InTypes.Shadowsocks, InTypes.VMess, InTypes.ShadowTLS, InTypes.Trojan, InTypes.Hysteria, InTypes.VLESS, InTypes.AnyTls, InTypes.TUIC, InTypes.Hysteria2, InTypes.Naive]

const initUsers = ref<{ model: string; values: any[] }>({ model: 'none', values: [] })

const clientList = computed<any[]>(() => Data().clients ?? [])

const hasTls = computed(() => HasTls.includes(inbound.value.type))

const hasUser = computed(() => {
  if (props.id > 0) return false
  if (!inboundWithUsers.includes(inbound.value.type)) return false
  if (inbound.value.type === InTypes.ShadowTLS && (<ShadowTLS>inbound.value).version < 3) return false
  if ((inbound.value as any).managed) return false
  return true
})

const validate = computed(() => {
  if (!inbound.value || !inbound.value.tag) return false
  if (inbound.value.type !== InTypes.Tun && (inbound.value.listen_port > 65535 || inbound.value.listen_port < 1)) return false
  if (OnlyTLS.includes(inbound.value.type) && !inbound.value.tls_id) return false
  return true
})

const refreshJson = () => {
  inboundJson.value = JSON.stringify(inbound.value, null, 2)
  jsonError.value = ''
}

const syncFromJson = () => {
  refreshJson()
}

const onJsonEdit = () => {
  try {
    const parsed = JSON.parse(inboundJson.value)
    if (typeof parsed === 'object' && parsed !== null) {
      inbound.value = parsed
      jsonError.value = ''
    }
  } catch (e: any) {
    jsonError.value = `JSON: ${e.message}`
  }
}

watch(() => inbound.value.type, refreshJson)
watch(() => inbound.value.tag, refreshJson)
watch(() => inbound.value.listen, refreshJson)
watch(() => inbound.value.listen_port, refreshJson)
watch(() => inbound.value.tls_id, refreshJson)

const loadData = async (id: number) => {
  loading.value = true
  const arr = await Data().loadInbounds([id])
  inbound.value = arr[0]
  if (HasInData.includes(inbound.value.type) && inbound.value.out_json == null) {
    inbound.value.out_json = {}
  }
  refreshJson()
  loading.value = false
}

const updateData = (id: number) => {
  if (id > 0) {
    loadData(id)
    title.value = 'edit'
  } else {
    const port = RandomUtil.randomIntRange(10000, 60000)
    inbound.value = createInbound('direct', { id: 0, tag: 'direct-' + port, listen: '::', listen_port: port })
    if (HasInData.includes(inbound.value.type)) {
      inbound.value.addrs = []
      inbound.value.out_json = {}
    } else {
      delete inbound.value.addrs
      delete inbound.value.out_json
    }
    title.value = 'add'
    loading.value = false
    refreshJson()
  }
  initUsers.value = { model: 'none', values: [] }
}

const changeType = () => {
  if (!inbound.value.listen_port) inbound.value.listen_port = RandomUtil.randomIntRange(10000, 60000)
  const tag = props.id > 0 ? inbound.value.tag : inbound.value.type + '-' + inbound.value.listen_port
  const prev = { id: inbound.value.id, tag, listen: inbound.value.listen ?? '::', listen_port: inbound.value.listen_port }
  inbound.value = createInbound(inbound.value.type, inbound.value.type !== InTypes.Tun ? prev : { tag })
  if (HasInData.includes(inbound.value.type)) {
    inbound.value.addrs = []
    inbound.value.out_json = {}
  } else {
    delete inbound.value.addrs
    delete inbound.value.out_json
  }
  refreshJson()
}

const closeModal = () => {
  updateData(0)
  emit('close')
}

const saveChanges = async () => {
  if (!props.visible) return
  if (jsonError.value) return
  // 同步最新 JSON 编辑
  try { inbound.value = JSON.parse(inboundJson.value) } catch { /* ignore */ }
  if (Data().checkTag('inbound', inbound.value.id, inbound.value.tag)) return
  loading.value = true
  let clientIds: number[] = []
  if (hasUser.value) {
    switch (initUsers.value.model) {
      case 'all':
        clientIds = clientList.value.map((c: any) => c.id); break
      case 'group':
        clientIds = clientList.value.filter((c: any) => initUsers.value.values.includes(c.group)).map((c: any) => c.id); break
      case 'client':
        clientIds = initUsers.value.values
    }
  }
  const success = await Data().save('inbounds', props.id == 0 ? 'new' : 'edit', inbound.value, clientIds)
  if (success) closeModal()
  loading.value = false
}

watch(() => props.visible, (v) => {
  if (v) loading.value = true
})
</script>

<style scoped>
.modal-loading {
  display: flex;
  justify-content: center;
  padding: 60px 0;
  font-size: 32px;
  color: var(--nc-primary);
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 6px 16px;
  margin-bottom: 12px;
}

.advanced-section {
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px dashed var(--nc-border);
}

.advanced-section__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.advanced-section__title {
  font-size: 11.5px;
  font-weight: 600;
  color: var(--nc-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.advanced-section__hint {
  font-size: 11.5px;
  color: var(--nc-text-faint);
  margin: 4px 0 8px;
}

.json-editor :deep(.el-textarea__inner) {
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.6;
  background: #f8fafc;
  border-color: var(--nc-border);
}

.json-error {
  margin-top: 6px;
  font-size: 11.5px;
  color: var(--nc-danger);
  font-family: var(--font-mono);
}
</style>
