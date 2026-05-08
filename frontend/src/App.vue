<template>
  <el-config-provider :locale="epLocale" size="default" :z-index="3000">
    <div v-if="loading" class="global-loading">
      <el-icon class="is-loading global-loading__spinner"><Loading /></el-icon>
      <span class="global-loading__text">{{ $t('loading') }}</span>
    </div>
    <router-view />
  </el-config-provider>
</template>

<script lang="ts" setup>
import { computed, inject, ref, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Loading } from '@element-plus/icons-vue'
import enLocale from 'element-plus/es/locale/lang/en'
import zhCnLocale from 'element-plus/es/locale/lang/zh-cn'
import zhTwLocale from 'element-plus/es/locale/lang/zh-tw'
import faLocale from 'element-plus/es/locale/lang/fa'
import ruLocale from 'element-plus/es/locale/lang/ru'
import viLocale from 'element-plus/es/locale/lang/vi'

const loading: Ref<boolean> = inject('loading') ?? ref(false)

const { locale: i18nLocale } = useI18n()

const epLocaleMap: Record<string, any> = {
  en: enLocale,
  zhHans: zhCnLocale,
  zhHant: zhTwLocale,
  fa: faLocale,
  ru: ruLocale,
  vi: viLocale,
}

const epLocale = computed(() => epLocaleMap[i18nLocale.value] || enLocale)
</script>

<style scoped>
.global-loading {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background: rgba(15, 23, 42, 0.45);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  backdrop-filter: blur(2px);
}
.global-loading__spinner {
  font-size: 36px;
  color: #fff;
}
.global-loading__text {
  color: #fff;
  font-size: 13px;
  font-family: var(--font-display);
  letter-spacing: 0.02em;
}
</style>
