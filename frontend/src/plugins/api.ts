import axios from 'axios'
import router from '@/router'

// baseURL / 通用头显式设在实例上,不依赖 axios.defaults 的隐式继承。
const api = axios.create({
  baseURL: './',
  headers: {
    'X-Requested-With': 'XMLHttpRequest',
    'Content-Type': 'application/x-www-form-urlencoded; charset=UTF-8',
  },
})

api.interceptors.request.use(
  (config) => {
    if (config.data instanceof FormData) {
      config.headers['Content-Type'] = 'multipart/form-data'
    }
    return config
  },
  (error) => Promise.reject(error),
)

// 会话失效统一处理。历史实现把去重/取消拦截器错误地挂在全局 axios 上,而请求
// 全部走 axios.create() 出来的独立实例 —— 拦截器从未生效(死代码),同时真正的
// HTTP 401(cookie 过期)不会被识别,用户卡在满屏报错的僵尸界面。
//
// 这里在**真正生效的实例**上补一个响应拦截器:遇到 401 就清掉前端登录标记并跳
// 登录页。请求去重故意不重新实现 —— 它本就没生效过,且盲目加回会误伤
// Inbounds/Outbounds 页并发的 checkOutbound 请求(同 URL 互相取消)。
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error?.response?.status === 401) {
      try {
        localStorage.removeItem('admin_username')
      } catch {
        // localStorage 不可用时忽略,跳转仍然进行
      }
      if (router.currentRoute.value.path !== '/login') {
        router.push('/login')
      }
    }
    return Promise.reject(error)
  },
)

export default api
