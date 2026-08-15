<template>
  <div class="page-container">
    <div class="page-header with-actions">
      <div class="page-header-text">
        <h2 class="page-title">{{ $t('pages.rules') }}</h2>
        <p class="page-desc">{{ $t('rule.page.desc') }}</p>
      </div>
      <div class="page-header-actions">
        <el-button @click="applyBestPractice">
          <el-icon><MagicStick /></el-icon>{{ $t('rule.page.bestPractice') }}
        </el-button>
        <el-button type="primary" @click="showRuleModal(-1)">
          <el-icon><Plus /></el-icon>{{ $t('rule.add') }}
        </el-button>
        <el-button @click="showRulesetModal(-1)">
          <el-icon><Plus /></el-icon>{{ $t('ruleset.add') }}
        </el-button>
        <el-dropdown trigger="click">
          <el-button>
            <el-icon><MagicStick /></el-icon>{{ $t('rule.tmpl.title') }}
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item @click="applyTemplate('block-ads')">
                <el-icon style="margin-right: 6px"><CircleClose /></el-icon>{{ $t('rule.tmpl.blockAds') }}
              </el-dropdown-item>
              <el-dropdown-item @click="applyTemplate('block-tracker')">
                <el-icon style="margin-right: 6px"><Warning /></el-icon>{{ $t('rule.tmpl.blockTracker') }}
              </el-dropdown-item>
              <el-dropdown-item @click="applyTemplate('block-porn')">
                <el-icon style="margin-right: 6px"><WarnTriangleFilled /></el-icon>{{ $t('rule.tmpl.blockPorn') }}
              </el-dropdown-item>
              <el-dropdown-item @click="applyTemplate('cn-direct')">
                <el-icon style="margin-right: 6px"><Location /></el-icon>{{ $t('rule.tmpl.cnDirect') }}
              </el-dropdown-item>
              <el-dropdown-item @click="applyTemplate('private-direct')">
                <el-icon style="margin-right: 6px"><Lock /></el-icon>{{ $t('rule.tmpl.privateDirect') }}
              </el-dropdown-item>
              <el-dropdown-item divided @click="applyTemplate('block-ads,block-tracker,private-direct,cn-direct')">
                <el-icon style="margin-right: 6px"><Star /></el-icon>{{ $t('rule.tmpl.recommended') }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-dropdown trigger="click">
          <el-button>
            <el-icon><Tools /></el-icon>{{ $t('rule.import.title') }}
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
          <el-form-item>
            <template #label>
              <span>{{ $t('rule.page.finalLabel') }}</span>
              <el-tooltip :content="$t('rule.page.finalTip')" placement="top">
                <el-icon class="label-tip"><QuestionFilled /></el-icon>
              </el-tooltip>
            </template>
            <el-select v-model="route.final" clearable filterable :placeholder="$t('rule.page.finalPlaceholder')">
              <el-option v-for="t in outboundTags" :key="t" :label="t" :value="t" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <template #label>
              <span>{{ $t('rule.page.defaultIfLabel') }}</span>
              <el-tooltip :content="$t('rule.page.defaultIfTip')" placement="top">
                <el-icon class="label-tip"><QuestionFilled /></el-icon>
              </el-tooltip>
            </template>
            <el-input v-model="route.default_interface" clearable :placeholder="$t('rule.page.defaultIfPlaceholder')" />
          </el-form-item>
          <el-form-item>
            <template #label>
              <span>{{ $t('rule.page.markLabel') }}</span>
              <el-tooltip :content="$t('rule.page.markTip')" placement="top">
                <el-icon class="label-tip"><QuestionFilled /></el-icon>
              </el-tooltip>
            </template>
            <el-input-number v-model="routeMark" :min="0" controls-position="right" style="width: 100%" />
          </el-form-item>
          <el-form-item>
            <template #label>
              <span>{{ $t('rule.page.autoDetectLabel') }}</span>
              <el-tooltip :content="$t('rule.page.autoDetectTip')" placement="top">
                <el-icon class="label-tip"><QuestionFilled /></el-icon>
              </el-tooltip>
            </template>
            <el-switch v-model="route.auto_detect_interface" />
          </el-form-item>
        </div>
      </el-form>
    </div>

    <!-- 推荐路由规则 — 开关即用 -->
    <div class="nc-card preset-card">
      <div class="preset-head">
        <h4 class="section-title">{{ $t('rule.page.recRules') }}</h4>
        <span class="preset-hint">{{ $t('rule.page.toggleAutoSave') }}</span>
      </div>
      <div class="preset-grid">
        <div v-for="p in routeRulePresets" :key="p.key" class="preset-item">
          <div class="preset-item__main">
            <div class="preset-item__title">
              <span class="preset-item__icon" :style="{ background: p.color }">{{ p.iconText }}</span>
              <span class="preset-item__name">{{ p.name }}</span>
              <el-tag v-if="p.badge" size="small" :type="p.badgeType" effect="plain">{{ p.badge }}</el-tag>
            </div>
            <div class="preset-item__desc">{{ p.desc }}</div>
          </div>
          <el-switch :model-value="isPresetEnabled(p.key)" @change="(v) => togglePreset(p, v)" />
        </div>
      </div>
    </div>

    <div>
      <div class="nc-divider"><span>{{ $t('rule.ruleset') }} ({{ rulesets.length }})</span></div>
      <div v-if="!rulesets.length" class="empty-state">
        {{ $t('rule.page.emptyRulesets1') }}<br />
        {{ $t('rule.page.emptyRulesets2a') }}<b>{{ $t('rule.tmpl.title') }}</b>{{ $t('rule.page.emptyRulesets2b') }}
      </div>
      <div v-else class="cards-grid">
        <div v-for="(item, index) in (rulesets as any[])" :key="index" class="entity-card nc-card">
          <div class="entity-card__head">
            <span class="entity-card__type">{{ $t('ruleset.' + item.type) }}</span>
            <span class="entity-card__tag">{{ item.tag }}</span>
          </div>
          <dl class="entity-card__meta">
            <div class="entity-card__row"><dt>{{ $t('ruleset.format') }}</dt><dd class="mono">{{ item.format ?? '—' }}</dd></div>
            <div class="entity-card__row"><dt>{{ $t('objects.outbound') }}</dt><dd class="mono">{{ item.download_detour ?? '—' }}</dd></div>
            <div class="entity-card__row"><dt>{{ $t('actions.update') }}</dt><dd class="mono">{{ item.update_interval ?? '—' }}</dd></div>
            <div v-if="item.url" class="entity-card__row"><dt>{{ $t('rule.page.source') }}</dt><dd class="mono ellipsis" :title="item.url">{{ shortenUrl(item.url) }}</dd></div>
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
      <div v-if="!rules.length" class="empty-state">
        {{ $t('rule.page.emptyRules1') }}<br />
        {{ $t('rule.page.emptyRules2a') }}<b>{{ $t('rule.tmpl.title') }}</b>{{ $t('rule.page.emptyRules2b') }}<b>{{ $t('rule.add') }}</b>{{ $t('rule.page.emptyRules2c') }}
      </div>
      <div v-else class="cards-grid">
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
            <span class="entity-card__type" :class="ruleActionClass(item)">#{{ Number(index) + 1 }} · {{ ruleActionLabel(item) }}</span>
            <span class="entity-card__tag">{{ item.type ? `${$t('rule.logical')} (${item.mode})` : $t('rule.simple') }}</span>
          </div>
          <dl class="entity-card__meta">
            <div class="entity-card__row"><dt>{{ $t('admin.action') }}</dt><dd>{{ item.action ?? '—' }}</dd></div>
            <div class="entity-card__row"><dt>{{ $t('objects.outbound') }}</dt><dd class="mono">{{ item.outbound ?? '—' }}</dd></div>
            <div v-if="Array.isArray(item.rule_set) && item.rule_set.length" class="entity-card__row">
              <dt>{{ $t('rule.ruleset') }}</dt>
              <dd class="mono ellipsis" :title="item.rule_set.join(', ')">{{ item.rule_set.join(', ') }}</dd>
            </div>
            <div class="entity-card__row"><dt>{{ $t('rule.page.conditionCount') }}</dt><dd class="mono">{{ item.rules ? item.rules.length : Object.keys(item).filter((r: string) => !actionKeys.includes(r)).length }}</dd></div>
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
import { Plus, Edit, Delete, Tools, Connection, Download, Check, MagicStick, CircleClose, Warning, WarnTriangleFilled, Location, Lock, Star, QuestionFilled } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { i18n } from '@/locales'

const oldConfig = ref({})
const loading = ref(false)
// appConfig 是 store.config 的**本地深拷贝**(非引用):编辑/自愈只作用在副本,
// save 成功才落库。防止未保存的路由编辑污染 Pinia store、被别页整份保存顺带
// 持久化 + 被轮询覆盖(幽灵保存)。onBeforeMount 在 config 加载完成后深拷贝初始化。
const appConfig = ref<Config>({} as Config)

// 自检:配置中是否引用了 direct(rule_set 的 download_detour 或路由规则的 outbound)
const configReferencesDirect = (): boolean => {
  const cfg = appConfig.value as any
  const ruleSets = (cfg?.route?.rule_set as any[]) ?? []
  if (ruleSets.some((rs: any) => rs.download_detour === 'direct')) return true
  const routeRules = (cfg?.route?.rules as any[]) ?? []
  if (routeRules.some((r: any) => r?.outbound === 'direct')) return true
  return false
}

const isDirectOutboundMissing = (): boolean => {
  const list = (Data().outbounds as any[]) ?? []
  return !list.some((o: any) => o.tag === 'direct' && o.type === 'direct')
}

onBeforeMount(async () => {
  loading.value = true
  let waited = 0
  while (Data().lastLoad === 0 && waited < 100) {
    await new Promise((r) => setTimeout(r, 100))
    waited++
  }
  // 深拷贝到本地副本,之后所有编辑/自愈都作用在 cfg(= appConfig.value)上
  appConfig.value = JSON.parse(JSON.stringify(Data().config ?? {}))
  const cfg = appConfig.value as any
  // 防御性兜底:确保 route 对象存在,避免 v-model 双向绑定丢失
  if (!cfg.route) cfg.route = { rules: [], rule_set: [] }
  if (!cfg.route.rules) cfg.route.rules = []
  if (!cfg.route.rule_set) cfg.route.rule_set = []

  // 全面自愈 — sing-box 启动失败常见类型
  const fixed: string[] = []

  // 1) direct 出站缺失(被 rule_set download_detour 或路由规则 outbound 引用)
  if (configReferencesDirect() && isDirectOutboundMissing()) {
    await Data().save('outbounds', 'new', { type: 'direct', tag: 'direct' })
    fixed.push(i18n.global.t('rule.fix.addDirect'))
  }

  // 2) outbounds 完全为空,sing-box 启动失败
  if (((Data().outbounds as any[]) ?? []).length === 0) {
    await Data().save('outbounds', 'new', { type: 'direct', tag: 'direct' })
    fixed.push(i18n.global.t('rule.fix.addOutbounds'))
  }

  // 3) 路由规则 outbound 引用了不存在的 outbound tag → 清空 outbound 字段(规则保留,走 final)
  const outboundTags = new Set(((Data().outbounds as any[]) ?? []).map((o: any) => o.tag).filter(Boolean))
  const endpointTagsSet = new Set(((Data().endpoints as any[]) ?? []).map((e: any) => e.tag).filter(Boolean))
  if (configReferencesDirect()) outboundTags.add('direct')
  let orphanOutboundCount = 0
  for (const r of (cfg.route.rules as any[])) {
    if (r?.outbound && !outboundTags.has(r.outbound) && !endpointTagsSet.has(r.outbound)) {
      delete r.outbound
      orphanOutboundCount++
    }
  }
  if (orphanOutboundCount > 0) fixed.push(i18n.global.t('rule.fix.clearOrphanOutbound', { n: orphanOutboundCount }))

  // 4) route.final 悬空清空
  if (cfg.route.final && !outboundTags.has(cfg.route.final) && !endpointTagsSet.has(cfg.route.final)) {
    fixed.push(i18n.global.t('rule.fix.clearRouteFinal', { val: cfg.route.final }))
    delete cfg.route.final
  }

  // 5) 路由规则的 rule_set 引用了不存在的 rule_set tag → 清空 rule_set 字段(规则保留)
  // 这里不主动补 rule_set,因为 Rules 页面不知道每个 tag 应对应什么 URL;
  // 如果规则引用 geosite-cn 等预设标签,DNS.vue 进入时会负责补全。
  const ruleSetTags = new Set((cfg.route.rule_set as any[]).map((rs: any) => rs.tag).filter(Boolean))
  let orphanRuleSetCount = 0
  for (const r of (cfg.route.rules as any[])) {
    if (Array.isArray(r?.rule_set)) {
      const filtered = r.rule_set.filter((tag: string) => ruleSetTags.has(tag))
      if (filtered.length !== r.rule_set.length) {
        if (filtered.length === 0) delete r.rule_set
        else r.rule_set = filtered
        orphanRuleSetCount++
      }
    }
  }
  if (orphanRuleSetCount > 0) fixed.push(i18n.global.t('rule.fix.clearOrphanRuleSet', { n: orphanRuleSetCount }))

  if (fixed.length) {
    const ok = await Data().save('config', 'set', cfg)
    if (ok) {
      appConfig.value = JSON.parse(JSON.stringify(Data().config ?? {}))
      ElMessage.success(i18n.global.t('rule.msg.configFixed', { list: fixed.join(';') }))
    } else {
      ElMessage.warning(i18n.global.t('rule.msg.fixedButSaveFailed', { list: fixed.join(';') }))
    }
  }

  oldConfig.value = JSON.parse(JSON.stringify(appConfig.value))
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

// 共享保存:开关切换 / 模板 / 最佳实践 / modal 提交 / 删除 / 拖拽 / 导入,
// 都走它。基础参数表单(final / default_interface / default_mark / auto_detect_interface)
// 仍走头部「保存」按钮,避免每个 keystroke 都打到后端 sing-box reload。
const autoSave = async (label?: string): Promise<boolean> => {
  loading.value = true
  // 1) direct 出站缺失时补(rule_set download_detour 或路由规则 outbound 引用)
  if (configReferencesDirect() && isDirectOutboundMissing()) {
    await Data().save('outbounds', 'new', { type: 'direct', tag: 'direct' })
  }
  // 2) outbounds 完全为空兜底(sing-box 1.10+ 不再隐式提供 direct)
  if (((Data().outbounds as any[]) ?? []).length === 0) {
    await Data().save('outbounds', 'new', { type: 'direct', tag: 'direct' })
  }
  // 3) 路由规则引用了不存在的 rule_set tag → 清空字段(规则保留)
  //    用户删 ruleset 后,引用它的 rule 就悬空,这里兜底,免得 sing-box 启动失败
  const knownTags = new Set(rulesets.value.map((rs: any) => rs.tag).filter(Boolean))
  let cleaned = 0
  for (const r of (rules.value as any[])) {
    if (Array.isArray(r?.rule_set)) {
      const filtered = r.rule_set.filter((t: string) => knownTags.has(t))
      if (filtered.length !== r.rule_set.length) {
        if (filtered.length === 0) delete r.rule_set
        else r.rule_set = filtered
        cleaned++
      }
    }
  }
  if (cleaned > 0) ElMessage.info(i18n.global.t('rule.msg.autoCleanRuleSet', { n: cleaned }))

  // 4) route.final 悬空 → 清空,sing-box 才不会启动失败
  const outboundTagSet = new Set([
    ...(((Data().outbounds as any[]) ?? []).map((o: any) => o.tag).filter(Boolean)),
    ...(((Data().endpoints as any[]) ?? []).map((e: any) => e.tag).filter(Boolean)),
  ])
  if (configReferencesDirect()) outboundTagSet.add('direct')
  if (route.value?.final && !outboundTagSet.has(route.value.final)) {
    delete (route.value as any).final
  }

  const success = await Data().save('config', 'set', appConfig.value)
  if (success) {
    oldConfig.value = JSON.parse(JSON.stringify(appConfig.value))
    if (label) ElMessage.success(label)
  } else if (label) {
    ElMessage.error(i18n.global.t('rule.msg.saveFailed'))
  }
  loading.value = false
  return success
}

const saveConfig = () => autoSave()

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
const saveRuleModal = async (data: any) => {
  const isNew = ruleModal.value.index === -1
  if (isNew) rules.value.push(data)
  else rules.value[ruleModal.value.index] = data
  ruleModal.value.visible = false
  await autoSave(isNew ? i18n.global.t('rule.msg.ruleAdded') : i18n.global.t('rule.msg.ruleUpdated'))
}
const delRule = async (index: number) => {
  rules.value.splice(index, 1)
  await autoSave(i18n.global.t('rule.msg.ruleDeleted'))
}

const rulesetModal = ref({ visible: false, index: -1, data: '' })
const showRulesetModal = (index: number) => {
  rulesetModal.value.index = index
  rulesetModal.value.data = index === -1 ? '' : JSON.stringify(rulesets.value[index])
  rulesetModal.value.visible = true
}
const closeRulesetModal = () => { rulesetModal.value.visible = false }
const saveRulesetModal = async (data: ruleset) => {
  const isNew = rulesetModal.value.index === -1
  if (isNew) rulesets.value.push(data)
  else rulesets.value[rulesetModal.value.index] = data
  rulesetModal.value.visible = false
  await autoSave(isNew ? i18n.global.t('rule.msg.rulesetAdded') : i18n.global.t('rule.msg.rulesetUpdated'))
}
const delRuleset = async (index: number) => {
  // 删之前先扫一下有没有规则在引用,有就告警(autoSave 的 self-check 会清悬空引用)
  const tag = rulesets.value[index]?.tag
  const refCount = tag ? (rules.value as any[]).filter((r: any) => Array.isArray(r.rule_set) && r.rule_set.includes(tag)).length : 0
  rulesets.value.splice(index, 1)
  if (refCount > 0) {
    ElMessage.warning(i18n.global.t('rule.msg.rulesetRefCleared', { tag, n: refCount }))
  }
  await autoSave(i18n.global.t('rule.msg.rulesetDeleted'))
}

// ---------- 一键路由模板 ----------
// URL 必须实际存在于 sing-geosite/rule-set 分支(2026-05 校验过)。
// SagerNet/sing-geosite 是从 v2fly geosite 派生,不维护安全相关分类
// (没有 malware/phishing/cryptominers),所以这里只放真实存在的 srs。
// 每个 template 给一个 rule_set + 一条 rule。reject 类用 action=reject;
// 直连类用 outbound=direct。详情可在生成后双击编辑微调。
// 用 jsdelivr CDN 镜像 SagerNet/sing-geosite 的 rule-set 分支 — 国内可访问，
// 避免国内落地机直拉 raw.githubusercontent.com 失败导致 sing-box 启动报 rejected。
const SRS_BASE = 'https://cdn.jsdelivr.net/gh/SagerNet/sing-geosite@rule-set'

const TEMPLATES: Record<string, { tag: string; url: string; action?: string; outbound?: string }> = {
  'block-ads': {
    tag: 'tmpl-ads',
    url: `${SRS_BASE}/geosite-category-ads-all.srs`,
    action: 'reject',
  },
  'block-tracker': {
    tag: 'tmpl-tracker',
    url: `${SRS_BASE}/geosite-category-public-tracker.srs`,
    action: 'reject',
  },
  'block-porn': {
    tag: 'tmpl-porn',
    url: `${SRS_BASE}/geosite-category-porn.srs`,
    action: 'reject',
  },
  'cn-direct': {
    tag: 'tmpl-cn',
    url: `${SRS_BASE}/geosite-cn.srs`,
    outbound: 'direct',
  },
  'private-direct': {
    tag: 'tmpl-private',
    url: `${SRS_BASE}/geosite-private.srs`,
    outbound: 'direct',
  },
}

const applyTemplate = async (keysCsv: string) => {
  let added = 0
  let skipped: string[] = []
  let needsDirect = false
  const queued: { rs: any; rule: any }[] = []
  for (const key of keysCsv.split(',').map((s) => s.trim()).filter(Boolean)) {
    const t = TEMPLATES[key]
    if (!t) continue
    if (rulesets.value.some((rs: any) => rs.tag === t.tag)) {
      skipped.push(t.tag)
      continue
    }
    needsDirect = true // download_detour:'direct' + 可能的 outbound:'direct'
    const rs = {
      tag: t.tag,
      type: 'remote',
      format: 'binary',
      url: t.url,
      download_detour: 'direct',
      update_interval: '24h',
    } as any
    const rule: any = { rule_set: [t.tag] }
    if (t.action) rule.action = t.action
    else if (t.outbound) rule.outbound = t.outbound
    queued.push({ rs, rule })
  }
  // 先确保 direct 出站存在,再注入规则集和规则
  if (needsDirect) await ensureDirectOutbound()
  for (const q of queued) {
    rulesets.value.push(q.rs)
    rules.value.push(q.rule)
    added++
  }
  if (added > 0) {
    await autoSave(`${i18n.global.t('rule.tmpl.applied')}: +${added}`)
  } else if (skipped.length > 0) {
    ElMessage.info(`${i18n.global.t('rule.tmpl.alreadyExists')}: ${skipped.join(', ')}`)
  }
}

const draggedItemIndex = ref<number | null>(null)
const onDragStart = (index: number) => { draggedItemIndex.value = index }
const onDrop = async (index: number) => {
  if (draggedItemIndex.value === null) return
  if (draggedItemIndex.value === index) { draggedItemIndex.value = null; return }
  const dragged = rules.value[draggedItemIndex.value]
  rules.value.splice(draggedItemIndex.value, 1)
  rules.value.splice(index, 0, dragged)
  draggedItemIndex.value = null
  await autoSave(i18n.global.t('rule.msg.orderSaved'))
}

const importRulesModal = ref({ visible: false })
const showImportRule = () => { importRulesModal.value.visible = true }
const closeImportRule = () => { importRulesModal.value.visible = false }
const saveImportRule = async (block: any, mode: 'merge' | 'replace', applyFinal: boolean) => {
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
  await autoSave(mode === 'replace' ? i18n.global.t('rule.msg.importRulesReplaced') : i18n.global.t('rule.msg.importRulesMerged'))
}

const importRulesetsModal = ref({ visible: false })
const showImportRulesets = () => { importRulesetsModal.value.visible = true }
const closeImportRulesets = () => { importRulesetsModal.value.visible = false }
const saveImportRulesets = async (items: any[]) => {
  rulesets.value.push(...items)
  importRulesetsModal.value.visible = false
  await autoSave(i18n.global.t('rule.msg.importRulesetsDone', { n: items.length }))
}

// ---------- 推荐路由规则（开关即用） ----------
type RoutePreset = {
  key: string
  name: string
  desc: string
  iconText: string
  color: string
  badge?: string
  badgeType?: 'success' | 'info' | 'warning' | 'danger'
  ruleSets?: { tag: string; url: string }[]
  match: (r: any) => boolean
  build: () => any
}

const SRS_GS = 'https://cdn.jsdelivr.net/gh/SagerNet/sing-geosite@rule-set'
const SRS_GI = 'https://cdn.jsdelivr.net/gh/SagerNet/sing-geoip@rule-set'

const routeRulePresets: RoutePreset[] = [
  {
    key: 'sniff',
    name: i18n.global.t('rule.preset.sniffName'),
    desc: i18n.global.t('rule.preset.sniffDesc'),
    iconText: '👁',
    color: '#7c3aed',
    badge: i18n.global.t('rule.preset.sniffBadge'),
    badgeType: 'success',
    match: (r) => r?.action === 'sniff',
    build: () => ({ action: 'sniff' }),
  },
  {
    key: 'hijack-dns',
    name: i18n.global.t('rule.preset.hijackName'),
    desc: i18n.global.t('rule.preset.hijackDesc'),
    iconText: '🔌',
    color: '#d97706',
    badge: i18n.global.t('rule.preset.hijackBadge'),
    badgeType: 'success',
    match: (r) => r?.action === 'hijack-dns' && (r?.port === 53 || (Array.isArray(r?.port) && r.port.includes(53))),
    build: () => ({ port: 53, action: 'hijack-dns' }),
  },
  {
    key: 'private-direct',
    name: i18n.global.t('rule.preset.privateName'),
    desc: i18n.global.t('rule.preset.privateDesc'),
    iconText: '🏠',
    color: '#10b981',
    match: (r) => r?.outbound === 'direct' && r?.ip_is_private === true,
    build: () => ({ ip_is_private: true, outbound: 'direct' }),
  },
  {
    key: 'cn-direct',
    name: i18n.global.t('rule.preset.cnName'),
    desc: i18n.global.t('rule.preset.cnDesc'),
    iconText: '🇨🇳',
    color: '#dc2626',
    badge: i18n.global.t('rule.preset.cnBadge'),
    badgeType: 'success',
    ruleSets: [{ tag: 'geosite-cn', url: `${SRS_GS}/geosite-cn.srs` }],
    match: (r) => r?.outbound === 'direct' && Array.isArray(r?.rule_set) && r.rule_set.includes('geosite-cn'),
    build: () => ({ rule_set: ['geosite-cn'], outbound: 'direct' }),
  },
  {
    key: 'cn-ip-direct',
    name: i18n.global.t('rule.preset.cnIpName'),
    desc: i18n.global.t('rule.preset.cnIpDesc'),
    iconText: '🌐',
    color: '#0ea5e9',
    ruleSets: [{ tag: 'geoip-cn', url: `${SRS_GI}/geoip-cn.srs` }],
    match: (r) => r?.outbound === 'direct' && Array.isArray(r?.rule_set) && r.rule_set.includes('geoip-cn'),
    build: () => ({ rule_set: ['geoip-cn'], outbound: 'direct' }),
  },
  {
    key: 'block-ads',
    name: i18n.global.t('rule.preset.blockAdName'),
    desc: i18n.global.t('rule.preset.blockAdDesc'),
    iconText: '🚫',
    color: '#475569',
    ruleSets: [{ tag: 'geosite-category-ads-all', url: `${SRS_GS}/geosite-category-ads-all.srs` }],
    match: (r) => r?.action === 'reject' && Array.isArray(r?.rule_set) && r.rule_set.includes('geosite-category-ads-all'),
    build: () => ({ rule_set: ['geosite-category-ads-all'], action: 'reject' }),
  },
  {
    key: 'block-tracker',
    name: i18n.global.t('rule.preset.blockTrackerName'),
    desc: i18n.global.t('rule.preset.blockTrackerDesc'),
    iconText: '⚠️',
    color: '#f59e0b',
    ruleSets: [{ tag: 'geosite-category-public-tracker', url: `${SRS_GS}/geosite-category-public-tracker.srs` }],
    match: (r) => r?.action === 'reject' && Array.isArray(r?.rule_set) && r.rule_set.includes('geosite-category-public-tracker'),
    build: () => ({ rule_set: ['geosite-category-public-tracker'], action: 'reject' }),
  },
]

const isPresetEnabled = (key: string) => {
  const p = routeRulePresets.find((x) => x.key === key)
  if (!p) return false
  return rules.value.some(p.match)
}

// 确保 outbounds 里有 direct 出站。sing-box 1.10+ 不再隐式提供 direct，
// 任何引用 outbound:'direct' 或 download_detour:'direct' 的配置都会启动失败。
// 这个函数被 rule_set 注册和 outbound:'direct' 规则共用。
const ensureDirectOutbound = async () => {
  const existing = (Data().outbounds as any[]) ?? []
  if (existing.some((o: any) => o.tag === 'direct' && o.type === 'direct')) return
  await Data().save('outbounds', 'new', { type: 'direct', tag: 'direct' })
}

const ensureRulesetRegistered = async (deps: { tag: string; url: string }[]) => {
  if (!route.value.rule_set) route.value.rule_set = []
  // download_detour 引用了 direct,所以先确保 direct 出站存在
  await ensureDirectOutbound()
  for (const d of deps) {
    if (!rulesets.value.some((rs: any) => rs.tag === d.tag)) {
      rulesets.value.push({
        tag: d.tag,
        type: 'remote',
        format: 'binary',
        url: d.url,
        download_detour: 'direct',
        update_interval: '24h',
      })
    }
  }
}

// 不带 autoSave 的纯插入,被批量入口(applyBestPractice / applyTemplate)复用,
// 末尾统一 autoSave 一次,免对每条规则单独 reload sing-box。
const enablePreset = async (p: RoutePreset): Promise<boolean> => {
  if (!route.value.rules) route.value.rules = []
  if (rules.value.findIndex(p.match) !== -1) return false
  const built = p.build()
  if (built?.outbound === 'direct') await ensureDirectOutbound()
  if (p.ruleSets?.length) await ensureRulesetRegistered(p.ruleSets)
  rules.value.push(built)
  return true
}

const togglePreset = async (p: RoutePreset, on: boolean) => {
  if (!route.value.rules) route.value.rules = []
  if (on) {
    await enablePreset(p)
  } else {
    const idx = rules.value.findIndex(p.match)
    if (idx >= 0) rules.value.splice(idx, 1)
  }
  await autoSave(on ? i18n.global.t('rule.msg.enabledSaved', { name: p.name }) : i18n.global.t('rule.msg.disabledSaved', { name: p.name }))
}

// 一键最佳实践：商业机场默认开这套(按推荐顺序加,末尾一次性 autoSave)
// 旧版本 fire-and-forget 调 togglePreset → 5 个 ensureDirectOutbound 同时
// race,可能产生重复 direct 出站或丢状态。这里改成串行 await。
const applyBestPractice = async () => {
  const order = ['hijack-dns', 'sniff', 'private-direct', 'cn-direct', 'block-ads']
  let added = 0
  for (const key of order) {
    const p = routeRulePresets.find((x) => x.key === key)
    if (!p) continue
    const ok = await enablePreset(p)
    if (ok) added++
  }
  if (added > 0) {
    await autoSave(i18n.global.t('rule.msg.bestPracticeApplied', { n: added }))
  } else {
    ElMessage.info(i18n.global.t('rule.msg.bestPracticeAllEnabled'))
  }
}

// ---------- 视觉辅助 ----------
const shortenUrl = (url: string): string => {
  try {
    const u = new URL(url)
    const path = u.pathname.split('/').filter(Boolean).slice(-2).join('/')
    return `${u.host}/.../${path}`
  } catch { return url }
}

const ruleActionLabel = (rule: any): string => {
  if (rule?.action === 'reject') return i18n.global.t('rule.actionLabel.reject')
  if (rule?.action === 'route' || rule?.outbound) return i18n.global.t('rule.actionLabel.route')
  if (rule?.action === 'sniff') return i18n.global.t('rule.actionLabel.sniff')
  if (rule?.action === 'hijack-dns') return i18n.global.t('rule.actionLabel.hijackDns')
  if (rule?.action === 'resolve') return i18n.global.t('rule.actionLabel.resolve')
  return rule?.action ?? i18n.global.t('rule.actionLabel.route')
}

const ruleActionClass = (rule: any): string => {
  const a = rule?.action
  if (a === 'reject') return 'tag--reject'
  if (a === 'sniff') return 'tag--sniff'
  if (a === 'hijack-dns') return 'tag--hijack'
  return ''
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

/* 操作类型彩色徽章 */
.tag--reject { color: #dc2626 !important; background: rgba(220, 38, 38, 0.08) !important; }
.tag--sniff { color: #7c3aed !important; background: rgba(124, 58, 237, 0.08) !important; }
.tag--hijack { color: #d97706 !important; background: rgba(217, 119, 6, 0.08) !important; }

/* 来源 / 规则集 URL 的截断 */
.ellipsis { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 180px; display: inline-block; }

/* 字段标签的 ? 提示图标 */
.label-tip { margin-left: 4px; color: var(--nc-text-muted); cursor: help; vertical-align: -2px; }

/* 空状态 */
.empty-state {
  padding: 24px;
  text-align: center;
  color: var(--nc-text-muted);
  font-size: 13px;
  line-height: 1.7;
  background: var(--nc-surface-soft, #f8fafc);
  border: 1px dashed var(--nc-border-soft);
  border-radius: var(--radius-md);
}
.empty-state b { color: var(--nc-text-1); font-weight: 600; }

/* 推荐预设卡片（与 DNS 页面统一） */
.preset-card { display: flex; flex-direction: column; gap: 14px; }
.preset-head { display: flex; align-items: baseline; gap: 12px; }
.preset-hint { font-size: 12px; color: var(--nc-text-muted); }
.preset-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 10px; }
.preset-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  background: var(--nc-surface, #fff);
  border: 1px solid var(--nc-border-soft);
  border-radius: var(--radius-md);
  transition: border-color 0.15s, box-shadow 0.15s;
}
.preset-item:hover { border-color: var(--nc-primary); box-shadow: 0 2px 8px rgba(59, 130, 246, 0.08); }
.preset-item__main { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.preset-item__title { display: flex; align-items: center; gap: 8px; }
.preset-item__icon {
  width: 24px;
  height: 24px;
  border-radius: 6px;
  color: #fff;
  font-size: 13px;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.preset-item__name { font-size: 13.5px; font-weight: 600; color: var(--nc-text-1); }
.preset-item__desc { font-size: 12px; color: var(--nc-text-muted); line-height: 1.5; }
</style>
