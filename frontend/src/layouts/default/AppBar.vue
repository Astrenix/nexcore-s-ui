<template>
  <header class="nc-header">
    <button
      v-if="isMobile"
      class="nc-header__icon-btn"
      @click="$emit('toggleDrawer')"
      :aria-label="$t('actions.menu', '菜单')"
    >
      <el-icon><Expand /></el-icon>
    </button>
    <button
      v-else
      class="nc-header__icon-btn"
      @click="$emit('toggleCollapse')"
      :aria-label="$t('actions.toggleSidebar', '折叠侧边栏')"
    >
      <el-icon><Fold v-if="!collapsed" /><Expand v-else /></el-icon>
    </button>

    <h1 class="nc-header__title">{{ pageTitle }}</h1>

    <div class="nc-header__actions">
      <el-dropdown trigger="click" @command="changeLocale">
        <button class="nc-header__icon-btn" :aria-label="$t('language', '语言')">
          <el-icon><LocationInformation /></el-icon>
        </button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item
              v-for="lang in languages"
              :key="lang.value"
              :command="lang.value"
              :class="{ 'is-active': i18nLocale === lang.value }"
            >
              {{ lang.title }}
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>

      <el-dropdown trigger="click" @command="changeTheme">
        <button class="nc-header__icon-btn" :aria-label="$t('theme.system')">
          <el-icon><Sunny v-if="currentTheme === 'light'" /><Moon v-else-if="currentTheme === 'dark'" /><Monitor v-else /></el-icon>
        </button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item v-for="th in themes" :key="th.value" :command="th.value">
              <el-icon style="margin-right: 8px"><component :is="th.icon" /></el-icon>
              {{ $t(`theme.${th.value}`) }}
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
  </header>
</template>

<script lang="ts" setup>
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { languages } from '@/locales'
import { applyEPLocale } from '@/plugins/element-plus'
import {
  Expand,
  Fold,
  Sunny,
  Moon,
  Monitor,
  LocationInformation,
} from '@element-plus/icons-vue'
import { markRaw } from 'vue'

defineProps<{ isMobile: boolean; collapsed: boolean }>()
defineEmits<{ toggleDrawer: []; toggleCollapse: [] }>()

const route = useRoute()
const { locale: i18nLocale, t } = useI18n()

const pageTitle = computed(() => {
  const name = route.name
  if (typeof name === 'string') return t(name)
  return 'S-UI'
})

const themes = [
  { value: 'light',  icon: markRaw(Sunny) },
  { value: 'dark',   icon: markRaw(Moon) },
  { value: 'system', icon: markRaw(Monitor) },
]

const currentTheme = ref(localStorage.getItem('theme') ?? 'system')

const changeLocale = (l: string) => {
  i18nLocale.value = l
  localStorage.setItem('locale', l)
  applyEPLocale(l)
  window.location.reload()
}

const changeTheme = (th: string) => {
  currentTheme.value = th
  localStorage.setItem('theme', th)
  const html = document.documentElement
  if (th === 'dark') {
    html.classList.add('dark')
  } else if (th === 'light') {
    html.classList.remove('dark')
  } else {
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
    html.classList.toggle('dark', prefersDark)
  }
}

// 初始化主题
changeTheme(currentTheme.value)
</script>

<style scoped>
.nc-header {
  height: var(--shell-header-h);
  background: #ffffff;
  border-bottom: 1px solid var(--nc-border);
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 16px;
  position: sticky;
  top: 0;
  z-index: 100;
}

.nc-header__icon-btn {
  width: 36px;
  height: 36px;
  border-radius: var(--radius-md);
  background: transparent;
  border: none;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--nc-text-3);
  transition: background var(--t-fast);
  font-size: 16px;
}

.nc-header__icon-btn:hover {
  background: var(--nc-border-soft);
  color: var(--nc-text-1);
}

.nc-header__title {
  flex: 1;
  font-size: 15px;
  font-weight: 600;
  color: var(--nc-text-1);
  font-family: var(--font-display);
  letter-spacing: -0.01em;
  margin: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.nc-header__actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}
</style>
