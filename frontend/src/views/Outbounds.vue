<template>
  <div class="page-container">
    <div class="page-header with-actions">
      <div class="page-header-text">
        <h2 class="page-title">{{ $t('pages.outbounds') }}</h2>
        <p class="page-desc">{{ $t('outbounds.desc', '配置出站协议、批量导入与连通性测试') }}</p>
      </div>
      <div class="page-header-actions">
        <el-button type="primary" @click="showModal(0)">
          <el-icon><Plus /></el-icon>{{ $t('actions.add') }}
        </el-button>
        <el-button @click="showBulkModal">
          <el-icon><DocumentAdd /></el-icon>{{ $t('actions.addbulk') }}
        </el-button>
        <el-button :loading="testingAll" :disabled="outbounds.length === 0" @click="checkAllOutbounds">
          <el-icon><Stopwatch /></el-icon>{{ $t('actions.testAll', 'Test all') }}
        </el-button>
      </div>
    </div>

    <div v-if="outbounds.length === 0" class="empty-state nc-card">
      <el-icon class="empty-state__icon"><Box /></el-icon>
      <p class="empty-state__text">{{ $t('noData') }}</p>
    </div>

    <div v-else class="cards-grid">
      <div v-for="item in outbounds" :key="item.id" class="entity-card nc-card">
        <div class="entity-card__head">
          <span class="entity-card__type">{{ item.type }}</span>
          <span class="entity-card__tag">{{ item.tag }}</span>
        </div>

        <dl class="entity-card__meta">
          <div class="entity-card__row">
            <dt>{{ $t('in.addr') }}</dt>
            <dd class="mono">{{ item.server ?? '—' }}</dd>
          </div>
          <div class="entity-card__row">
            <dt>{{ $t('in.port') }}</dt>
            <dd class="mono">{{ item.server_port ?? '—' }}</dd>
          </div>
          <div class="entity-card__row">
            <dt>{{ $t('objects.tls') }}</dt>
            <dd>
              <el-tag
                v-if="Object.hasOwn(item, 'tls')"
                :type="item.tls?.enabled ? 'success' : 'info'"
                size="small"
                effect="plain"
              >
                {{ $t(item.tls?.enabled ? 'enable' : 'disable') }}
              </el-tag>
              <span v-else>—</span>
            </dd>
          </div>
          <div class="entity-card__row">
            <dt>{{ $t('online') }}</dt>
            <dd>
              <span v-if="onlines.includes(item.tag)" class="status-pill status-pill--ok">
                <span class="status-dot online"></span>{{ $t('online') }}
              </span>
              <span v-else>—</span>
            </dd>
          </div>
          <div class="entity-card__row">
            <dt>{{ $t('out.delay') }}</dt>
            <dd>
              <el-button
                v-if="!checkResults[item.tag]?.loading"
                text
                size="small"
                @click="checkOutbound(item.tag)"
              >
                <el-icon><Stopwatch /></el-icon>
              </el-button>
              <el-icon v-else class="is-loading"><Loading /></el-icon>
              <template v-if="checkResults[item.tag] && !checkResults[item.tag]?.loading">
                <el-tag
                  v-if="checkResults[item.tag].success"
                  type="success"
                  size="small"
                  effect="plain"
                >
                  {{ checkResults[item.tag].data?.Delay }}{{ $t('date.ms') }}
                </el-tag>
                <el-tooltip
                  v-else
                  :content="checkResults[item.tag].errorMessage || $t('failed')"
                  placement="top"
                >
                  <el-icon style="color: var(--nc-danger)"><CircleClose /></el-icon>
                </el-tooltip>
              </template>
            </dd>
          </div>
        </dl>

        <div class="entity-card__actions">
          <el-tooltip :content="$t('actions.edit')" placement="top">
            <el-button text @click="showModal(item.id)">
              <el-icon><Edit /></el-icon>
            </el-button>
          </el-tooltip>
          <el-popconfirm
            :title="$t('confirm')"
            :confirm-button-text="$t('yes')"
            :cancel-button-text="$t('no')"
            @confirm="delOutbound(item.tag)"
          >
            <template #reference>
              <el-button text>
                <el-tooltip :content="$t('actions.del')" placement="top">
                  <el-icon><Delete /></el-icon>
                </el-tooltip>
              </el-button>
            </template>
          </el-popconfirm>
          <el-tooltip v-if="Data().enableTraffic" :content="$t('stats.graphTitle')" placement="top">
            <el-button text @click="showStats(item.tag)">
              <el-icon><DataLine /></el-icon>
            </el-button>
          </el-tooltip>
        </div>
      </div>
    </div>

    <OutboundVue
      v-model="modal.visible"
      :visible="modal.visible"
      :id="modal.id"
      :data="modal.data"
      :tags="outboundTags"
      @close="closeModal"
    />
    <OutboundBulk
      v-model="bulkModal.visible"
      :visible="bulkModal.visible"
      :outboundTags="outboundTags"
      @close="closeBulkModal"
    />
    <Stats
      v-model="stats.visible"
      :visible="stats.visible"
      :resource="stats.resource"
      :tag="stats.tag"
      @close="closeStats"
    />
  </div>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import HttpUtils from '@/plugins/httputil'
