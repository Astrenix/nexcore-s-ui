<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="$emit('close')"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="$t('rule.import.title')"
    destroy-on-close
  >
    <el-tabs v-model="tab" @tab-change="tabChanged" class="import-tabs">
      <el-tab-pane :label="$t('rule.import.text', '文本')" name="text">
        <el-input
          v-model="importRawText"
          type="textarea"
          :rows="6"
          placeholder="https://...&#10;https://..."
          spellcheck="false"
          class="mono"
        />
      </el-tab-pane>
      <el-tab-pane :label="$t('rule.import.upload', '文件')" name="file">
        <el-upload :auto-upload="false" :show-file-list="false" :on-change="onFileChange">
          <el-button><el-icon><Upload /></el-icon>{{ $t('rule.import.upload', '选择文件') }}</el-button>
        </el-upload>
      </el-tab-pane>
    </el-tabs>

    <div class="form-grid">
      <el-form-item :label="$t('ruleset.format')">
        <el-select v-model="importFormat">
          <el-option label="binary" value="binary" />
          <el-option label="source" value="source" />
        </el-select>
      </el-form-item>
      <el-form-item :label="$t('objects.outbound')">
        <el-select v-model="importDetour" clearable filterable>
          <el-option v-for="t in outTags" :key="t" :label="t" :value="t" />
        </el-select>
      </el-form-item>
      <el-form-item :label="`${$t('ruleset.interval')} (${$t('date.d')})`">
        <el-input-number v-model="importInterval" :min="0" controls-position="right" style="width: 100%" />
      </el-form-item>
    </div>

    <div class="actions-row">
      <el-button type="primary" :disabled="!importRawText" @click="parseImport">
        <el-icon><Search /></el-icon>{{ $t('rule.import.parse') }}
      </el-button>
    </div>

    <div v-if="importPreview.length" class="preview-section">
      <div class="preview-section__head">
        {{ $t('rule.ruleset') }} ({{ newCount }} new
        <template v-if="importSkipped > 0">/ {{ importSkipped }} skipped</template>)
      </div>
      <el-table :data="importPreview" size="small" max-height="200">
        <el-table-column prop="tag" :label="$t('objects.tag')" min-width="140">
          <template #default="{ row }">
            <span :class="{ 'row-skipped': row.exists }">{{ row.tag }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="url" label="URL" min-width="240" show-overflow-tooltip />
        <el-table-column prop="format" :label="$t('ruleset.format')" width="100" />
      </el-table>
    </div>

    <template #footer>
      <el-button @click="$emit('close')">{{ $t('actions.close') }}</el-button>
      <el-button type="primary" :disabled="newCount === 0" @click="save">{{ $t('actions.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue'
import { Search, Upload } from '@element-plus/icons-vue'

interface ImportItem { tag: string; url: string; format: string; exists: boolean }

const props = defineProps<{ visible: boolean; outTags: string[]; rsTags: string[] }>()
const emit = defineEmits<{ close: []; save: [items: any[]]; 'update:modelValue': [v: boolean] }>()

const tab = ref<'text' | 'file'>('text')
const importRawText = ref('')
const importFormat = ref('binary')
const importDetour = ref('')
const importInterval = ref(1)
const importPreview = ref<ImportItem[]>([])

const importSkipped = computed(() => importPreview.value.filter((i) => i.exists).length)
const newCount = computed(() => importPreview.value.filter((i) => !i.exists).length)

const tabChanged = () => {
  importPreview.value = []
  importRawText.value = ''
}

const urlToTag = (url: string): string => {
  try {
    const filename = new URL(url).pathname.split('/').pop() ?? ''
    return filename.replace(/\.[^.]+$/, '')
  } catch {
    const parts = url.split('/')
    return parts[parts.length - 1].replace(/\.[^.]+$/, '') || url
  }
}

const parseImport = () => {
  const existing = new Set(props.rsTags)
  const seen = new Set<string>()
  importPreview.value = importRawText.value
    .split('\n')
    .map((l) => l.trim())
    .filter((l) => l.length > 0 && l.startsWith('http'))
    .filter((url) => {
      if (seen.has(url)) return false
      seen.add(url)
      return true
    })
    .map((url) => ({
      tag: urlToTag(url),
      url,
      format: importFormat.value,
      exists: existing.has(urlToTag(url)),
    }))
}

const save = () => {
  const items = importPreview.value
    .filter((i) => !i.exists)
    .map((item) => {
      const rs: any = { type: 'remote', tag: item.tag, format: item.format, url: item.url }
      if (importDetour.value) rs.download_detour = importDetour.value
      if (importInterval.value > 0) rs.update_interval = importInterval.value + 'd'
      return rs
    })
  emit('save', items)
}

const onFileChange = async (file: any) => {
  const raw = file?.raw ?? file
  if (!raw) return
  importRawText.value = await raw.text()
  parseImport()
}

watch(() => props.visible, (v) => {
  if (v) {
    tab.value = 'text'
    tabChanged()
  }
})
</script>

<style scoped>
.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 6px 16px;
  margin-top: 12px;
}

.actions-row {
  display: flex;
  justify-content: flex-end;
  margin-top: 8px;
}

.preview-section {
  margin-top: 12px;
}

.preview-section__head {
  font-size: 11.5px;
  font-weight: 600;
  color: var(--nc-text-muted);
  margin-bottom: 6px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.row-skipped {
  color: var(--nc-text-faint);
  text-decoration: line-through;
}

.import-tabs :deep(.el-tabs__nav-wrap::after) { display: none; }
</style>
