<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="closeModal"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="$t('actions.addbulk')"
    destroy-on-close
  >
    <el-form label-position="top">
      <div class="form-grid form-grid--narrow">
        <el-form-item :label="$t('count')">
          <el-input-number v-model="count" :min="1" :max="100" controls-position="right" style="width: 100%" />
        </el-form-item>
      </div>

      <el-form-item :label="$t('client.name')">
        <NamePatternEditor v-model="bulkData.name" :patterns="patterns" />
      </el-form-item>
      <el-form-item :label="$t('client.desc')">
        <NamePatternEditor v-model="bulkData.desc" :patterns="patterns" />
      </el-form-item>

      <div class="form-grid">
        <el-form-item :label="$t('client.group')">
          <el-select v-model="bulkData.group" allow-create filterable default-first-option>
            <el-option v-for="g in groups" :key="g" :label="g.length > 0 ? g : $t('none')" :value="g" />
          </el-select>
        </el-form-item>
        <el-form-item :label="`${$t('stats.volume')} (GiB)`">
          <el-input-number v-model="bulkData.Volume" :min="0" controls-position="right" style="width: 100%" />
        </el-form-item>
        <DatePick
          v-if="!(bulkData.delayStart && !bulkData.autoReset)"
          :expiry="bulkData.expiry"
          @submit="setDate"
        />
      </div>

      <div class="form-grid">
        <el-form-item :label="$t('client.delayStart')">
          <el-switch v-model="bulkData.delayStart" />
        </el-form-item>
        <el-form-item :label="$t('client.autoReset')">
          <el-switch v-model="bulkData.autoReset" />
        </el-form-item>
        <el-form-item v-if="bulkData.autoReset || bulkData.delayStart" :label="$t('client.resetDays')">
          <el-input-number v-model="bulkData.resetDays" :min="1" controls-position="right" style="width: 100%" />
        </el-form-item>
      </div>

      <el-form-item :label="$t('client.inboundTags')">
        <el-select v-model="bulkData.clientInbounds" multiple collapse-tags collapse-tags-tooltip>
          <el-option v-for="t in inboundTags" :key="t.value" :label="t.title" :value="t.value" />
        </el-select>
        <el-button text style="margin-left: 4px" @click="setAllInbounds">{{ $t('all') }}</el-button>
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="closeModal">{{ $t('actions.close') }}</el-button>
      <el-button type="primary" :loading="loading" @click="saveChanges">{{ $t('actions.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, watch, h, defineComponent } from 'vue'
import DatePick from '@/components/DateTime.vue'
import { ElMessage } from 'element-plus'
import RandomUtil from '@/plugins/randomUtil'
import { Client, createClient, randomConfigs } from '@/types/clients'
import { i18n } from '@/locales'
import Data from '@/store/modules/data'

const props = defineProps<{ visible: boolean; inboundTags: any[]; groups: string[] }>()
const emit = defineEmits<{ close: []; 'update:modelValue': [v: boolean] }>()

const count = ref(1)
const clients = ref<Client[]>([])
const patterns = [
  { title: i18n.global.t('bulk.random'), value: 'random' },
  { title: i18n.global.t('bulk.order'), value: 'order' },
]

const initBulkData = () => ({
  name: <any[]>[patterns[1], '-', patterns[0]],
  desc: <any[]>[],
  group: '',
  clientInbounds: <number[]>[],
  expiry: 0,
  Volume: 0,
  delayStart: false,
  autoReset: false,
  resetDays: 0,
})

const bulkData = ref(initBulkData())
const loading = ref(false)

const closeModal = () => emit('close')

const saveChanges = async () => {
  if (!props.visible) return
  // 必须包含至少一个 pattern token(对象类型)
  if (bulkData.value.name.findIndex((n: any) => typeof n === 'object') === -1) {
    ElMessage.error(i18n.global.t('error.dplData'))
    return
  }
  clients.value = []
  loading.value = true
  for (let i = 0; i < count.value; i++) {
    const name = genByPattern(bulkData.value.name, i)
    clients.value.push(createClient({
      enable: true,
      name,
      config: randomConfigs(name),
      inbounds: bulkData.value.clientInbounds.length > 0 ? [...bulkData.value.clientInbounds].sort() : [],
      links: [],
      volume: bulkData.value.Volume * 1024 ** 3,
      expiry: bulkData.value.delayStart && !bulkData.value.autoReset ? 0 : bulkData.value.expiry,
      up: 0,
      down: 0,
      desc: genByPattern(bulkData.value.desc, i),
      group: bulkData.value.group,
      delayStart: bulkData.value.delayStart,
      autoReset: bulkData.value.autoReset,
      resetDays: bulkData.value.resetDays,
    }))
  }
  if (Data().checkBulkClientNames(clients.value.map((c) => c.name))) {
    loading.value = false
    return
  }
  const success = await Data().save('clients', 'addbulk', clients.value)
  if (success) closeModal()
  loading.value = false
}

const genByPattern = (pattern: any[], order: number) => {
  if (pattern.length === 0) return RandomUtil.randomSeq(8)
  let result = ''
  pattern.forEach((p) => {
    if (typeof p === 'object') {
      switch (p.value) {
        case 'random': result += RandomUtil.randomSeq(8); break
        case 'order': result += order + 1
      }
    } else {
      result += p
    }
  })
  return result
}

const setDate = (v: number) => { bulkData.value.expiry = v }
const setAllInbounds = () => {
  bulkData.value.clientInbounds = props.inboundTags.map((i: any) => i.value).sort()
}

watch(() => props.visible, (v) => { if (v) bulkData.value = initBulkData() })

// 简易 pattern 编辑器:展示当前 token 序列,允许追加 token / 文本
const NamePatternEditor = defineComponent({
  props: { modelValue: { type: Array, required: true }, patterns: { type: Array, required: true } },
  emits: ['update:modelValue'],
  setup(p, { emit }) {
    const text = ref('')
    const removeAt = (i: number) => {
      const arr = (p.modelValue as any[]).slice()
      arr.splice(i, 1)
      emit('update:modelValue', arr)
    }
    const addText = () => {
      if (!text.value) return
      emit('update:modelValue', [...(p.modelValue as any[]), text.value])
      text.value = ''
    }
    const addToken = (tok: any) => {
      emit('update:modelValue', [...(p.modelValue as any[]), tok])
    }
    return () => h('div', { class: 'pattern-editor' }, [
      h('div', { class: 'pattern-editor__list' }, (p.modelValue as any[]).map((tok, i) => h(
        'span',
        {
          class: ['pattern-chip', typeof tok === 'object' ? 'pattern-chip--token' : 'pattern-chip--text'],
          onClick: () => removeAt(i),
        },
        typeof tok === 'object' ? tok.title : tok,
      ))),
      h('div', { class: 'pattern-editor__add' }, [
        h('input', {
          class: 'pattern-editor__input',
          placeholder: '+ text',
          value: text.value,
          onInput: (e: any) => (text.value = e.target.value),
          onKeyup: (e: any) => { if (e.key === 'Enter') addText() },
        }),
        ...(p.patterns as any[]).map((pp) => h('button', {
          type: 'button',
          class: 'pattern-editor__btn',
          onClick: () => addToken(pp),
        }, '+ ' + pp.title)),
      ]),
    ])
  },
})
</script>

<style scoped>
.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 6px 16px;
}

.form-grid--narrow {
  grid-template-columns: repeat(auto-fit, minmax(160px, 240px));
}

:deep(.pattern-editor) {
  display: flex;
  flex-direction: column;
  gap: 6px;
  width: 100%;
}

:deep(.pattern-editor__list) {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  padding: 6px;
  border: 1px solid var(--nc-border);
  border-radius: var(--radius-md);
  min-height: 36px;
  align-items: center;
}

:deep(.pattern-chip) {
  font-size: 11.5px;
  padding: 2px 8px;
  border-radius: var(--radius-pill);
  cursor: pointer;
  border: 1px solid var(--nc-border);
  font-family: var(--font-mono);
}

:deep(.pattern-chip--token) {
  background: var(--nc-primary-soft);
  color: var(--nc-primary-deep);
  border-color: var(--nc-primary-border);
  font-family: var(--font-display);
}

:deep(.pattern-chip--text) {
  background: var(--nc-border-soft);
}

:deep(.pattern-editor__add) {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

:deep(.pattern-editor__input) {
  height: 30px;
  padding: 0 8px;
  border: 1px solid var(--nc-border);
  border-radius: var(--radius-md);
  font-size: 12px;
  flex: 1 1 120px;
  outline: none;
}

:deep(.pattern-editor__input:focus) {
  border-color: var(--nc-primary);
}

:deep(.pattern-editor__btn) {
  height: 30px;
  padding: 0 10px;
  border-radius: var(--radius-md);
  border: 1px dashed var(--nc-border);
  background: transparent;
  font-size: 11.5px;
  color: var(--nc-text-muted);
  cursor: pointer;
}

:deep(.pattern-editor__btn:hover) {
  border-color: var(--nc-primary);
  color: var(--nc-primary);
}
</style>
