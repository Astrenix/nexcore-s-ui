<template>
  <div class="app-shell" :class="{ 'is-mobile': isMobile, 'is-collapsed': !isMobile && collapsed }">
    <Drawer
      :is-mobile="isMobile"
      :display-drawer="displayDrawer"
      :collapsed="collapsed"
      @toggle-drawer="toggleDrawer"
      @toggle-collapse="toggleCollapse"
    />
    <div class="app-shell__main">
      <AppBar
        :is-mobile="isMobile"
        :collapsed="collapsed"
        @toggle-drawer="toggleDrawer"
        @toggle-collapse="toggleCollapse"
      />
      <View />
    </div>
    <div
      v-if="isMobile && displayDrawer"
      class="app-shell__mask"
      @click="toggleDrawer"
    ></div>
  </div>
</template>

<script lang="ts" setup>
import { ref, watch } from 'vue'
import AppBar from './AppBar.vue'
import Drawer from './Drawer.vue'
import View from './View.vue'
import { useBreakpoint } from '@/composables/useBreakpoint'

const { isMobile } = useBreakpoint()

const displayDrawer = ref(false)
const collapsed = ref(localStorage.getItem('sidebarCollapsed') === '1')

const toggleDrawer = () => {
  displayDrawer.value = !displayDrawer.value
}

const toggleCollapse = () => {
  collapsed.value = !collapsed.value
  localStorage.setItem('sidebarCollapsed', collapsed.value ? '1' : '0')
}

watch(isMobile, (m) => {
  if (m) displayDrawer.value = false
})
</script>

<style scoped>
.app-shell {
  display: flex;
  min-height: 100vh;
  background: var(--nc-bg);
}

.app-shell__main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  margin-left: var(--shell-aside-w);
  transition: margin-left var(--t-base);
}

.app-shell.is-collapsed .app-shell__main {
  margin-left: var(--shell-aside-w-collapsed);
}

.app-shell.is-mobile .app-shell__main {
  margin-left: 0;
}

.app-shell__mask {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.45);
  z-index: 999;
  backdrop-filter: blur(2px);
  animation: maskFade var(--t-base);
}

@keyframes maskFade {
  from { opacity: 0; }
  to { opacity: 1; }
}
</style>
