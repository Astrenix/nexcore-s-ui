<template>
  <div class="page-container">
    <div class="page-header with-actions">
      <div class="page-header-text">
        <h2 class="page-title">{{ $t('pages.clients') }}</h2>
        <p class="page-desc">{{ $t('clients.desc', '管理用户、订阅、流量配额与到期时间') }}</p>
      </div>
      <div class="page-header-actions">
        <el-button type="primary" @click="showModal(0)">
          <el-icon><Plus /></el-icon>{{ $t('actions.add') }}
        </el-button>
        <el-dropdown trigger="click">
          <el-button>
            <el-icon><Tools /></el-icon>{{ $t('main.tiles', '批量') }}
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item @click="addBulk">
                <el-icon style="margin-right: 6px"><Plus /></el-icon>
                {{ $t('actions.addbulk') }}
              </el-dropdown-item>
              <el-dropdown-item @click="editBulk">
                <el-icon style="margin-right: 6px"><Edit /></el-icon>
                {{ $t('actions.editbulk') }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <div class="filter-bar">
      <div class="filter-row">
        <el-select
          v-model="filterSettings.state"
          :placeholder="$t('type')"
          clearable
          class="filter-select"
        >
          <el-option v-for="f in filterItems" :key="f.value" :label="f.title" :value="f.value" />
        </el-select>
        <el-select
          v-model="filterSettings.group"
          :placeholder="$t('client.group')"
          clearable
          class="filter-select"
        >
          <el-option :label="$t('all')" value="-" />
          <el-option
            v-for="g in groups"
            :key="g"
            :label="g.length > 0 ? g : $t('none')"
            :value="g"
          />
        </el-select>
        <el-input
          v-model="filterSettings.text"
          :placeholder="$t('client.name')"
          clearable
          class="filter-input"
        >
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <div class="filter-actions">
          <el-button type="primary" @click="doFilter">
            <el-icon><Search /></el-icon>{{ $t('actions.update') }}
          </el-button>
          <el-button @click="clearFilter">
            <el-icon><RefreshLeft /></el-icon>{{ $t('reset') }}
          </el-button>
        </div>
      </div>
    </div>

    <div class="list-card">
      <el-table
        :data="filterSettings.enabled ? filterSettings.filteredClients : clients"
        empty-text=" "
        stripe
        size="default"
        @row-dblclick="(row: any) => showModal(row.id)"
      >
        <el-table-column prop="name" :label="$t('client.name')" min-width="120" sortable />
        <el-table-column prop="desc" :label="$t('client.desc')" min-width="120" show-overflow-tooltip />
        <el-table-column prop="group" :label="$t('client.group')" min-width="100" sortable />
        <el-table-column :label="$t('pages.inbounds')" width="90">
          <template #default="{ row }">
            <el-tooltip
              v-if="row.inbounds?.length"
              :content="(row.inbounds || []).map((id: number) => inbounds.find((i) => i.id == id)?.tag).filter(Boolean).join(' / ')"
              placement="top"
            >
              <span class="number-cell">{{ row.inbounds.length }}</span>
            </el-tooltip>
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('stats.volume')" min-width="200">
          <template #default="{ row }">
            <div class="volume-cell">
              <el-tooltip
                placement="top"
                :content="`↓ ${HumanReadable.sizeFormat(row.down)}  /  ↑ ${HumanReadable.sizeFormat(row.up)}`"
              >
                <el-tag
                  size="small"
                  :type="row.volume == 0 ? 'success' : (row.volume <= row.up + row.down ? 'danger' : 'info')"
                  effect="light"
                >
                  {{ HumanReadable.sizeFormat(row.up + row.down) }} /
                  {{ row.volume == 0 ? $t('unlimited') : HumanReadable.sizeFormat(row.volume) }}
                </el-tag>
              </el-tooltip>
              <el-progress
                v-if="row.volume > 0"
                :percentage="percent(row)"
                :status="percentStatus(row)"
                :show-text="false"
                :stroke-width="3"
              />
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="$t('date.expiry')" width="130">
          <template #default="{ row }">
            <el-tooltip
              v-if="row.expiry > 0"
              placement="top"
              :content="new Date(row.expiry * 1000).toLocaleString(locale)"
            >
              <el-tag
                size="small"
                :type="row.expiry == 0 ? 'success' : (row.expiry <= Date.now() / 1000 ? 'danger' : 'info')"
                effect="light"
              >{{ HumanReadable.remainedDays(row.expiry) }}</el-tag>
            </el-tooltip>
            <el-tag v-else type="success" size="small" effect="light">
              {{ HumanReadable.remainedDays(row.expiry) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('online')" width="80">
          <template #default="{ row }">
            <span v-if="onlineUsers.includes(row.name)" class="status-pill status-pill--ok">
              <span class="status-dot online"></span>{{ $t('online') }}
            </span>
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('actions.action')" width="160" fixed="right">
          <template #default="{ row }">
            <div class="row-actions">
              <el-tooltip :content="$t('actions.edit')" placement="top">
                <el-button text @click="showModal(row.id)"><el-icon><Edit /></el-icon></el-button>
              </el-tooltip>
              <el-popconfirm
                :title="$t('confirm')"
                :confirm-button-text="$t('yes')"
                :cancel-button-text="$t('no')"
                @confirm="delClient(row.id)"
              >
                <template #reference>
                  <el-button text>
                    <el-tooltip :content="$t('actions.del')" placement="top">
                      <el-icon style="color: var(--nc-danger)"><Delete /></el-icon>
                    </el-tooltip>
                  </el-button>
                </template>
              </el-popconfirm>
              <el-tooltip :content="$t('main.qr', 'QR')" placement="top">
                <el-button text @click="showQrCode(row.id)"><el-icon><Picture /></el-icon></el-button>
              </el-tooltip>
              <el-tooltip v-if="Data().enableTraffic" :content="$t('stats.graphTitle')" placement="top">
                <el-button text @click="showStats(row.name)"><el-icon><DataLine /></el-icon></el-button>
              </el-tooltip>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <ClientModal
      v-model="modal.visible"
      :visible="modal.visible"
      :id="modal.id"
      :groups="groups"
      :inboundTags="inboundTags"
      @close="closeModal"
    />
    <ClientAddBulk
      v-model="addBulkModal"
      :visible="addBulkModal"
      :groups="groups"
      :inboundTags="inboundTags"
      @close="closeAddBulk"
    />
    <ClientEditBulk
      v-model="editBulkModal"
      :visible="editBulkModal"
      :inboundTags="inboundTags"
      :clients="clients"
      @close="closeEditBulk"
    />
    <QrCode v-model="qrcode.visible" :visible="qrcode.visible" :id="qrcode.id" @close="closeQrCode" />
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
import { Client } from '@/types/clients'
import { computed, defineAsyncComponent, ref } from 'vue'

const ClientModal = defineAsyncComponent(() => import('@/layouts/modals/Client.vue'))
const ClientAddBulk = defineAsyncComponent(() => import('@/layouts/modals/ClientAddBulk.vue'))
const ClientEditBulk = defineAsyncComponent(() => import('@/layouts/modals/ClientEditBulk.vue'))
const QrCode = defineAsyncComponent(() => import('@/layouts/modals/QrCode.vue'))
const Stats = defineAsyncComponent(() => import('@/layouts/modals/Stats.vue'))
import { HumanReadable } from '@/plugins/utils'
import { i18n, locale } from '@/locales'
import {
  Plus, Edit, Delete, DataLine, Picture, Search, RefreshLeft, Tools,
} from '@element-plus/icons-vue'

const clients = computed((): any[] => Data().clients ?? [])

const onlineUsers = computed(() => Data().onlines?.user ?? [])

const inbounds = computed((): any[] => Data().inbounds ?? [])

const inboundTags = computed((): any[] => {
  if (!inbounds.value) return []
  return inbounds.value
    .filter((i) => i.tag != '' && i.users)
    .map((i) => ({ title: i.tag, value: i.id }))
})

const groups = computed((): string[] => {
  if (!clients.value) return []
  if (filterSettings.value.enabled)
    return Array.from(new Set(filterSettings.value.filteredClients.map((c: any) => c.group)))
  return Array.from(new Set(clients.value.map((c: any) => c.group)))
})

const filterSettings = ref({
  enabled: false,
  state: '',
  group: '-',
  text: '',
  filteredClients: <any[]>[],
})

const filterItems = [
  { title: i18n.global.t('none'), value: '' },
  { title: i18n.global.t('disable'), value: 'disable' },
  { title: i18n.global.t('date.expired'), value: 'expired' },
  { title: i18n.global.t('online'), value: 'online' },
]

const modal = ref({ visible: false, id: 0 })

const showModal = (id: number) => {
  modal.value.id = id
  modal.value.visible = true
}
const closeModal = () => { modal.value.visible = false }

const delClient = async (id: number) => {
  await Data().save('clients', 'del', id)
}

const qrcode = ref({ visible: false, id: 0 })
const showQrCode = (id: number) => {
  qrcode.value.id = id
  qrcode.value.visible = true
}
const closeQrCode = () => { qrcode.value.visible = false }

const stats = ref({ visible: false, resource: 'user', tag: '' })
const showStats = (tag: string) => {
  stats.value.tag = tag
  stats.value.visible = true
}
const closeStats = () => { stats.value.visible = false }

const doFilter = () => {
  let filtered = clients.value.slice()
  const f = filterSettings.value
  if (f.group !== '-') filtered = filtered.filter((c) => c.group === f.group)
  if (f.text.length > 0) {
    const t = f.text
    filtered = filtered.filter((c) => c.name?.includes(t) || c.desc?.includes(t))
  }
  switch (f.state) {
    case 'disable':
      filtered = filtered.filter((c) => !c.enable)
      break
    case 'expired':
      filtered = filtered.filter((c) => c.expiry > 0 && c.expiry < Date.now() / 1000)
      break
    case 'online':
      filtered = filtered.filter((c) => Data().onlines?.user?.includes(c.name))
      break
  }
  f.filteredClients = filtered
  f.enabled = true
}

const clearFilter = () => {
  filterSettings.value = { enabled: false, state: '', group: '-', text: '', filteredClients: [] }
}

const addBulkModal = ref(false)
const addBulk = () => { addBulkModal.value = true }
const closeAddBulk = () => { addBulkModal.value = false }

const editBulkModal = ref(false)
const editBulk = () => { editBulkModal.value = true }
const closeEditBulk = () => { editBulkModal.value = false }

const percent = (c: Client) => (c.volume > 0 ? Math.round(((c.up + c.down) * 100) / c.volume) : 0)
const percentStatus = (c: Client): 'success' | 'warning' | 'exception' =>
  c.up + c.down >= c.volume ? 'exception' : percent(c) > 90 ? 'warning' : 'success'
</script>

<style scoped>
.filter-bar {
  background: #fff;
  border: 1px solid var(--nc-border);
  border-radius: var(--radius-lg);
  padding: 12px 16px;
}

.filter-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.filter-select {
  width: 150px;
}

.filter-input {
  width: 200px;
}

.filter-actions {
  margin-left: auto;
  display: flex;
  gap: 8px;
}

.list-card {
  background: #fff;
  border: 1px solid var(--nc-border);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.volume-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.row-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}

.row-actions .el-button {
  margin: 0;
  padding: 4px 6px;
}

.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11.5px;
  color: var(--nc-success);
  font-weight: 500;
}

.number-cell {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}

@media (max-width: 768px) {
  .filter-row .filter-select,
  .filter-row .filter-input {
    width: 100%;
  }
  .filter-actions {
    width: 100%;
    margin-left: 0;
  }
}
</style>
