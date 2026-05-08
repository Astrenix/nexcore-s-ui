/**
 * plugins/element-plus.ts
 *
 * dayjs locale 初始化(EP 内部用 dayjs)+ 程序化 API CSS 注入。
 * 组件本身的 CSS 通过 unplugin-vue-components 的 ElementPlusResolver 按需引入(见 vite.config.mts)。
 * EP 全局 locale 由 App.vue 中的 <el-config-provider> 通过 vue-i18n 联动。
 */

import type { App } from 'vue'

// 程序化 API 必需的 CSS(组件 CSS 已被 ElementPlusResolver 按需引入)
import 'element-plus/theme-chalk/el-message.css'
import 'element-plus/theme-chalk/el-notification.css'
import 'element-plus/theme-chalk/el-loading.css'
import 'element-plus/theme-chalk/el-message-box.css'
import 'element-plus/theme-chalk/el-overlay.css'
import 'element-plus/theme-chalk/dark/css-vars.css'

import dayjs from 'dayjs'
import 'dayjs/locale/zh-cn'
import 'dayjs/locale/zh-tw'
import 'dayjs/locale/fa'
import 'dayjs/locale/ru'
import 'dayjs/locale/vi'

const dayjsLocaleMap: Record<string, string> = {
  en: 'en',
  zhHans: 'zh-cn',
  zhHant: 'zh-tw',
  fa: 'fa',
  ru: 'ru',
  vi: 'vi',
}

export function applyEPLocale(lang: string) {
  const dl = dayjsLocaleMap[lang] || 'en'
  dayjs.locale(dl)
}

export default {
  install(_app: App) {
    const stored = localStorage.getItem('locale') ?? 'en'
    applyEPLocale(stored)
  },
}