import { Outbound } from '@/types/outbounds'
import { computed, defineAsyncComponent, ref } from 'vue'

const OutboundVue = defineAsyncComponent(() => import('@/layouts/modals/Outbound.vue'))
const OutboundBulk = defineAsyncComponent(() => import('@/layouts/modals/OutboundBulk.vue'))
const Stats = defineAsyncComponent(() => import('@/layouts/modals/Stats.vue'))
import {
  Plus, Edit, Delete, DataLine, DocumentAdd, Stopwatch, Loading, CircleClose, Box,
} from '@element-plus/icons-vue'

interface CheckResult {
  loading?: boolean
  success: boolean
  data?: { OK?: boolean; Delay?: number; Error?: string } | null
  errorMessage?: string
}

const checkResults = ref<Record<string, CheckResult>>({})

const checkOutbound = async (tag: string) => {
  checkResults.value = { ...checkResults.value, [tag]: { loading: true, success: false } }
  const msg = await HttpUtils.get('api/checkOutbound', { tag })
  const success = msg.success && msg.obj?.OK
  const errorMessage = success ? undefined : (msg.obj?.Error ?? msg.msg ?? '')
  checkResults.value = {
    ...checkResults.value,
    [tag]: { loading: false, success, data: msg.obj ?? null, errorMessage },
  }
}

const testingAll = ref(false)
const checkAllOutbounds = async () => {
  const list = outbounds.value
  if (list.length === 0) return
  testingAll.value = true
  try {
    await Promise.all(list.map((o) => checkOutbound(o.tag)))
  } finally {
    testingAll.value = false
  }
}

const outbounds = computed((): Outbound[] => <Outbound[]>Data().outbounds)
const outboundTags = computed((): string[] => [
  ...(Data().outbounds?.map((o: Outbound) => o.tag) ?? []),
  ...(Data().endpoints?.map((e: any) => e.tag) ?? []),
])
const onlines = computed(() => Data().onlines.outbound ?? [])

const modal = ref({ visible: false, id: 0, data: '' })
const showModal = (id: number) => {
  modal.value.id = id
  modal.value.data = id == 0 ? '' : JSON.stringify(outbounds.value.findLast((o) => o.id == id))
  modal.value.visible = true
}
const closeModal = () => { modal.value.visible = false }

const bulkModal = ref({ visible: false })
const showBulkModal = () => { bulkModal.value.visible = true }
const closeBulkModal = () => { bulkModal.value.visible = false }

const stats = ref({ visible: false, resource: 'outbound', tag: '' })
const delOutbound = async (tag: string) => {
  await Data().save('outbounds', 'del', tag)
}
const showStats = (tag: string) => {
  stats.value.tag = tag
  stats.value.visible = true
}
const closeStats = () => { stats.value.visible = false }
</script>

<style scoped>
.cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 12px;
}

.entity-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 16px 10px;
}

.entity-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  border-bottom: 1px solid var(--nc-border-soft);
  padding-bottom: 8px;
}

.entity-card__type {
  font-size: 11px;
  font-weight: 600;
  color: var(--nc-primary);
  background: var(--nc-primary-soft);
  padding: 2px 8px;
  border-radius: var(--radius-pill);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.entity-card__tag {
  font-family: var(--font-display);
  font-size: 14px;
  font-weight: 600;
  color: var(--nc-text-1);
  flex: 1;
  text-align: right;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.entity-card__meta {
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.entity-card__row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  font-size: 12.5px;
}

.entity-card__row dt {
  color: var(--nc-text-muted);
}

.entity-card__row dd {
  margin: 0;
  color: var(--nc-text-1);
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: 6px;
}

.entity-card__row .mono {
  font-family: var(--font-mono);
}

.entity-card__actions {
  display: flex;
  align-items: center;
  gap: 4px;
  border-top: 1px solid var(--nc-border-soft);
  padding-top: 4px;
  margin: 4px -4px -4px;
}

.entity-card__actions .el-button {
  flex: 1;
  min-width: 0;
  height: 32px;
  margin: 0 !important;
}

.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11.5px;
  color: var(--nc-success);
  font-weight: 500;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 48px 16px;
  text-align: center;
}

.empty-state__icon {
  font-size: 36px;
  color: var(--nc-text-faint);
}

.empty-state__text {
  margin: 0;
  font-size: 13px;
  color: var(--nc-text-muted);
}
</style>
