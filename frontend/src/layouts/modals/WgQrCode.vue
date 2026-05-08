<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="$emit('close')"
    class="constrained-dialog"
    :align-center="false"
    title="WireGuard QrCode"
    destroy-on-close
  >
    <div v-for="(l, i) in wgLinks" :key="i" v-show="l.length > 0" class="wg-block">
      <div class="wg-block__head">
        <span class="wg-block__title">{{ $t('types.wg.peer') }} {{ i + 1 }}</span>
        <el-tooltip :content="$t('actions.update')" placement="top">
          <el-button text @click="download(l, i)">
            <el-icon><Download /></el-icon>
          </el-button>
        </el-tooltip>
      </div>
      <QrcodeVue :value="l" :size="size" :margin="1" class="wg-block__qr" @click="copyToClipboard(l)" />
    </div>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue'
import QrcodeVue from 'qrcode.vue'
import Clipboard from 'clipboard'
import { i18n } from '@/locales'
import { ElMessage } from 'element-plus'
import { Download } from '@element-plus/icons-vue'

const props = defineProps<{ data: any; visible: boolean }>()
defineEmits<{ close: []; 'update:modelValue': [v: boolean] }>()

const wgData = ref<any>({})
const wgLinks = ref<string[]>([])

const load = () => {
  wgData.value = props.data
  wgLinks.value = []
  const address = location.hostname
  wgData.value.peers?.forEach((_: any, idx: number) => {
    wgLinks.value.push(getWireguardLink(idx, address))
  })
}

const getWireguardLink = (peerId: number, address: string) => {
  const peer = wgData.value.peers[peerId]
  const keys = wgData.value.ext?.keys?.find((k: any) => k.public_key === peer.public_key)
  if (!keys || !wgData.value.ext?.public_key) return ''
  let txt = `[Interface]\nPrivateKey = ${keys.private_key}\nAddress = ${peer.allowed_ips.join(',')}\nDNS = ${wgData.value.ext?.dns?.length > 0 ? wgData.value.ext.dns : '1.1.1.1, 9.9.9.9'}\n`
  if (wgData.value.mtu) txt += `MTU = ${wgData.value.mtu}\n`
  txt += `\n# ${wgData.value.tag} - ${peerId}\n[Peer]\nPublicKey = ${wgData.value.ext.public_key}\nAllowedIPs = 0.0.0.0/0, ::/0\nEndpoint = ${address}:${wgData.value.listen_port}\n`
  if (peer.pre_shared_key) txt += `\nPresharedKey = ${peer.pre_shared_key}`
  if (peer.persistent_keepalive_interval) txt += `\nPersistentKeepalive = ${peer.persistent_keepalive_interval}\n`
  return txt
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

const download = (text: string, i: number) => {
  const filename = `${wgData.value.tag}_peer_${i + 1}.conf`
  const a = document.createElement('a')
  a.href = 'data:application/json;charset=utf-8,' + encodeURIComponent(text)
  a.download = filename
  a.style.display = 'none'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

const size = computed(() => {
  if (typeof window === 'undefined') return 240
  if (window.innerWidth > 480) return 240
  if (window.innerWidth > 360) return 200
  return 180
})

watch(() => props.visible, (v) => { if (v) load() })
</script>

<style scoped>
.wg-block {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
}

.wg-block__head {
  display: flex;
  align-items: center;
  gap: 6px;
}

.wg-block__title {
  font-size: 11.5px;
  color: var(--nc-text-muted);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  background: var(--nc-border-soft);
  padding: 3px 10px;
  border-radius: var(--radius-pill);
}

.wg-block__qr {
  border-radius: 12px;
  cursor: copy;
  padding: 6px;
  background: #fff;
  border: 1px solid var(--nc-border);
}
</style>
