// Composables
import { createRouter, createWebHistory } from 'vue-router'
import Login from '@/views/Login.vue'
import Data from '@/store/modules/data'

const routes = [
  {
    path: '/login',
    name: 'pages.login',
    component: Login,
  },
  {
    path: '/',
    component: () => import('@/layouts/default/Default.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: '/',
        name: 'pages.home',
        component: () => import('@/views/Home.vue'),
      },
      {
        path: '/inbounds',
        name: 'pages.inbounds',
        component: () => import('@/views/Inbounds.vue'),
      },
      {
        path: '/outbounds',
        name: 'pages.outbounds',
        component: () => import('@/views/Outbounds.vue'),
      },
      {
        // 订阅池:导入机场订阅链接,自动探测 + 国家分组 + Winner 选举 → 写 pool-{cc} 出站。
        // 入站可绑 "订阅池(选国家)",节点更新对入站透明。详见后端 service/sub.go。
        path: '/sub-pools',
        name: 'pages.subPools',
        component: () => import('@/views/SubPools.vue'),
      },
      {
        path: '/endpoints',
        name: 'pages.endpoints',
        component: () => import('@/views/Endpoints.vue'),
      },
      {
        path: '/rules',
        name: 'pages.rules',
        component: () => import('@/views/Rules.vue'),
      },
      {
        path: '/block-rules',
        name: 'pages.blockRules',
        component: () => import('@/views/BlockRules.vue'),
      },
      {
        path: '/tls',
        name: 'pages.tls',
        component: () => import('@/views/Tls.vue'),
      },
      {
        path: '/dns',
        name: 'pages.dns',
        component: () => import('@/views/Dns.vue'),
      },
      {
        path: '/api',
        name: 'pages.api',
        component: () => import('@/views/Api.vue'),
      },
      {
        path: '/settings',
        name: 'pages.settings',
        component: () => import('@/views/Settings.vue'),
      },
      // 老路径兼容:/clients 已合并进 /inbounds(每个入站点「客户端」按钮),
      // /basics 已合并进 /settings 的「内核」tab,/admins 改密合并进「账号」tab。
      // 这些 redirect 让旧书签 / 浏览器历史不至于报 No match 警告。
      { path: '/clients', redirect: '/inbounds' },
      { path: '/basics', redirect: '/settings' },
      { path: '/admins', redirect: '/settings' },
      { path: '/services', redirect: '/inbounds' },
      // catch-all:其它未知路径回首页
      { path: '/:pathMatch(.*)*', redirect: '/' },
    ],
  },
]

const router = createRouter({
  history: createWebHistory((window as any).BASE_URL),
  routes,
})

const DEFAULT_TITLE = 'S-UI'
let intervalId:any

// Navigation guard to check authentication state.
//
// 用 localStorage('admin_username') 而不是读 cookie:v1.7.12 给 session
// cookie 加了 HttpOnly,document.cookie 拿不到 — 守卫永远判未登录,登录
// 成功后 router.push('/') 又被守卫拦回 /login,死循环。
//
// localStorage 只表示"曾经登录过",不等于 session 仍有效;cookie 真过期
// 后 API 会 401,由 plugins/httputil 全局拦截器统一清 localStorage + 跳
// /login(单一鉴权信号源)。
router.beforeEach((to) => {
  const isAuthenticated = !!localStorage.getItem('admin_username')

  if (to.meta.requiresAuth && !isAuthenticated) {
    return '/login'
  }
  if (to.path === '/login' && isAuthenticated) {
    return '/'
  }

  // Load default data
  if (to.path !== '/login') {
    loadDataInterval()
  } else {
    if (intervalId) {
      clearInterval(intervalId)
      intervalId = undefined
    }
  }
})

const loadDataInterval = () => {
  if (intervalId) return
  Data().loadData()
  intervalId = setInterval(() => {
    Data().loadData()
  }, 10000)
}

export default router
