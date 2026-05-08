<template>
  <div class="page-container">
    <div class="page-header with-actions">
      <div class="page-header-text">
        <h2 class="page-title">{{ $t('pages.endpoints') }}</h2>
        <p class="page-desc">{{ $t('endpoints.desc', 'WireGuard / Tailscale / Warp 端点') }}</p>
      </div>
      <div class="page-header-actions">
        <el-button type="primary" @click="showModal(0)">
          <el-icon><Plus /></el-icon>{{ $t('actions.add') }}
        </el-button>
      </div>
    </div>

    <div v-if="endpoints.length === 0" class="empty-state nc-card">
      <el-icon class="empty-state__icon"><Box /></el-icon>
      <p class="empty-state__text">{{ $t('noData') }}</p>
    </div>

    <div v-else class="cards-grid">
      <div v-for="item in endpoints" :key="item.id" class="entity-card nc-card">
        <div class="entity-card__head">
          <span class="entity-card__type">{{ item.type }}</span>
          <span class="entity-card__tag">{{ item.tag }}</span>
        </div>
        <dl class="entity-card__meta">
          <div class="entity-card__row">
            <dt>{{ $t('in.addr') }}</dt>
            <dd class="mono">{{ item.address?.length > 0 ? item.address[0] : '—' }}</dd>
          </div>
          <div class="entity-card__row">
            <dt>{{ $t('in.port') }}</dt>
            <dd class="mono">{{ item.listen_port > 0 ? item.listen_port : '—' }}</dd>
          </div>
          <div class="entity-card__row">
            <dt>{{ $t('types.wg.peers') }}</dt>
            <dd class="mono">{{ item.peers?.length ?? '—' }}</dd>
          </div>
          <div class="entity-card__row">
            <dt>{{ $t('online') }}</dt>
            <dd>
              <span v-if="onlines.includes(item.tag)" class="status-pill"><span class="status-dot online"></span>{{ $t('online') }}</span>
              <span v-else>—</span>
            </dd>
          </div>
        </dl>
        <div class="entity-card__actions">
          <el-tooltip :content="$t('actions.edit')" placement="top">
            <el-button text @click="showModal(item.id)"><el-icon><Edit /></el-icon></el-button>
          </el-tooltip>
          <el-popconfirm :title="$t('confirm')" :confirm-button-text="$t('yes')" :cancel-button-text="$t('no')" @confirm="delEndpoint(item.tag)">
            <template #reference>
              <el-button text>
                <el-tooltip :content="$t('actions.del')" placement="top">
                  <el-icon><Delete /></el-icon>
                </el-tooltip>
              </el-button>
            </template>
          </el-popconfirm>
          <el-tooltip v-if="item.type == 'wireguard' && item.peers?.length > 0" :content="$t('main.qr', 'QR')" placement="top">
            <el-button text @click="showQrCode(item.id)"><el-icon><Picture /></el-icon></el-button>
          </el-tooltip>
          <el-tooltip v-if="Data().enableTraffic" :content="$t('stats.graphTitle')" placement="top">
            <el-button text @click="showStats(item.tag)"><el-icon><DataLine /></el-icon></el-button>
          </el-tooltip>
        </div>
      </div>
    </div>

    <EndpointVue
      v-model="modal.visible"
      :visible="modal.visible"
      :id="modal.id"
      :data="modal.data"
      :tags="endpointTags"
      @close="closeModal"
    />
    <Stats v-model="stats.visible" :visible="stats.visible" :resource="stats.resource" :tag="stats.tag" @close="closeStats" />
    <WgQrCode v-model="qrcode.visible" :visible="qrcode.visible" :data="qrcode.data" @close="closeQrCode" />
  </div>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import { Endpoint } from '@/types/endpoints'
import { computed, defineAsyncComponent, ref } from 'vue'

const EndpointVue = defineAsyncComponent(() => import('@/layouts/modals/Endpoint.vue'))
const Stats = defineAsyncComponent(() => import('@/layouts/modals/Stats.vue'))
const WgQrCode = defineAsyncComponent(() => import('@/layouts/modals/WgQrCode.vue'))
import { Plus, Edit, Delete, DataLine, Picture, Box } from '@element-plus/icons-vue'

const endpoints = computed((): Endpoint[] => <Endpoint[]>Data().endpoints)
const endpointTags = computed((): any[] => endpoints.value?.map((o: Endpoint) => o.tag) ?? [])
const onlines = computed(() => [...(Data().onlines.inbound ?? []), ...(Data().onlines.outbound ?? [])])

const modal = ref({ visible: false, id: 0, data: '' })
const showModal = (id: number) => {
  modal.value.id = id
  modal.value.data = id == 0 ? '' : JSON.stringify(endpoints.value.findLast((o: any) => o.id == id))
  modal.value.visible = true
}
const closeModal = () => { modal.value.visible = false }

const stats = ref({ visible: false, resource: 'endpoint', tag: '' })
const delEndpoint = async (tag: string) => { await Data().save('endpoints', 'del', tag) }
const showStats = (tag: string) => { stats.value.tag = tag; stats.value.visible = true }
const closeStats = () => { stats.value.visible = false }

const qrcode = ref({ visible: false, data: <any>{} })
const showQrCode = (id: number) => {
  qrcode.value.data = endpoints.value.findLast((o: any) => o.id == id)
  qrcode.value.visible = true
}
const closeQrCode = () => { qrcode.value.visible = false }
</script>

<style scoped>
.cards-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 12px; }
.entity-card { display: flex; flex-direction: column; gap: 10px; padding: 14px 16px 10px; }
.entity-card__head { display: flex; align-items: center; justify-content: space-between; gap: 8px; border-bottom: 1px solid var(--nc-border-soft); padding-bottom: 8px; }
.entity-card__type { font-size: 11px; font-weight: 600; color: var(--nc-primary); background: var(--nc-primary-soft); padding: 2px 8px; border-radius: var(--radius-pill); text-transform: uppercase; letter-spacing: 0.04em; }
.entity-card__tag { font-family: var(--font-display); font-size: 14px; font-weight: 600; color: var(--nc-text-1); flex: 1; text-align: right; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.entity-card__meta { margin: 0; display: flex; flex-direction: column; gap: 4px; }
.entity-card__row { display: flex; justify-content: space-between; align-items: center; gap: 8px; font-size: 12.5px; }
.entity-card__row dt { color: var(--nc-text-muted); }
.entity-card__row dd { margin: 0; color: var(--nc-text-1); font-weight: 500; }
.entity-card__row .mono { font-family: var(--font-mono); }
.entity-card__actions { display: flex; gap: 4px; border-top: 1px solid var(--nc-border-soft); padding-top: 4px; margin: 4px -4px -4px; }
.entity-card__actions .el-button { flex: 1; min-width: 0; height: 32px; margin: 0 !important; }
.status-pill { display: inline-flex; align-items: center; gap: 4px; font-size: 11.5px; color: var(--nc-success); font-weight: 500; }
.empty-state { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; padding: 48px 16px; text-align: center; }
.empty-state__icon { font-size: 36px; color: var(--nc-text-faint); }
.empty-state__text { margin: 0; font-size: 13px; color: var(--nc-text-muted); }
</style>
