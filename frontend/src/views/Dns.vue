<template>
  <div class="page-container">
    <div class="page-header with-actions">
      <div class="page-header-text">
        <h2 class="page-title">{{ $t('pages.dns') }}</h2>
        <p class="page-desc">{{ $t('dns.desc', 'DNS 服务器、规则与策略') }}</p>
      </div>
      <div class="page-header-actions">
        <el-button type="primary" @click="showDnsModal(-1)">
          <el-icon><Plus /></el-icon>{{ $t('dns.add') }}
        </el-button>
        <el-button @click="showDnsRuleModal(-1)">
          <el-icon><Plus /></el-icon>{{ $t('dns.rule.add') }}
        </el-button>
        <el-button type="warning" plain :loading="loading" :disabled="stateChange" @click="saveConfig">
          <el-icon><Check /></el-icon>{{ $t('actions.save') }}
        </el-button>
      </div>
    </div>

    <div class="nc-card">
      <h4 class="section-title">{{ $t('pages.basics') }}</h4>
      <el-form label-position="top">
        <div class="form-grid">
          <el-form-item :label="$t('dns.final')">
            <el-select v-model="finalDns">
              <el-option :label="$t('dns.firstServer')" value="" />
              <el-option v-for="t in dnsServerTags" :key="t" :label="t" :value="t" />
            </el-select>
          </el-form-item>
          <el-form-item :label="$t('dns.domainStrategy')">
            <el-select v-model="dns.strategy" clearable>
              <el-option v-for="s in ['prefer_ipv4', 'prefer_ipv6', 'ipv4_only', 'ipv6_only']" :key="s" :label="s" :value="s" />
            </el-select>
          </el-form-item>
          <el-form-item :label="$t('dns.rule.action.clientSubnet')">
            <el-input v-model="dns.client_subnet" clearable />
          </el-form-item>
          <el-form-item :label="$t('dns.cacheCapacity')">
            <el-input-number v-model="dns.cache_capacity" :min="0" controls-position="right" style="width: 100%" />
          </el-form-item>
          <el-form-item :label="$t('dns.disableCache')"><el-switch v-model="dns.disable_cache" /></el-form-item>
          <el-form-item :label="$t('dns.disableExpire')"><el-switch v-model="dns.disable_expire" /></el-form-item>
          <el-form-item :label="$t('dns.independentCache')"><el-switch v-model="dns.independent_cache" /></el-form-item>
          <el-form-item :label="$t('dns.reverseMapping')"><el-switch v-model="dns.reverse_mapping" /></el-form-item>
        </div>
      </el-form>
    </div>

    <div>
      <div class="nc-divider"><span>{{ $t('dns.title') }} ({{ dns.servers?.length ?? 0 }})</span></div>
      <div class="cards-grid">
        <div v-for="(item, index) in (dns.servers as any[])" :key="index" class="entity-card nc-card">
          <div class="entity-card__head">
            <span class="entity-card__type">{{ item.type }}</span>
            <span class="entity-card__tag">{{ item.tag }}</span>
          </div>
          <dl class="entity-card__meta">
            <div class="entity-card__row"><dt>{{ $t('dns.server') }}</dt><dd class="mono">{{ item.server ?? '—' }}</dd></div>
            <div class="entity-card__row"><dt>{{ $t('in.port') }}</dt><dd class="mono">{{ item.server_port ?? '—' }}</dd></div>
            <div class="entity-card__row">
              <dt>{{ $t('objects.tls') }}</dt>
              <dd>
                <el-tag v-if="Object.hasOwn(item, 'tls')" size="small" :type="item.tls?.enabled ? 'success' : 'info'" effect="plain">
                  {{ $t(item.tls?.enabled ? 'enable' : 'disable') }}
                </el-tag>
                <span v-else>—</span>
              </dd>
            </div>
          </dl>
          <div class="entity-card__actions">
            <el-tooltip :content="$t('actions.edit')" placement="top">
              <el-button text @click="showDnsModal(Number(index))"><el-icon><Edit /></el-icon></el-button>
            </el-tooltip>
            <el-popconfirm :title="$t('confirm')" :confirm-button-text="$t('yes')" :cancel-button-text="$t('no')" @confirm="delDns(Number(index))">
              <template #reference>
                <el-button text><el-tooltip :content="$t('actions.del')" placement="top"><el-icon><Delete /></el-icon></el-tooltip></el-button>
              </template>
            </el-popconfirm>
          </div>
        </div>
      </div>
    </div>

    <div>
      <div class="nc-divider"><span>{{ $t('dns.rule.title') }} ({{ dnsRules.length }})</span></div>
      <div class="cards-grid">
        <div
          v-for="(item, index) in (dnsRules as any[])"
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
            <div class="entity-card__row"><dt>{{ $t('dns.server') }}</dt><dd>{{ item.server ?? '—' }}</dd></div>
            <div class="entity-card__row"><dt>{{ $t('pages.rules') }}</dt><dd class="mono">{{ item.rules ? item.rules.length : Object.keys(item).filter((r: string) => !actionDnsRuleKeys.includes(r)).length }}</dd></div>
            <div class="entity-card__row"><dt>{{ $t('rule.invert') }}</dt><dd>{{ $t(item.invert ? 'yes' : 'no') }}</dd></div>
          </dl>
          <div class="entity-card__actions">
            <el-tooltip :content="$t('actions.edit')" placement="top">
              <el-button text @click="showDnsRuleModal(Number(index))"><el-icon><Edit /></el-icon></el-button>
            </el-tooltip>
            <el-popconfirm :title="$t('confirm')" :confirm-button-text="$t('yes')" :cancel-button-text="$t('no')" @confirm="delDnsRule(Number(index))">
              <template #reference>
                <el-button text><el-tooltip :content="$t('actions.del')" placement="top"><el-icon><Delete /></el-icon></el-tooltip></el-button>
              </template>
            </el-popconfirm>
          </div>
        </div>
      </div>
    </div>

    <DnsVue
      v-model="dnsModal.visible"
      :visible="dnsModal.visible"
      :index="dnsModal.index"
      :data="dnsModal.data"
      :tsTags="tsTags"
      :rslvdTags="rslvdTags"
      @close="closeDnsModal"
      @save="saveDnsModal"
    />
    <DnsRuleVue
      v-model="dnsRuleModal.visible"
      :visible="dnsRuleModal.visible"
      :index="dnsRuleModal.index"
      :data="dnsRuleModal.data"
      :clients="clients"
      :inTags="inboundTags"
      :serverTags="dnsServerTags"
      :ruleSets="ruleSets"
      @close="closeDnsRuleModal"
      @save="saveDnsRuleModal"
    />
  </div>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import { computed, ref, onBeforeMount, defineAsyncComponent } from 'vue'

