<template>
  <aside
    class="nc-aside"
    :class="{
      'is-mobile': isMobile,
      'is-mobile-open': isMobile && displayDrawer,
      'is-collapsed': !isMobile && collapsed,
    }"
  >
    <div class="nc-aside__brand">
      <img src="@/assets/logo.svg" alt="S-UI" class="nc-aside__logo" />
      <span class="nc-aside__title" v-show="!collapsed || isMobile">S-UI</span>
      <button
        v-if="isMobile"
        class="nc-aside__close"
        @click="$emit('toggleDrawer')"
        :aria-label="$t('actions.close')"
      >
        <el-icon><Close /></el-icon>
      </button>
    </div>

    <nav class="nc-aside__nav">
      <router-link
        v-for="item in menu"
        :key="item.path"
        :to="item.path"
        class="nc-aside__item"
        :class="{ 'is-active': route.path === item.path }"
        @click="onNavClick"
      >
        <el-icon class="nc-aside__icon">
          <component :is="item.icon" />
        </el-icon>
        <span class="nc-aside__label" v-show="!collapsed || isMobile">{{ $t(item.title) }}</span>
      </router-link>
    </nav>

    <div class="nc-aside__footer">
      <button class="nc-aside__item nc-aside__logout" @click="onLogout">
        <el-icon class="nc-aside__icon"><SwitchButton /></el-icon>
        <span class="nc-aside__label" v-show="!collapsed || isMobile">{{ $t('menu.logout') }}</span>
      </button>
    </div>
  </aside>
</template>

<script lang="ts" setup>
import { useRoute } from 'vue-router'
import { logout } from '@/plugins/httputil'
import {
  House,
  Download,
  User,
  Upload,
  PriceTag,
  Operation,
  Lock,
  Setting,
  Connection,
  Promotion,
  Avatar,
  Tools,
  Close,
  SwitchButton,
} from '@element-plus/icons-vue'
import { markRaw } from 'vue'

const props = defineProps<{
  isMobile: boolean
  displayDrawer: boolean
  collapsed: boolean
}>()
const emit = defineEmits<{ toggleDrawer: []; toggleCollapse: [] }>()
void props
void emit

const route = useRoute()

const menu = [
  { title: 'pages.home',      icon: markRaw(House),       path: '/' },
  { title: 'pages.inbounds',  icon: markRaw(Download),    path: '/inbounds' },
  { title: 'pages.clients',   icon: markRaw(User),        path: '/clients' },
  { title: 'pages.outbounds', icon: markRaw(Upload),      path: '/outbounds' },
  { title: 'pages.endpoints', icon: markRaw(PriceTag),    path: '/endpoints' },
  { title: 'pages.services',  icon: markRaw(Operation),   path: '/services' },
  { title: 'pages.tls',       icon: markRaw(Lock),        path: '/tls' },
  { title: 'pages.basics',    icon: markRaw(Tools),       path: '/basics' },
  { title: 'pages.rules',     icon: markRaw(Connection),  path: '/rules' },
  { title: 'pages.dns',       icon: markRaw(Promotion),   path: '/dns' },
  { title: 'pages.admins',    icon: markRaw(Avatar),      path: '/admins' },
  { title: 'pages.settings',  icon: markRaw(Setting),     path: '/settings' },
]

const onNavClick = () => {
  if (props.isMobile) emit('toggleDrawer')
}

const onLogout = () => {
  logout()
}
</script>

<style scoped>
.nc-aside {
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  width: var(--shell-aside-w);
  background: #ffffff;
  border-right: 1px solid var(--nc-border);
  display: flex;
  flex-direction: column;
  z-index: 1000;
  transition: width var(--t-base), transform var(--t-base);
}

.nc-aside.is-collapsed {
  width: var(--shell-aside-w-collapsed);
}

.nc-aside.is-mobile {
  transform: translateX(-100%);
  width: 260px;
}

.nc-aside.is-mobile-open {
  transform: translateX(0);
  box-shadow: 4px 0 20px rgba(15, 23, 42, 0.12);
}

.nc-aside__brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 18px;
  height: var(--shell-header-h);
  border-bottom: 1px solid var(--nc-border);
  flex-shrink: 0;
  position: relative;
}

.nc-aside__logo {
  width: 28px;
  height: 28px;
  flex-shrink: 0;
}

.nc-aside__title {
  font-family: var(--font-display);
  font-size: 16px;
  font-weight: 700;
  color: var(--nc-text-1);
  letter-spacing: -0.01em;
}

.nc-aside__close {
  position: absolute;
  right: 8px;
  top: 50%;
  transform: translateY(-50%);
  background: transparent;
  border: none;
  width: 32px;
  height: 32px;
  border-radius: var(--radius-md);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--nc-text-muted);
}

.nc-aside__close:hover {
  background: var(--nc-border-soft);
  color: var(--nc-text-1);
}

.nc-aside__nav {
  flex: 1;
  overflow-y: auto;
  padding: 12px 10px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.nc-aside__item {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 38px;
  padding: 0 12px;
  border-radius: var(--radius-md);
  color: var(--nc-text-3);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  text-decoration: none;
  background: transparent;
  border: none;
  width: 100%;
  text-align: left;
  font-family: inherit;
  transition: background var(--t-fast), color var(--t-fast);
  white-space: nowrap;
  overflow: hidden;
}

.nc-aside__item:hover {
  background: var(--nc-border-soft);
  color: var(--nc-text-1);
}

.nc-aside__item.is-active {
  background: var(--nc-primary-soft);
  color: var(--nc-primary-deep);
  font-weight: 600;
}

.nc-aside__item.is-active .nc-aside__icon {
  color: var(--nc-primary);
}

.nc-aside__icon {
  font-size: 16px;
  flex-shrink: 0;
  color: var(--nc-text-muted);
}

.nc-aside__label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.nc-aside__footer {
  padding: 10px;
  border-top: 1px solid var(--nc-border);
  flex-shrink: 0;
}

.nc-aside.is-collapsed .nc-aside__brand {
  padding: 0;
  justify-content: center;
}

.nc-aside.is-collapsed .nc-aside__item {
  justify-content: center;
  padding: 0;
}
</style>
