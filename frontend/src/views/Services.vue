<template>
  <div class="page-container">
    <div class="page-header with-actions">
      <div class="page-header-text">
        <h2 class="page-title">{{ $t('pages.services') }}</h2>
        <p class="page-desc">{{ $t('services.desc', '内置 DNS resolver / DERP / Cache 等服务') }}</p>
      </div>
      <div class="page-header-actions">
        <el-button type="primary" @click="showModal(0)">
          <el-icon><Plus /></el-icon>{{ $t('actions.add') }}
        </el-button>
      </div>
    </div>

    <div v-if="services.length === 0" class="empty-state nc-card">
      <el-icon class="empty-state__icon"><Box /></el-icon>
      <p class="empty-state__text">{{ $t('noData') }}</p>
    </div>

    <div v-else class="cards-grid">
      <div v-for="item in services" :key="item.id" class="entity-card nc-card">
        <div class="entity-card__head">
          <span class="entity-card__type">{{ item.type }}</span>
          <span class="entity-card__tag">{{ item.tag }}</span>
        </div>
        <dl class="entity-card__meta">
          <div class="entity-card__row">
            <dt>{{ $t('in.addr') }}</dt>
            <dd class="mono">{{ item.listen ?? '—' }}</dd>
          </div>
          <div class="entity-card__row">
            <dt>{{ $t('in.port') }}</dt>
            <dd class="mono">{{ item.listen_port ?? '—' }}</dd>
          </div>
          <div class="entity-card__row">
            <dt>{{ $t('objects.tls') }}</dt>
            <dd>
              <el-tag size="small" :type="item.tls_id > 0 ? 'success' : 'info'" effect="plain">
                {{ item.tls_id > 0 ? $t('enable') : $t('disable') }}
              </el-tag>
            </dd>
          </div>
        </dl>
        <div class="entity-card__actions">
          <el-tooltip :content="$t('actions.edit')" placement="top">
            <el-button text @click="showModal(item.id)"><el-icon><Edit /></el-icon></el-button>
          </el-tooltip>
          <el-popconfirm :title="$t('confirm')" :confirm-button-text="$t('yes')" :cancel-button-text="$t('no')" @confirm="delSrv(item.id)">
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

    <ServiceVue
      v-model="modal.visible"
      :visible="modal.visible"
      :id="modal.id"
      :data="modal.data"
      :inTags="inTags"
      :tsTags="tsTags"
      :ssTags="ssTags"
      :tlsConfigs="tlsConfigs"
      @close="closeModal"
    />
  </div>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import { Srv } from '@/types/services'
import { computed, defineAsyncComponent, ref } from 'vue'

const ServiceVue = defineAsyncComponent(() => import('@/layouts/modals/Service.vue'))
import { Plus, Edit, Delete, Box } from '@element-plus/icons-vue'

const services = computed((): Srv[] => <Srv[]>Data().services)

const tsTags = computed((): any[] =>
  Data().endpoints?.filter((o: any) => o.type == 'tailscale')?.map((o: any) => o.tag) ?? [])
const ssTags = computed((): any[] =>
  Data().inbounds?.filter((o: any) => o.type == 'shadowsocks' && !o.users)?.map((o: any) => o.tag) ?? [])
const inTags = computed((): any[] => [
  ...(Data().inbounds?.map((o: any) => o.tag).filter((t) => t != null) ?? []),
  ...(Data().endpoints?.filter((e: any) => e.listen_port > 0).map((e: any) => e.tag) ?? []),
])
const tlsConfigs = computed((): any[] => <any[]>Data().tlsConfigs)

const modal = ref({ visible: false, id: 0, data: '' })

const showModal = (id: number) => {
  modal.value.id = id
  modal.value.data = id == 0 ? '' : JSON.stringify(services.value.findLast((o: any) => o.id == id))
  modal.value.visible = true
}
const closeModal = () => { modal.value.visible = false }

const delSrv = async (id: number) => {
  const item = services.value.find((i: any) => i.id == id)
  if (item) await Data().save('services', 'del', item.tag)
}
</script>

<style scoped>
.cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 12px;
}
.entity-card { display: flex; flex-direction: column; gap: 10px; padding: 14px 16px 10px; }
.entity-card__head {
  display: flex; align-items: center; justify-content: space-between; gap: 8px;
  border-bottom: 1px solid var(--nc-border-soft); padding-bottom: 8px;
}
.entity-card__type {
  font-size: 11px; font-weight: 600; color: var(--nc-primary);
  background: var(--nc-primary-soft); padding: 2px 8px; border-radius: var(--radius-pill);
  text-transform: uppercase; letter-spacing: 0.04em;
}
.entity-card__tag {
  font-family: var(--font-display); font-size: 14px; font-weight: 600; color: var(--nc-text-1);
  flex: 1; text-align: right; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.entity-card__meta { margin: 0; display: flex; flex-direction: column; gap: 4px; }
.entity-card__row { display: flex; justify-content: space-between; align-items: center; gap: 8px; font-size: 12.5px; }
.entity-card__row dt { color: var(--nc-text-muted); }
.entity-card__row dd { margin: 0; color: var(--nc-text-1); font-weight: 500; }
.entity-card__row .mono { font-family: var(--font-mono); }
.entity-card__actions { display: flex; gap: 4px; border-top: 1px solid var(--nc-border-soft); padding-top: 4px; margin: 4px -4px -4px; }
.entity-card__actions .el-button { flex: 1; min-width: 0; height: 32px; margin: 0 !important; }
.empty-state {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 12px; padding: 48px 16px; text-align: center;
}
.empty-state__icon { font-size: 36px; color: var(--nc-text-faint); }
.empty-state__text { margin: 0; font-size: 13px; color: var(--nc-text-muted); }
</style>
