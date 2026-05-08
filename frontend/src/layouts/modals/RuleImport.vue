<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="$emit('close')"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="$t('rule.import.rulesTitle')"
    destroy-on-close
  >
    <el-tabs v-model="tab" @tab-change="tabChanged" class="import-tabs">
      <el-tab-pane label="JSON" name="json">
        <el-form label-position="top">
          <el-form-item :label="$t('rule.import.json', 'JSON')">
            <el-input v-model="rawJson" type="textarea" :rows="10" spellcheck="false" class="mono" />
          </el-form-item>
          <div class="actions-row">
            <el-button type="primary" :disabled="rawJson.trim().length === 0" @click="parseJson">
              <el-icon><Search /></el-icon>{{ $t('rule.import.parse') }}
            </el-button>
          </div>
        </el-form>
      </el-tab-pane>
      <el-tab-pane label="URL" name="url">
        <el-form label-position="top">
          <el-form-item label="URL">
            <el-input v-model="fetchUrl" placeholder="https://..." />
          </el-form-item>
          <div class="actions-row">
            <el-button type="primary" :loading="fetching" :disabled="!fetchUrl" @click="fetchFromUrl">
              <el-icon><Download /></el-icon>{{ $t('actions.update') }}
            </el-button>
          </div>
        </el-form>
      </el-tab-pane>
      <el-tab-pane :label="$t('rule.import.upload', '文件')" name="file">
        <el-upload
          :auto-upload="false"
          :show-file-list="false"
          accept=".json,application/json"
          :on-change="onFileChange"
        >
          <el-button><el-icon><Upload /></el-icon>{{ $t('rule.import.upload', '选择文件') }}</el-button>
        </el-upload>
      </el-tab-pane>
    </el-tabs>

    <p v-if="error" class="error-text">{{ error }}</p>

    <template v-if="parsed">
      <div class="nc-divider"><span>{{ $t('rule.import.preview', 'PREVIEW') }}</span></div>

      <el-radio-group v-model="mode" class="mode-row">
        <el-radio-button label="merge">{{ $t('rule.import.merge', 'merge') }}</el-radio-button>
        <el-radio-button label="replace">{{ $t('rule.import.replace', 'replace') }}</el-radio-button>
      </el-radio-group>
      <el-checkbox v-model="applyFinal" v-if="parsed.final" class="apply-final">
        {{ $t('basic.routing.defaultOut') }}: {{ parsed.final }}
      </el-checkbox>

      <div v-if="parsed.rules?.length" class="preview-section">
        <div class="preview-section__head">{{ $t('pages.rules') }} ({{ parsed.rules.length }})</div>
        <el-table :data="parsed.rules" size="small" max-height="180">
          <el-table-column prop="action" :label="$t('admin.action')" width="100" />
          <el-table-column prop="outbound" :label="$t('objects.outbound')" min-width="120" />
        </el-table>
      </div>
      <div v-if="parsed.rule_set?.length" class="preview-section">
        <div class="preview-section__head">
          {{ $t('rule.ruleset') }} ({{ parsed.rule_set.length }})
          <el-tag v-if="skippedRulesets > 0" size="small" type="warning">{{ skippedRulesets }} {{ $t('rule.import.skipped') }}</el-tag>
        </div>
        <el-table :data="parsed.rule_set" size="small" max-height="180">
          <el-table-column prop="tag" :label="$t('objects.tag')" min-width="120" />
          <el-table-column prop="format" :label="$t('ruleset.format')" width="100" />
          <el-table-column prop="type" :label="$t('type')" width="100" />
          <el-table-column prop="update_interval" :label="$t('ruleset.interval')" width="100" />
        </el-table>
      </div>
    </template>

    <template #footer>
      <el-button @click="$emit('close')">{{ $t('actions.close') }}</el-button>
      <el-button type="primary" :disabled="!parsed" @click="save">{{ $t('actions.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue'
import { Search, Download, Upload } from '@element-plus/icons-vue'
import { i18n } from '@/locales'

const props = defineProps<{
  visible: boolean
  existingRulesCount: number
  existingRulesetsCount: number
  existingRulesetTags: string[]
}>()
const emit = defineEmits<{
  close: []
  save: [block: any, mode: 'merge' | 'replace', applyFinal: boolean]
  'update:modelValue': [v: boolean]
}>()

const tab = ref<'json' | 'url' | 'file'>('json')
const rawJson = ref('')
const fetchUrl = ref('')
const fetching = ref(false)
const error = ref('')
const parsed = ref<any>(null)
const mode = ref<'merge' | 'replace'>('merge')
const applyFinal = ref(false)

const hasConflicts = computed(() => props.existingRulesCount > 0 || props.existingRulesetsCount > 0)
const skippedRulesets = computed(() => {
  if (!parsed.value?.rule_set) return 0
  const set = new Set(props.existingRulesetTags)
  return parsed.value.rule_set.filter((rs: any) => set.has(rs.tag)).length
})

const tabChanged = () => {
  rawJson.value = ''
  fetchUrl.value = ''
  error.value = ''
  parsed.value = null
  mode.value = hasConflicts.value ? 'merge' : 'replace'
  applyFinal.value = false
}

const extractRouteBlock = (obj: any) => {
  if (obj?.route && (obj.route.rules || obj.route.rule_set)) return obj.route
  if (obj?.rules || obj?.rule_set) return obj
  return null
}

const setParsed = (block: any) => {
  parsed.value = block
  mode.value = hasConflicts.value ? 'merge' : 'replace'
  applyFinal.value = false
}

const reset = () => {
  tab.value = 'json'
  tabChanged()
}

const parseJson = () => {
  error.value = ''
  parsed.value = null
  try {
    const block = extractRouteBlock(JSON.parse(rawJson.value))
    if (!block) {
      error.value = i18n.global.t('rule.import.errNoArrays')
      return
    }
    setParsed(block)
  } catch (e: any) {
    error.value = i18n.global.t('rule.import.errJsonParse', { message: e.message })
  }
}

const fetchFromUrl = async () => {
  error.value = ''
  parsed.value = null
  fetching.value = true
  try {
    const resp = await fetch(fetchUrl.value)
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
    const block = extractRouteBlock(await resp.json())
    if (!block) error.value = i18n.global.t('rule.import.errNoArraysFetched')
    else setParsed(block)
  } catch (e: any) {
    error.value = i18n.global.t('rule.import.errFetch', { message: e.message })
  } finally {
    fetching.value = false
  }
}

const onFileChange = async (file: any) => {
  error.value = ''
  parsed.value = null
  const raw = file?.raw ?? file
  if (!raw) {
    error.value = i18n.global.t('rule.import.errNoFile')
    return
  }
  try {
    const text = await raw.text()
    const block = extractRouteBlock(JSON.parse(text))
    if (!block) {
      error.value = i18n.global.t('rule.import.errNoArraysInFile')
      return
    }
    setParsed(block)
  } catch (e: any) {
    error.value = i18n.global.t('rule.import.errJsonParse', { message: e.message })
  }
}

const save = () => {
  if (!parsed.value) return
  emit('save', parsed.value, mode.value, applyFinal.value)
}

watch(() => props.visible, (v) => { if (v) reset() })
</script>

<style scoped>
.actions-row {
  display: flex;
  justify-content: flex-end;
}

.error-text {
  color: var(--nc-danger);
  font-family: var(--font-mono);
  font-size: 12px;
  margin: 8px 0 0;
}

.mode-row {
  margin: 4px 0 8px;
}

.apply-final {
  margin-bottom: 8px;
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
  display: flex;
  align-items: center;
  gap: 6px;
}

.import-tabs :deep(.el-tabs__nav-wrap::after) { display: none; }
</style>
