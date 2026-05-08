<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="closeModal"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="$t('actions.' + title) + ' ' + $t('objects.outbound')"
    destroy-on-close
  >
    <el-tabs v-model="tab">
      <el-tab-pane :label="$t('client.basics')" name="t1">
        <el-form label-position="top">
          <div class="form-grid">
            <el-form-item :label="$t('type')">
              <el-select v-model="outbound.type" filterable @change="changeType">
                <el-option v-for="(v, k) in OutTypes" :key="k" :label="k" :value="v" />
              </el-select>
            </el-form-item>
            <el-form-item :label="$t('objects.tag')">
              <el-input v-model="outbound.tag" />
            </el-form-item>
            <template v-if="!NoServer.includes(outbound.type)">
              <el-form-item :label="$t('out.addr')">
                <el-input v-model="outbound.server" />
              </el-form-item>
              <el-form-item :label="$t('out.port')">
                <el-input-number v-model="outbound.server_port" :min="0" controls-position="right" style="width: 100%" />
              </el-form-item>
            </template>
          </div>

          <div class="advanced-section">
            <div class="advanced-section__head">
              <span class="advanced-section__title">{{ $t('client.config') }} (JSON)</span>
              <el-tooltip :content="$t('actions.update')" placement="top">
                <el-button text @click="syncFromJson"><el-icon><RefreshRight /></el-icon></el-button>
              </el-tooltip>
            </div>
            <p class="advanced-section__hint">
              完整协议 / TLS / Transport 字段编辑界面在阶段 4 重写;当前可通过 JSON 直接调整。
            </p>
            <el-input
              v-model="outboundJson"
              type="textarea"
              :rows="14"
              spellcheck="false"
              class="json-editor mono"
              @change="onJsonEdit"
            />
            <p v-if="jsonError" class="json-error">{{ jsonError }}</p>
          </div>
        </el-form>
      </el-tab-pane>

      <el-tab-pane :label="$t('client.external')" name="t2">
        <el-form label-position="top">
          <el-form-item :label="$t('client.external')">
            <el-input v-model="link" />
          </el-form-item>
          <div class="actions-center">
            <el-button type="primary" :loading="loading" @click="linkConvert">{{ $t('submit') }}</el-button>
          </div>
        </el-form>
      </el-tab-pane>
    </el-tabs>

    <template #footer>
      <el-button @click="closeModal">{{ $t('actions.close') }}</el-button>
      <el-button type="primary" :loading="loading" @click="saveChanges">{{ $t('actions.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, watch } from 'vue'
import { OutTypes, createOutbound } from '@/types/outbounds'
import RandomUtil from '@/plugins/randomUtil'
import HttpUtils from '@/plugins/httputil'
import Data from '@/store/modules/data'
import { RefreshRight } from '@element-plus/icons-vue'

const props = defineProps<{ visible: boolean; data: string; id: number; tags: string[] }>()
const emit = defineEmits<{ close: []; 'update:modelValue': [v: boolean] }>()
void props.tags

const outbound = ref<any>(createOutbound('direct', { tag: '' }))
const title = ref<'add' | 'edit'>('add')
const tab = ref('t1')
const link = ref('')
const loading = ref(false)
const outboundJson = ref('{}')
const jsonError = ref('')

const NoServer = [OutTypes.Direct, OutTypes.Selector, OutTypes.URLTest, OutTypes.Tor]

const refreshJson = () => {
  outboundJson.value = JSON.stringify(outbound.value, null, 2)
  jsonError.value = ''
}

const syncFromJson = () => refreshJson()

const onJsonEdit = () => {
  try {
    const parsed = JSON.parse(outboundJson.value)
    if (typeof parsed === 'object' && parsed !== null) {
      outbound.value = parsed
      jsonError.value = ''
    }
  } catch (e: any) {
    jsonError.value = `JSON: ${e.message}`
  }
}

watch(() => outbound.value.type, refreshJson)
watch(() => outbound.value.tag, refreshJson)
watch(() => outbound.value.server, refreshJson)
watch(() => outbound.value.server_port, refreshJson)

const updateData = (id: number) => {
  if (id > 0) {
    const newData = JSON.parse(props.data)
    outbound.value = createOutbound(newData.type, newData)
    title.value = 'edit'
  } else {
    outbound.value = createOutbound('direct', { tag: 'direct-' + RandomUtil.randomSeq(3) })
    title.value = 'add'
  }
  tab.value = 't1'
  refreshJson()
}

const changeType = () => {
  const tag = props.id > 0 ? outbound.value.tag : outbound.value.type + '-' + RandomUtil.randomSeq(3)
  const prev = {
    id: outbound.value.id,
    tag,
    listen: outbound.value.listen,
    listen_port: outbound.value.listen_port,
  }
  outbound.value = createOutbound(outbound.value.type, prev)
  refreshJson()
}

const closeModal = () => {
  updateData(0)
  emit('close')
}

const saveChanges = async () => {
  if (!props.visible) return
  if (jsonError.value) return
  try { outbound.value = JSON.parse(outboundJson.value) } catch { /* ignore */ }
  if (Data().checkTag('outbound', props.id, outbound.value.tag)) return
  loading.value = true
  const success = await Data().save('outbounds', props.id == 0 ? 'new' : 'edit', outbound.value)
  if (success) closeModal()
  loading.value = false
}

const linkConvert = async () => {
  if (link.value.length > 0) {
    loading.value = true
    const msg = await HttpUtils.post('api/linkConvert', { link: link.value })
    loading.value = false
    if (msg.success) {
      outbound.value = msg.obj
      if (props.id > 0) outbound.value.id = props.id
      tab.value = 't1'
      link.value = ''
      refreshJson()
    }
  }
}

watch(() => props.visible, (v) => {
  if (v) updateData(props.id)
})
</script>

<style scoped>
.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 6px 16px;
  margin-bottom: 12px;
}

.actions-center {
  display: flex;
  justify-content: center;
  margin-top: 8px;
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
