<template>
  <div class="page-container">
    <div class="page-header with-actions">
      <div class="page-header-text">
        <h2 class="page-title">{{ $t('pages.rules') }}</h2>
        <p class="page-desc">{{ $t('rules.desc', '路由规则、规则集与导入导出') }}</p>
      </div>
      <div class="page-header-actions">
        <el-button type="primary" @click="showRuleModal(-1)">
          <el-icon><Plus /></el-icon>{{ $t('rule.add') }}
        </el-button>
        <el-button @click="showRulesetModal(-1)">
          <el-icon><Plus /></el-icon>{{ $t('ruleset.add') }}
        </el-button>
        <el-dropdown trigger="click">
          <el-button>
            <el-icon><Tools /></el-icon>{{ $t('rule.import.title', '导入') }}
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item @click="showImportRule">
                <el-icon style="margin-right: 6px"><Connection /></el-icon>{{ $t('rule.import.rulesTitle') }}
              </el-dropdown-item>
              <el-dropdown-item @click="showImportRulesets">
                <el-icon style="margin-right: 6px"><Download /></el-icon>{{ $t('rule.import.title') }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button type="warning" plain :loading="loading" :disabled="stateChange" @click="saveConfig">
          <el-icon><Check /></el-icon>{{ $t('actions.save') }}
        </el-button>
      </div>
    </div>

    <div class="nc-card">
      <h4 class="section-title">{{ $t('basic.routing.title') }}</h4>
      <el-form label-position="top">
        <div class="form-grid">
          <el-form-item :label="$t('basic.routing.defaultOut')">
            <el-select v-model="route.final" clearable filterable>
              <el-option v-for="t in outboundTags" :key="t" :label="t" :value="t" />
            </el-select>
          </el-form-item>
          <el-form-item :label="$t('basic.routing.defaultIf')">
            <el-input v-model="route.default_interface" clearable />
          </el-form-item>
          <el-form-item :label="$t('basic.routing.defaultRm')">
            <el-input-number v-model="routeMark" :min="0" controls-position="right" style="width: 100%" />
          </el-form-item>
          <el-form-item :label="$t('basic.routing.autoBind')">
            <el-switch v-model="route.auto_detect_interface" />
          </el-form-item>
        </div>
      </el-form>
    </div>

    <div>
      <div class="nc-divider"><span>{{ $t('rule.ruleset') }} ({{ rulesets.length }})</span></div>
      <div class="cards-grid">
        <div v-for="(item, index) in (rulesets as any[])" :key="index" class="entity-card nc-card">
          <div class="entity-card__head">
            <span class="entity-card__type">{{ $t('ruleset.' + item.type) }}</span>
            <span class="entity-card__tag">{{ item.tag }}</span>
          </div>
          <dl class="entity-card__meta">
            <div class="entity-card__row"><dt>{{ $t('ruleset.format') }}</dt><dd class="mono">{{ item.format ?? '—' }}</dd></div>
            <div class="entity-card__row"><dt>{{ $t('objects.outbound') }}</dt><dd class="mono">{{ item.download_detour ?? '—' }}</dd></div>
            <div class="entity-card__row"><dt>{{ $t('actions.update') }}</dt><dd class="mono">{{ item.update_interval ?? '—' }}</dd></div>
          </dl>
          <div class="entity-card__actions">
            <el-tooltip :content="$t('actions.edit')" placement="top">
              <el-button text @click="showRulesetModal(Number(index))"><el-icon><Edit /></el-icon></el-button>
            </el-tooltip>
            <el-popconfirm :title="$t('confirm')" :confirm-button-text="$t('yes')" :cancel-button-text="$t('no')" @confirm="delRuleset(Number(index))">
              <template #reference>
                <el-button text>
                  <el-tooltip :content="$t('actions.del')" placement="top">
                    <el-icon><Delete /></el-icon>
                  </el-tooltip>
                </el-button>
              </template>
            </el-popconfirm>
          </div>
        </div>
      </div>
    </div>

    <div>
      <div class="nc-divider"><span>{{ $t('pages.rules') }} ({{ rules.length }})</span></div>
      <div class="cards-grid">
        <div
          v-for="(item, index) in (rules as any[])"
          :key="index"
          class="entity-card nc-card"
          draggable="true"
          @dragstart="onDragStart(Number(index))"
          @dragover.prevent
          @drop="onDrop(Number(index))"
        >
          <div class="entity-card__head">
            <span class="entity-card__type">#{{ Number(index) + 1 }}</span>
            <span class="entity-card__tag">{{ item.type ? `${$t('rule.logical')} (${item.mode})` : $t('rule.simple') }}</span>
          </div>
          <dl class="entity-card__meta">
            <div class="entity-card__row"><dt>{{ $t('admin.action') }}</dt><dd>{{ item.action }}</dd></div>
            <div class="entity-card__row"><dt>{{ $t('objects.outbound') }}</dt><dd>{{ item.outbound ?? '—' }}</dd></div>
            <div class="entity-card__row"><dt>{{ $t('pages.rules') }}</dt><dd class="mono">{{ item.rules ? item.rules.length : Object.keys(item).filter((r: string) => !actionKeys.includes(r)).length }}</dd></div>
            <div class="entity-card__row"><dt>{{ $t('rule.invert') }}</dt><dd>{{ $t(item.invert ? 'yes' : 'no') }}</dd></div>
          </dl>
          <div class="entity-card__actions">
            <el-tooltip :content="$t('actions.edit')" placement="top">
              <el-button text @click="showRuleModal(Number(index))"><el-icon><Edit /></el-icon></el-button>
            </el-tooltip>
            <el-popconfirm :title="$t('confirm')" :confirm-button-text="$t('yes')" :cancel-button-text="$t('no')" @confirm="delRule(Number(index))">
              <template #reference>
                <el-button text>
                  <el-tooltip :content="$t('actions.del')" placement="top">
                    <el-icon><Delete /></el-icon>
                  </el-tooltip>
                </el-button>
              </template>
            </el-popconfirm>
          </div>
        </div>
      </div>
    </div>

    <RuleVue
      v-model="ruleModal.visible"
      :visible="ruleModal.visible"
      :index="ruleModal.index"
      :data="ruleModal.data"
      :clients="clients"
      :inTags="inboundTags"
      :outTags="outboundTags"
      :rsTags="rulesetTags"
      @close="closeRuleModal"
      @save="saveRuleModal"
    />
    <RulesetVue
      v-model="rulesetModal.visible"
      :visible="rulesetModal.visible"
      :index="rulesetModal.index"
      :data="rulesetModal.data"
      :outTags="outboundTags"
      @close="closeRulesetModal"
      @save="saveRulesetModal"
    />
    <RuleImport
      v-model="importRulesModal.visible"
      :visible="importRulesModal.visible"
      :existingRulesCount="rules.length"
      :existingRulesetsCount="rulesets.length"
      :existingRulesetTags="rulesetTags"
      @save="saveImportRule"
      @close="closeImportRule"
    />
    <RulesetImport
      v-model="importRulesetsModal.visible"
      :visible="importRulesetsModal.visible"
      :outTags="outboundTags"
      :rsTags="rulesetTags"
      @save="saveImportRulesets"
      @close="closeImportRulesets"
    />
  </div>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import { computed, ref, onBeforeMount, defineAsyncComponent } from 'vue'

const RuleVue = defineAsyncComponent(() => import('@/layouts/modals/Rule.vue'))
const RulesetVue = defineAsyncComponent(() => import('@/layouts/modals/Ruleset.vue'))
const RulesetImport = defineAsyncComponent(() => import('@/layouts/modals/RulesetImport.vue'))
const RuleImport = defineAsyncComponent(() => import('@/layouts/modals/RuleImport.vue'))
import { Config } from '@/types/config'
import { actionKeys, ruleset } from '@/types/rules'
import { FindDiff } from '@/plugins/utils'
import { Plus, Edit, Delete, Tools, Connection, Download, Check } from '@element-plus/icons-vue'

const oldConfig = ref({})
const loading = ref(false)
const appConfig = computed((): Config => <Config>Data().config)

onBeforeMount(async () => {
  loading.value = true
  while (Data().lastLoad === 0) await new Promise((r) => setTimeout(r, 100))
  // 防御性兜底:确保 route 对象存在,避免 v-model 双向绑定丢失
  const cfg = Data().config as any
  if (!cfg.route) cfg.route = { rules: [], rule_set: [] }
  if (!cfg.route.rules) cfg.route.rules = []
  if (!cfg.route.rule_set) cfg.route.rule_set = []
  oldConfig.value = JSON.parse(JSON.stringify(Data().config))
  loading.value = false
})

const routeMark = computed({
  get: () => route.value.default_mark ?? 0,
  set: (v: number) => {
    if (v > 0) route.value.default_mark = v
    else if (appConfig.value.route) delete (appConfig.value.route as any).default_mark
  },
})

const stateChange = computed(() => FindDiff.deepCompare(appConfig.value, oldConfig.value))

const saveConfig = async () => {
  loading.value = true
  const success = await Data().save('config', 'set', appConfig.value)
  if (success) oldConfig.value = JSON.parse(JSON.stringify(Data().config))
  loading.value = false
}

const clients = computed(() => Data().clients?.map((c: any) => c.name) ?? [])
const route = computed((): any => appConfig.value.route ?? {})

const rules = computed(() => {
  const data = route.value
  if (!data) return []
  if (!('rules' in data) || !Array.isArray(data.rules)) data.rules = []
  return data.rules
})

const rulesets = computed(() => {
  const data = route.value
  if (!data) return []
  if (!('rule_set' in data) || !Array.isArray(data.rule_set)) data.rule_set = []
  return data.rule_set
})

const rulesetTags = computed(() => rulesets.value.map((rs: any) => rs.tag))
const outboundTags = computed(() => [
  ...(Data().outbounds?.map((o: any) => o.tag) ?? []),
  ...(Data().endpoints?.map((e: any) => e.tag) ?? []),
])
const inboundTags = computed(() => [
  ...(Data().inbounds?.map((o: any) => o.tag) ?? []),
  ...(Data().endpoints?.filter((e: any) => e.listen_port > 0).map((e: any) => e.tag) ?? []),
])

const ruleModal = ref({ visible: false, index: -1, data: '' })
const showRuleModal = (index: number) => {
  ruleModal.value.index = index
  ruleModal.value.data = index === -1 ? '' : JSON.stringify(rules.value[index])
  ruleModal.value.visible = true
}
const closeRuleModal = () => { ruleModal.value.visible = false }
const saveRuleModal = (data: any) => {
  if (ruleModal.value.index === -1) rules.value.push(data)
  else rules.value[ruleModal.value.index] = data
  ruleModal.value.visible = false
}
const delRule = (index: number) => { rules.value.splice(index, 1) }

const rulesetModal = ref({ visible: false, index: -1, data: '' })
const showRulesetModal = (index: number) => {
  rulesetModal.value.index = index
  rulesetModal.value.data = index === -1 ? '' : JSON.stringify(rulesets.value[index])
  rulesetModal.value.visible = true
}
const closeRulesetModal = () => { rulesetModal.value.visible = false }
const saveRulesetModal = (data: ruleset) => {
  if (rulesetModal.value.index === -1) rulesets.value.push(data)
  else rulesets.value[rulesetModal.value.index] = data
  rulesetModal.value.visible = false
}
const delRuleset = (index: number) => { rulesets.value.splice(index, 1) }

const draggedItemIndex = ref<number | null>(null)
const onDragStart = (index: number) => { draggedItemIndex.value = index }
const onDrop = (index: number) => {
  if (draggedItemIndex.value !== null) {
    const dragged = rules.value[draggedItemIndex.value]
    rules.value.splice(draggedItemIndex.value, 1)
    rules.value.splice(index, 0, dragged)
    draggedItemIndex.value = null
  }
}

const importRulesModal = ref({ visible: false })
const showImportRule = () => { importRulesModal.value.visible = true }
const closeImportRule = () => { importRulesModal.value.visible = false }
const saveImportRule = (block: any, mode: 'merge' | 'replace', applyFinal: boolean) => {
  if (mode === 'replace') {
    route.value.rules = block.rules ?? []
    route.value.rule_set = block.rule_set ?? []
  } else {
    const existing = new Set(rulesetTags.value)
    if (block.rules) rules.value.push(...block.rules)
    if (block.rule_set) for (const rs of block.rule_set) if (!existing.has(rs.tag)) rulesets.value.push(rs)
  }
  if (applyFinal && block.final) route.value.final = block.final
  importRulesModal.value.visible = false
}

const importRulesetsModal = ref({ visible: false })
const showImportRulesets = () => { importRulesetsModal.value.visible = true }
const closeImportRulesets = () => { importRulesetsModal.value.visible = false }
const saveImportRulesets = (items: any[]) => {
  rulesets.value.push(...items)
  importRulesetsModal.value.visible = false
}
</script>

<style scoped>
.section-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--nc-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  margin-bottom: 12px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 6px 16px;
}

.cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 12px;
}

.entity-card { display: flex; flex-direction: column; gap: 10px; padding: 14px 16px 10px; cursor: grab; }
.entity-card:active { cursor: grabbing; }
.entity-card__head { display: flex; align-items: center; justify-content: space-between; gap: 8px; border-bottom: 1px solid var(--nc-border-soft); padding-bottom: 8px; }
.entity-card__type { font-size: 11px; font-weight: 600; color: var(--nc-primary); background: var(--nc-primary-soft); padding: 2px 8px; border-radius: var(--radius-pill); text-transform: uppercase; letter-spacing: 0.04em; }
.entity-card__tag { font-family: var(--font-display); font-size: 13px; font-weight: 600; color: var(--nc-text-1); flex: 1; text-align: right; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.entity-card__meta { margin: 0; display: flex; flex-direction: column; gap: 4px; }
.entity-card__row { display: flex; justify-content: space-between; align-items: center; gap: 8px; font-size: 12.5px; }
.entity-card__row dt { color: var(--nc-text-muted); }
.entity-card__row dd { margin: 0; color: var(--nc-text-1); font-weight: 500; }
.entity-card__row .mono { font-family: var(--font-mono); }
.entity-card__actions { display: flex; gap: 4px; border-top: 1px solid var(--nc-border-soft); padding-top: 4px; margin: 4px -4px -4px; }
.entity-card__actions .el-button { flex: 1; min-width: 0; height: 32px; margin: 0 !important; }
</style>
