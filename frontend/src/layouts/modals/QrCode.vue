<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="$emit('close')"
    class="constrained-dialog"
    :align-center="false"
    title="QrCode"
    destroy-on-close
  >
    <div v-if="loading" class="qrcode-loading">
      <el-icon class="is-loading"><Loading /></el-icon>
    </div>

    <div v-else class="qrcode-tabs nc-tabs">
      <el-tabs v-model="tab">
        <el-tab-pane :label="$t('setting.sub')" name="sub">
          <div class="qrcode-stack">
            <div class="qrcode-block">
              <span class="qrcode-block__title">{{ $t('setting.sub') }}</span>
              <QrcodeVue :value="clientSub" :size="size" :margin="1" class="qrcode-img" @click="copyToClipboard(clientSub)" />
            </div>
            <div class="qrcode-block">
              <span class="qrcode-block__title">{{ $t('setting.jsonSub') }}</span>
              <QrcodeVue :value="clientSub + '?format=json'" :size="size" :margin="1" class="qrcode-img" @click="copyToClipboard(clientSub + '?format=json')" />
            </div>
            <div class="qrcode-block">
              <span class="qrcode-block__title">{{ $t('setting.clashSub') }}</span>
              <QrcodeVue :value="clientSub + '?format=clash'" :size="size" :margin="1" class="qrcode-img" @click="copyToClipboard(clientSub + '?format=clash')" />
            </div>
            <div class="qrcode-block">
              <span class="qrcode-block__title">SING-BOX</span>
              <QrcodeVue :value="singbox" :size="size" :margin="1" class="qrcode-img qrcode-img--scan" />
            </div>
          </div>
        </el-tab-pane>
        <el-tab-pane :label="$t('client.links')" name="link">
          <div class="qrcode-stack">
            <div v-for="(l, i) in clientLinks" :key="i" class="qrcode-block">
              <span class="qrcode-block__title">{{ l.remark ?? $t('client.' + l.type) }}</span>
              <QrcodeVue :value="l.uri" :size="size" :margin="1" class="qrcode-img" @click="copyToClipboard(l.uri)" />
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue'
import QrcodeVue from 'qrcode.vue'
import Data from '@/store/modules/data'
import Clipboard from 'clipboard'
import { i18n } from '@/locales'
import { ElMessage } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'

const props = defineProps<{ id: number; visible: boolean }>()
defineEmits<{ close: []; 'update:modelValue': [v: boolean] }>()

const tab = ref('sub')
const client = ref<any>({})
const loading = ref(false)

const load = async () => {
  loading.value = true
  client.value = await Data().loadClients(props.id)
  loading.value = false
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

const clientSub = computed(() => Data().subURI + (client.value.name ?? ''))
const singbox = computed(() => {
  const url = Data().subURI + (client.value.name ?? '') + '?format=json'
  return 'sing-box://import-remote-profile?url=' + encodeURIComponent(url) + '#' + (client.value.name ?? '')
})
const clientLinks = computed(() => client.value.links ?? [])

const size = computed(() => {
  if (typeof window === 'undefined') return 260
  if (window.innerWidth > 480) return 240
  if (window.innerWidth > 360) return 200
  return 180
})

watch(() => props.visible, (v) => {
  if (v) {
    tab.value = 'sub'
    load()
  }
})
</script>

<style scoped>
.qrcode-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 60px 0;
  font-size: 32px;
  color: var(--nc-primary);
}

.qrcode-tabs {
  background: transparent;
  border: none;
  border-radius: 0;
  overflow: visible;
}

.qrcode-stack {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 18px;
  padding: 4px 0;
}

.qrcode-block {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
}

.qrcode-block__title {
  font-size: 11.5px;
  color: var(--nc-text-muted);
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  background: var(--nc-border-soft);
  padding: 3px 10px;
  border-radius: var(--radius-pill);
}

.qrcode-img {
  border-radius: 12px;
  cursor: copy;
  padding: 6px;
  background: #fff;
  border: 1px solid var(--nc-border);
}

.qrcode-img--scan {
  cursor: not-allowed;
}
</style>
