<template>
  <div class="page-container">
    <div class="page-header with-actions">
      <div class="page-header-text">
        <h2 class="page-title">{{ $t('pages.admins') }}</h2>
        <p class="page-desc">{{ $t('admins.desc', '管理员账号、操作日志与 API token') }}</p>
      </div>
      <div class="page-header-actions">
        <el-button @click="showChangesModal('')">
          <el-icon><Document /></el-icon>{{ $t('admin.changes') }}
        </el-button>
        <el-button type="primary" @click="showTokenModal">
          <el-icon><Key /></el-icon>{{ $t('admin.api.token') }}
        </el-button>
      </div>
    </div>

    <div class="cards-grid">
      <div v-for="item in users" :key="item.id" class="entity-card nc-card">
        <div class="entity-card__head">
          <span class="entity-card__type">ADMIN</span>
          <span class="entity-card__tag">{{ item.username }}</span>
        </div>
        <p class="entity-card__sub">{{ $t('admin.lastLogin') }}</p>
        <dl class="entity-card__meta">
          <div class="entity-card__row">
            <dt>{{ $t('admin.date') }}</dt>
            <dd class="mono">{{ item.loginDate || '—' }}</dd>
          </div>
          <div class="entity-card__row">
            <dt>{{ $t('admin.time') }}</dt>
            <dd class="mono">{{ item.loginTime || '—' }}</dd>
          </div>
          <div class="entity-card__row">
            <dt>IP</dt>
            <dd class="mono">{{ item.ip || '—' }}</dd>
          </div>
        </dl>
        <div class="entity-card__actions">
          <el-tooltip :content="$t('actions.edit')" placement="top">
            <el-button text @click="showEditModal(item)"><el-icon><Edit /></el-icon></el-button>
          </el-tooltip>
          <el-tooltip :content="$t('admin.changes')" placement="top">
            <el-button text @click="showChangesModal(item.username)"><el-icon><Document /></el-icon></el-button>
          </el-tooltip>
        </div>
      </div>
    </div>

    <AdminModal
      v-model="editModal.visible"
      :visible="editModal.visible"
      :user="editModal.user"
      @close="closeEditModal"
      @save="saveEditModal"
    />
    <ChangeModal
      v-model="changesModal.visible"
      :visible="changesModal.visible"
      :admins="users.map((u) => u.username)"
      :actor="changesModal.actor"
      @close="closeChangesModal"
    />
    <TokenModal v-model="tokenModal.visible" :visible="tokenModal.visible" @close="closeTokenModal" />
  </div>
</template>

<script lang="ts" setup>
import { i18n } from '@/locales'
import HttpUtils from '@/plugins/httputil'
import { Ref, ref, inject, onMounted, defineAsyncComponent } from 'vue'

const AdminModal = defineAsyncComponent(() => import('@/layouts/modals/Admin.vue'))
const ChangeModal = defineAsyncComponent(() => import('@/layouts/modals/Changes.vue'))
const TokenModal = defineAsyncComponent(() => import('@/layouts/modals/Token.vue'))
import { Document, Key, Edit } from '@element-plus/icons-vue'

const loading: Ref<boolean> = inject('loading') ?? ref(false)

const users = ref<any[]>([])

onMounted(async () => {
  loading.value = true
  await loadData()
  loading.value = false
})

const loadData = async () => {
  loading.value = true
  const msg = await HttpUtils.get('api/users')
  loading.value = false
  if (msg.success) {
    users.value = []
    msg.obj.forEach((u: any) => {
      const lastLogin = u.lastLogin?.split(' ') ?? []
      const localLastLogin = lastLogin.length > 2
        ? dateFormatted(Date.parse(lastLogin[0] + ' ' + lastLogin[1]))
        : '— —'
      const loginDateTime = localLastLogin.split(' ')
      users.value.push({
        id: u.id,
        username: u.username,
        loginDate: loginDateTime[0],
        loginTime: loginDateTime[1],
        ip: lastLogin[2] ?? '—',
      })
    })
  }
}

const dateFormatted = (dt: number): string => {
  const locale = i18n.global.locale.value.replace('zh', 'zh-')
  const date = new Date(dt)
  return date.toLocaleString(locale)
}

const editModal = ref({ visible: false, user: {} as any })
const showEditModal = (user: any) => { editModal.value.user = user; editModal.value.visible = true }
const closeEditModal = () => { editModal.value.visible = false; editModal.value.user = {} }
const saveEditModal = async (data: any) => {
  loading.value = true
  const r = await HttpUtils.post('api/changePass', data)
  if (r.success) setTimeout(() => { loading.value = false; editModal.value.visible = false }, 500)
  else loading.value = false
}

const changesModal = ref({ visible: false, actor: '' })
const showChangesModal = (actor: string) => { changesModal.value.actor = actor; changesModal.value.visible = true }
const closeChangesModal = () => { changesModal.value.visible = false; changesModal.value.actor = '' }

const tokenModal = ref({ visible: false })
const showTokenModal = () => { tokenModal.value.visible = true }
const closeTokenModal = () => { tokenModal.value.visible = false }
</script>

<style scoped>
.cards-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 12px; }
.entity-card { display: flex; flex-direction: column; gap: 10px; padding: 14px 16px 10px; }
.entity-card__head { display: flex; align-items: center; justify-content: space-between; gap: 8px; border-bottom: 1px solid var(--nc-border-soft); padding-bottom: 8px; }
.entity-card__type { font-size: 11px; font-weight: 600; color: var(--nc-primary); background: var(--nc-primary-soft); padding: 2px 8px; border-radius: var(--radius-pill); text-transform: uppercase; letter-spacing: 0.04em; }
.entity-card__tag { font-family: var(--font-display); font-size: 14px; font-weight: 600; color: var(--nc-text-1); flex: 1; text-align: right; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.entity-card__sub { margin: 0; font-size: 11.5px; color: var(--nc-text-muted); }
.entity-card__meta { margin: 0; display: flex; flex-direction: column; gap: 4px; }
.entity-card__row { display: flex; justify-content: space-between; align-items: center; gap: 8px; font-size: 12.5px; }
.entity-card__row dt { color: var(--nc-text-muted); }
.entity-card__row dd { margin: 0; color: var(--nc-text-1); font-weight: 500; }
.entity-card__row .mono { font-family: var(--font-mono); }
.entity-card__actions { display: flex; gap: 4px; border-top: 1px solid var(--nc-border-soft); padding-top: 4px; margin: 4px -4px -4px; }
.entity-card__actions .el-button { flex: 1; min-width: 0; height: 32px; margin: 0 !important; }
</style>
