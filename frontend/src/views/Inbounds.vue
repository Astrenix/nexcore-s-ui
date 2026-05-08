<template>
  <div class="page-container">
    <div class="page-header with-actions">
      <div class="page-header-text">
        <h2 class="page-title">{{ $t('pages.inbounds') }}</h2>
        <p class="page-desc">{{ $t('inbounds.desc', '配置 Sing-Box 入站协议、监听端口与关联用户') }}</p>
      </div>
      <div class="page-header-actions">
        <el-button type="primary" @click="showModal(0)">
          <el-icon><Plus /></el-icon>{{ $t('actions.add') }}
        </el-button>
      </div>
    </div>

    <div v-if="inbounds.length === 0" class="empty-state nc-card">
      <el-icon class="empty-state__icon"><Box /></el-icon>
      <p class="empty-state__text">{{ $t('noData') }}</p>
    </div>

    <div v-else class="cards-grid">
      <div v-for="item in inbounds" :key="item.id" class="entity-card nc-card">
        <div class="entity-card__head">
          <span class="entity-card__type">{{ item.type }}</span>
          <span class="entity-card__tag">{{ item.tag }}</span>
        </div>

        <dl class="entity-card__meta">
          <div class="entity-card__row">
            <dt>{{ $t('in.addr') }}</dt>
            <dd class="mono">{{ item.listen || '0.0.0.0' }}</dd>
          </div>
          <div class="entity-card__row">
            <dt>{{ $t('in.port') }}</dt>
            <dd class="mono">{{ item.listen_port }}</dd>
          </div>
          <div class="entity-card__row">
            <dt>{{ $t('objects.tls') }}</dt>
            <dd>
              <el-tag :type="item.tls_id > 0 ? 'success' : 'info'" size="small" effect="plain">
                {{ item.tls_id > 0 ? $t('enable') : $t('disable') }}
              </el-tag>
            </dd>
          </div>
          <div class="entity-card__row">
            <dt>{{ $t('pages.clients') }}</dt>
            <dd>
              <el-tooltip v-if="(item as any).users?.length" :content="((item as any).users || []).join('、')" placement="bottom">
                <span class="mono">{{ (item as any).users.length }}</span>
              </el-tooltip>
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
            @confirm="delInbound(item.id)"
          >
            <template #reference>
              <el-button text>
                <el-tooltip :content="$t('actions.del')" placement="top">
                  <el-icon><Delete /></el-icon>
                </el-tooltip>
              </el-button>
            </template>
          </el-popconfirm>
          <el-tooltip :content="$t('actions.clone')" placement="top">
            <el-button text :loading="cloneLoading" @click="clone(item.id)">
              <el-icon><CopyDocument /></el-icon>
            </el-button>
          </el-tooltip>
          <el-tooltip v-if="Data().enableTraffic" :content="$t('stats.graphTitle')" placement="top">
            <el-button text @click="showStats(item.tag)">
              <el-icon><DataLine /></el-icon>
            </el-button>
          </el-tooltip>
        </div>
      </div>
    </div>

    <InboundVue
      v-model="modal.visible"
      :visible="modal.visible"
      :id="modal.id"
      :inTags="inTags"
      :tlsConfigs="tlsConfigs"
      @close="closeModal"
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
import { Config } from '@/types/config'
import { computed, defineAsyncComponent, ref } from 'vue'

const InboundVue = defineAsyncComponent(() => import('@/layouts/modals/Inbound.vue'))
const Stats = defineAsyncComponent(() => import('@/layouts/modals/Stats.vue'))
import { createInbound, Inbound } from '@/types/inbounds'
import RandomUtil from '@/plugins/randomUtil'
import { Plus, Edit, Delete, CopyDocument, DataLine, Box } from '@element-plus/icons-vue'

const appConfig = computed((): Config => <Config>Data().config)
void appConfig

const inbounds = computed((): Inbound[] => <Inbound[]>Data().inbounds)
const tlsConfigs = computed((): any[] => <any[]>Data().tlsConfigs)

const inTags = computed((): string[] => [
  ...(inbounds.value?.map((i) => i.tag) ?? []),
  ...(Data().endpoints?.filter((e: any) => e.listen_port > 0).map((e: any) => e.tag) ?? []),
])

const onlines = computed(() => Data().onlines.inbound ?? [])

const modal = ref({ visible: false, id: 0 })

const showModal = (id: number) => {
  modal.value.id = id
  modal.value.visible = true
}
const closeModal = () => {
  modal.value.visible = false
}

const delInbound = async (id: number) => {
  const inbound = inbounds.value.find((i) => i.id == id)
  if (inbound) await Data().save('inbounds', 'del', inbound.tag)
}

const cloneLoading = ref(false)

const clone = async (id: number) => {
  cloneLoading.value = true
  try {
    const inboundArray = await Data().loadInbounds([id])
    const inbound = inboundArray[0]
    const newTag = inbound.type + '-' + RandomUtil.randomSeq(3)
    const newInbound = createInbound(inbound.type, {
      ...inbound,
      id: 0,
      tag: newTag,
      listen_port: RandomUtil.randomIntRange(10000, 60000),
    })
    await Data().save('inbounds', 'new', newInbound)
  } finally {
    cloneLoading.value = false
  }
}

const stats = ref({ visible: false, resource: 'inbound', tag: '' })
const showStats = (tag: string) => {
  stats.value.tag = tag
  stats.value.visible = true
}
const closeStats = () => {
  stats.value.visible = false
}
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
