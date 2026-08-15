<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="$emit('close')"
    class="constrained-dialog is-narrow"
    :align-center="true"
    :title="title || 'QrCode'"
    destroy-on-close
  >
    <div v-if="!link" class="qrcode-empty">
      <el-empty :description="emptyText || $t('noData')" :image-size="80" />
    </div>
    <div v-else class="qrcode-stack">
      <QrcodeVue :value="link" :size="size" :margin="1" class="qrcode-img" @click="copy" />
      <div class="qrcode-link mono select-all">{{ link }}</div>
      <el-button type="primary" size="small" @click="copy">
        <el-icon><DocumentCopy /></el-icon>{{ $t('copyLink') }}
      </el-button>
    </div>
  </el-dialog>
</template>

<script lang="ts" setup>
// 纯展示组件 — 链接由父组件传入,QR 自己只画。学 nexcore-x-ui 的
// QrcodeDialog 设计:不再自己拉数据 / 不再依赖 client.links 间接路径。
import { computed } from 'vue'
import QrcodeVue from 'qrcode.vue'
import Clipboard from 'clipboard'
import { i18n } from '@/locales'
import { ElMessage } from 'element-plus'
import { DocumentCopy } from '@element-plus/icons-vue'

const props = defineProps<{
  visible: boolean
  link?: string
  title?: string
  emptyText?: string
}>()
defineEmits<{ close: []; 'update:modelValue': [v: boolean] }>()

const size = computed(() => {
  if (typeof window === 'undefined') return 240
  if (window.innerWidth > 480) return 220
  if (window.innerWidth > 360) return 180
  return 160
})

// 兼容 http 部署环境 — navigator.clipboard 在 http 上下文会被浏览器拒,
// fallback 走 textarea + execCommand(Element Plus 用 clipboard.js 库已经
// 帮我们处理这层兼容)。
const copy = () => {
  if (!props.link) return
  const hidden = document.createElement('button')
  hidden.className = 'clipboard-copy-hidden'
  document.body.appendChild(hidden)
  const cb = new Clipboard('.clipboard-copy-hidden', { text: () => props.link! })
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
</script>

<style scoped>
.qrcode-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 30px 0;
}

.qrcode-stack {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
}

.qrcode-img {
  border-radius: 12px;
  cursor: copy;
  padding: 8px;
  background: #fff;
  border: 1px solid var(--nc-border);
}

.qrcode-link {
  font-size: 11.5px;
  color: var(--nc-text-muted);
  word-break: break-all;
  text-align: center;
  background: var(--nc-bg-3);
  padding: 8px 10px;
  border-radius: 6px;
  width: 100%;
  max-height: 80px;
  overflow-y: auto;
  user-select: all;
}
.select-all { user-select: all; }
</style>
