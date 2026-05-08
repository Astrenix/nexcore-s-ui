/**
 * main.ts
 *
 * Bootstraps Element Plus + plugins, mounts the App.
 */

import { createApp, ref } from 'vue'

// 全局样式(NexCore 设计 token + 全局覆盖 + dialog 约束)
import '@/styles/vars.css'
import '@/styles/global.css'
import '@/styles/dialog-constraints.css'

import App from './App.vue'

// 路由 / Store / 国际化
import router from './router'
import store from './store'
import { i18n } from '@/locales'

// 插件(Element Plus 主插件)
import { registerPlugins } from '@/plugins'

// 给 body 加上 NexCore 命名空间 class,激活 global.css 接管
document.body.classList.add('app-admin')
const _initLocale = localStorage.getItem('locale') ?? 'en'
document.body.classList.add(`app-${_initLocale}`)

// 全局 loading ref(被 App.vue 通过 inject 消费)
const loading = ref(false)

const app = createApp(App)
app.provide('loading', loading)

// EP 插件(必须在 app.use(router/store/i18n) 之前或之后均可,但样式必须早 import)
registerPlugins(app)

app
  .use(router)
  .use(store)
  .use(i18n)
  .mount('#app')

// 路由切换:同步 body 上的语言 class、document.title、RTL 方向
router.afterEach((to) => {
  const cur = i18n.global.locale.value
  document.body.classList.forEach((c) => {
    if (c.startsWith('app-') && c !== 'app-admin') document.body.classList.remove(c)
  })
  document.body.classList.add(`app-${cur}`)
  document.documentElement.dir = cur === 'fa' ? 'rtl' : 'ltr'

  if (typeof to.name === 'string') {
    const pageName = i18n.global.t(to.name)
    document.title = pageName === to.name ? 'S-UI' : `S-UI · ${pageName}`
  } else {
    document.title = 'S-UI'
  }
})