const DnsVue = defineAsyncComponent(() => import('@/layouts/modals/Dns.vue'))
const DnsRuleVue = defineAsyncComponent(() => import('@/layouts/modals/DnsRule.vue'))
import { Config } from '@/types/config'
import { actionDnsRuleKeys, dnsRule } from '@/types/dns'
import { FindDiff } from '@/plugins/utils'
import { Plus, Edit, Delete, Check } from '@element-plus/icons-vue'

const oldConfig = ref<any>({})
const loading = ref(false)
const appConfig = computed((): Config => <Config>Data().config)

onBeforeMount(async () => {
  if (!appConfig.value.dns) appConfig.value.dns = { servers: [], rules: [] }
  if (!appConfig.value.dns.servers) appConfig.value.dns.servers = []
  if (!appConfig.value.dns.rules) appConfig.value.dns.rules = []

  loading.value = true
  while (Data().lastLoad === 0) await new Promise((r) => setTimeout(r, 100))
  oldConfig.value = JSON.parse(JSON.stringify(Data().config))
  loading.value = false
})

const tsTags = computed(() => Data().endpoints?.filter((e: any) => e.type === 'tailscale').map((e: any) => e.tag) ?? [])
const rslvdTags = computed(() => Data().services?.filter((e: any) => e.type === 'resolved').map((e: any) => e.tag) ?? [])
const clients = computed(() => Data().clients?.map((c: any) => c.name) ?? [])
const stateChange = computed(() => FindDiff.deepCompare(appConfig.value.dns, oldConfig.value.dns))

const saveConfig = async () => {
  loading.value = true
  const success = await Data().save('config', 'set', appConfig.value)
  if (success) oldConfig.value = JSON.parse(JSON.stringify(Data().config))
  loading.value = false
}

const inboundTags = computed(() => [
  ...(Data().inbounds?.map((o: any) => o.tag) ?? []),
  ...(Data().endpoints?.filter((e: any) => e.listen_port > 0).map((e: any) => e.tag) ?? []),
])
const dns = computed((): any => appConfig.value.dns)
const dnsServerTags = computed<string[]>(() => dns.value?.servers?.filter((s: any) => s.tag).map((s: any) => s.tag) ?? [])
const finalDns = computed({
  get: () => dns.value?.final ?? '',
  set: (v: string) => { dns.value.final = v.length > 0 ? v : undefined },
})
const dnsRules = computed((): dnsRule[] => <dnsRule[]>(dns.value.rules ?? []))
const ruleSets = computed(() => appConfig.value?.route?.rule_set?.map((r: any) => r.tag) ?? [])

const dnsModal = ref({ visible: false, index: -1, data: '' })
const showDnsModal = (index: number) => {
  dnsModal.value.index = index
  dnsModal.value.data = index === -1 ? '' : JSON.stringify(dns.value.servers[index])
  dnsModal.value.visible = true
}
const closeDnsModal = () => { dnsModal.value.visible = false }
const saveDnsModal = (data: any) => {
  if (dnsModal.value.index === -1) dns.value.servers.push(data)
  else dns.value.servers[dnsModal.value.index] = data
  dnsModal.value.visible = false
}
const delDns = (index: number) => { dns.value.servers.splice(index, 1) }

const dnsRuleModal = ref({ visible: false, index: -1, data: '' })
const showDnsRuleModal = (index: number) => {
  dnsRuleModal.value.index = index
  dnsRuleModal.value.data = index === -1 ? '' : JSON.stringify(dnsRules.value[index])
  dnsRuleModal.value.visible = true
}
const closeDnsRuleModal = () => { dnsRuleModal.value.visible = false }
const saveDnsRuleModal = (data: dnsRule) => {
  if (dnsRuleModal.value.index === -1) dnsRules.value.push(data)
  else dnsRules.value[dnsRuleModal.value.index] = data
  dnsRuleModal.value.visible = false
}
const delDnsRule = (index: number) => { dnsRules.value.splice(index, 1) }

const draggedItemIndex = ref<number | null>(null)
const onDragStart = (index: number) => { draggedItemIndex.value = index }
const onDrop = (index: number) => {
  if (draggedItemIndex.value !== null) {
    const dragged = dnsRules.value[draggedItemIndex.value]
    dnsRules.value.splice(draggedItemIndex.value, 1)
    dnsRules.value.splice(index, 0, dragged)
    draggedItemIndex.value = null
  }
}
</script>

<style scoped>
.section-title { font-size: 12px; font-weight: 600; color: var(--nc-text-muted); text-transform: uppercase; letter-spacing: 0.06em; margin-bottom: 12px; }
.form-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 6px 16px; }
.cards-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 12px; }
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
