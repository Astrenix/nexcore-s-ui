<template>
  <el-dialog
    :model-value="control.visible"
    @update:model-value="(v) => (control.visible = v)"
    @close="control.visible = false"
    class="constrained-dialog"
    :align-center="false"
    :title="$t('main.backup.title')"
    destroy-on-close
  >
    <div class="backup-section">
      <div class="nc-card-title">{{ $t('main.backup.title') }}</div>
      <div class="backup-checks">
        <el-checkbox v-model="excludeStats">{{ $t('main.backup.exclStats') }}</el-checkbox>
        <el-checkbox v-model="excludeChanges">{{ $t('main.backup.exclChanges') }}</el-checkbox>
      </div>
      <div class="backup-actions">
        <el-button type="primary" @click="backup">
          <el-icon><Download /></el-icon>{{ $t('main.backup.backup') }}
        </el-button>
        <el-button @click="restore">
          <el-icon><Upload /></el-icon>{{ $t('main.backup.restore') }}
        </el-button>
      </div>
    </div>

    <div class="backup-section">
      <div class="nc-card-title">{{ $t('main.backup.sbConfig') }}</div>
      <div class="backup-actions">
        <el-button @click="config">
          <el-icon><Document /></el-icon>{{ $t('main.backup.sbConfig') }}
        </el-button>
      </div>
    </div>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, watch, computed } from 'vue'
import HttpUtils from '@/plugins/httputil'
import { Download, Upload, Document } from '@element-plus/icons-vue'

const props = defineProps<{ control: any; visible: boolean }>()

const excludeStats = ref(true)
const excludeChanges = ref(true)
const exclude = computed(() => [
  ...(excludeStats.value ? ['stats'] : []),
  ...(excludeChanges.value ? ['changes'] : []),
])

watch(() => props.visible, (v) => {
  if (v) {
    excludeStats.value = true
    excludeChanges.value = true
  }
})

const backup = () => {
  const opt = exclude.value.length > 0 ? '?exclude=' + exclude.value.join(',') : ''
  window.location.href = 'api/getdb' + opt
}

const config = () => {
  window.location.href = 'api/singbox-config'
}

const restore = () => {
  const fileInput = document.createElement('input')
  fileInput.type = 'file'
  fileInput.accept = '.db'
  fileInput.addEventListener('change', async (event: Event) => {
    const inputElement = event.target as HTMLInputElement
    const dbFile = inputElement.files ? inputElement.files[0] : null
    if (dbFile) {
      const formData = new FormData()
      formData.append('db', dbFile)
      props.control.visible = false
      const uploadMsg = await HttpUtils.post('api/importdb', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      })
      if (uploadMsg.success) {
        await new Promise((r) => setTimeout(r, 1000))
        location.reload()
      }
    }
  })
  fileInput.click()
}
</script>

<style scoped>
.backup-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 18px;
}

.backup-section:last-child {
  margin-bottom: 0;
}

.nc-card-title {
  font-size: 11px;
  font-weight: 600;
  color: var(--nc-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.backup-checks {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
}

.backup-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
</style>
