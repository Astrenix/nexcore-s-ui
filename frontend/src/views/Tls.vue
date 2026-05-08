<template>
  <div class="page-container">
    <div class="page-header with-actions">
      <div class="page-header-text">
        <h2 class="page-title">{{ $t('pages.tls') }}</h2>
        <p class="page-desc">{{ $t('tls.desc', 'TLS / Reality / ECH / ACME 证书集中管理') }}</p>
      </div>
      <div class="page-header-actions">
        <el-button type="primary" @click="showModal(0)">
          <el-icon><Plus /></el-icon>{{ $t('actions.add') }}
        </el-button>
      </div>
    </div>

    <div v-if="tlsConfigs.length === 0" class="empty-state nc-card">
      <el-icon class="empty-state__icon"><Box /></el-icon>
      <p class="empty-state__text">{{ $t('noData') }}</p>
    </div>

    <div v-else class="cards-grid">
      <div v-for="item in tlsConfigs" :key="item.id" class="entity-card nc-card">
        <div class="entity-card__head">
          <span class="entity-card__type">TLS</span>
          <span class="entity-card__tag">{{ item.name }}</span>
        </div>
        <p class="entity-card__sub mono">{{ item.server?.server_name?.length > 0 ? item.server.server_name : '—' }}</p>
        <dl class="entity-card__meta">
          <div class="entity-card__row">
            <dt>{{ $t('pages.inbounds') }}</dt>
            <dd>
              <el-tooltip v-if="tlsInbounds(item.id).length" :content="tlsInbounds(item.id).join('、')" placement="top">
                <span class="mono">{{ tlsInbounds(item.id).length }}</span>
              </el-tooltip>
              <span v-else>—</span>
            </dd>
          </div>
          <div class="entity-card__row">
            <dt>ACME</dt>
            <dd><el-tag size="small" :type="item.server?.acme ? 'success' : 'info'" effect="plain">{{ $t(item.server?.acme ? 'yes' : 'no') }}</el-tag></dd>
          </div>
          <div class="entity-card__row">
            <dt>ECH</dt>
            <dd><el-tag size="small" :type="item.server?.ech ? 'success' : 'info'" effect="plain">{{ $t(item.server?.ech ? 'yes' : 'no') }}</el-tag></dd>
          </div>
          <div class="entity-card__row">
            <dt>Reality</dt>
            <dd><el-tag size="small" :type="item.server?.reality ? 'success' : 'info'" effect="plain">{{ $t(item.server?.reality ? 'yes' : 'no') }}</el-tag></dd>
          </div>
        </dl>
        <div class="entity-card__actions">
          <el-tooltip :content="$t('actions.edit')" placement="top">
            <el-button text @click="showModal(item.id)"><el-icon><Edit /></el-icon></el-button>
          </el-tooltip>
          <el-popconfirm
            v-if="tlsInbounds(item.id).length === 0"
            :title="$t('confirm')"
            :confirm-button-text="$t('yes')"
            :cancel-button-text="$t('no')"
            @confirm="delTls(item.id)"
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
            <el-button text @click="clone(item)"><el-icon><CopyDocument /></el-icon></el-button>
          </el-tooltip>
        </div>
      </div>
    </div>

    <TlsVue
      v-model="modal.visible"
      :visible="modal.visible"
      :id="modal.id"
      :data="modal.data"
      @close="closeModal"
      @save="saveModal"
    />
  </div>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import { computed, defineAsyncComponent, ref } from 'vue'
import { Inbound } from '@/types/inbounds'
import { tls } from '@/types/tls'

const TlsVue = defineAsyncComponent(() => import('@/layouts/modals/Tls.vue'))
import { Plus, Edit, Delete, CopyDocument, Box } from '@element-plus/icons-vue'

const tlsConfigs = computed((): any[] => Data().tlsConfigs ?? [])
const inbounds = computed((): Inbound[] => Data().inbounds ?? [])
const tlsInbounds = (id: number): string[] => inbounds.value.filter((i) => i.tls_id == id).map((i) => i.tag)

const modal = ref({ visible: false, id: 0, data: '' })

const showModal = (id: number) => {
  modal.value.id = id
  modal.value.data = id == 0 ? '{}' : JSON.stringify(tlsConfigs.value.findLast((t) => t.id == id))
  modal.value.visible = true
}
const clone = (obj: any) => {
  const data = JSON.parse(JSON.stringify(obj))
  data.id = 0
  while (tlsConfigs.value.findIndex((t) => t.name === data.name) !== -1) data.name += '-copy'
  saveModal(data)
}
const closeModal = () => { modal.value.visible = false }
const saveModal = async (data: tls) => {
  const success = await Data().save('tls', data.id > 0 ? 'edit' : 'new', data)
  if (success) modal.value.visible = false
}
const delTls = async (id: number) => { await Data().save('tls', 'del', id) }
</script>

<style scoped>
.cards-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 12px; }
.entity-card { display: flex; flex-direction: column; gap: 10px; padding: 14px 16px 10px; }
.entity-card__head { display: flex; align-items: center; justify-content: space-between; gap: 8px; border-bottom: 1px solid var(--nc-border-soft); padding-bottom: 8px; }
.entity-card__type { font-size: 11px; font-weight: 600; color: var(--nc-primary); background: var(--nc-primary-soft); padding: 2px 8px; border-radius: var(--radius-pill); text-transform: uppercase; letter-spacing: 0.04em; }
.entity-card__tag { font-family: var(--font-display); font-size: 14px; font-weight: 600; color: var(--nc-text-1); flex: 1; text-align: right; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.entity-card__sub { margin: 0; font-size: 11.5px; color: var(--nc-text-muted); font-family: var(--font-mono); }
.entity-card__meta { margin: 0; display: flex; flex-direction: column; gap: 4px; }
.entity-card__row { display: flex; justify-content: space-between; align-items: center; gap: 8px; font-size: 12.5px; }
.entity-card__row dt { color: var(--nc-text-muted); }
.entity-card__row dd { margin: 0; color: var(--nc-text-1); font-weight: 500; }
.entity-card__row .mono { font-family: var(--font-mono); }
.entity-card__actions { display: flex; gap: 4px; border-top: 1px solid var(--nc-border-soft); padding-top: 4px; margin: 4px -4px -4px; }
.entity-card__actions .el-button { flex: 1; min-width: 0; height: 32px; margin: 0 !important; }
.empty-state { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; padding: 48px 16px; text-align: center; }
.empty-state__icon { font-size: 36px; color: var(--nc-text-faint); }
.empty-state__text { margin: 0; font-size: 13px; color: var(--nc-text-muted); }
</style>
