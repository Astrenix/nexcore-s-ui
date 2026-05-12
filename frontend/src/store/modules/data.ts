import HttpUtils from '@/plugins/httputil'
import { defineStore } from 'pinia'
import { ElMessage } from 'element-plus'
import { i18n } from '@/locales'
import { Inbound } from '@/types/inbounds'
import { Client } from '@/types/clients'

const Data = defineStore('Data', {
  state: () => ({
    lastLoad: 0,
    reloadItems: localStorage.getItem('reloadItems')?.split(',') ?? <string[]>[],
    enableTraffic: false,
    onlines: {
      inbound: <string[]>[],
      outbound: <string[]>[],
      user: <string[]>[],
      // 60s 窗口内的活跃 source IP 计数:tag → IP 数。
      // 入站列表显示并发 IP,客户端列表显示账号被多少 IP 同时使用(共享检测)。
      inbound_ips: <Record<string, number>>{},
      user_ips: <Record<string, number>>{},
    },
    config: <any>{},
    inbounds: <any[]>[],
    outbounds: <any[]>[],
    // 订阅池出站(pool_outbounds 表),独立于 outbounds;入站编辑器要列出来给中转绑定用
    poolOutbounds: <any[]>[],
    endpoints: <any[]>[],
    clients: <any>[],
    tlsConfigs: <any[]>[],
  }),
  actions: {
    async loadData() {
      const msg = await HttpUtils.get('api/load', this.lastLoad > 0 ? { lu: this.lastLoad } : {})
      if (msg.success) {
        // 防御性 merge:后端 onlines 字段都是 omitempty,可能为空对象/缺字段。
        // 直接整体替换会让 inbound_ips/user_ips 变 undefined,模板访问时崩。
        const o = msg.obj.onlines || {}
        this.onlines = {
          inbound: o.inbound || [],
          outbound: o.outbound || [],
          user: o.user || [],
          inbound_ips: o.inbound_ips || {},
          user_ips: o.user_ips || {},
        }
        if (msg.obj.lastLog) {
          // 后端把每行 sing-box stderr 都当成 lastLog 推过来,但实际包含 INFO/WARN/ERROR 多种级别。
          // 只对 ERROR/FATAL 级别弹错;WARN 弹警告;INFO(如 cooldown 倒计时)直接静默,避免每 15 秒一次红框骚扰。
          const log = String(msg.obj.lastLog)
          const isError = / ERROR | FATAL |panic:/i.test(log)
          const isWarn = / WARN /i.test(log)
          if (isError) {
            ElMessage({
              type: 'error',
              duration: 5000,
              message: `${i18n.global.t('error.core')}: ${log}`,
              showClose: true,
            })
          } else if (isWarn) {
            ElMessage({
              type: 'warning',
              duration: 4000,
              message: log,
              showClose: true,
            })
          }
        }

        if (msg.obj.config) {
          this.setNewData(msg.obj)
        }
      }
      // 并行拉订阅池出站(独立 API,不阻塞主 load)
      // 失败容忍 — 没有订阅池也不影响主流程
      try {
        const po = await HttpUtils.get('api/poolOutbounds')
        if (po.success) this.poolOutbounds = po.obj || []
      } catch (e) { /* ignore */ }
    },
    setNewData(data: any) {
      this.lastLoad = Math.floor((new Date()).getTime() / 1000)
      if (data.enableTraffic) this.enableTraffic = data.enableTraffic
      if (data.config) this.config = data.config
      if (Object.hasOwn(data, 'clients')) this.clients = data.clients ?? []
      if (Object.hasOwn(data, 'inbounds')) this.inbounds = data.inbounds ?? []
      if (Object.hasOwn(data, 'outbounds')) this.outbounds = data.outbounds ?? []
      if (Object.hasOwn(data, 'endpoints')) this.endpoints = data.endpoints ?? []
      if (Object.hasOwn(data, 'tls')) this.tlsConfigs = data.tls ?? []
    },
    async loadInbounds(ids: number[]): Promise<Inbound[]> {
      const options = ids.length > 0 ? { id: ids.join(',') } : {}
      const msg = await HttpUtils.get('api/inbounds', options)
      if (msg.success) {
        return msg.obj.inbounds
      }
      return <Inbound[]>[]
    },
    async loadClients(id: number): Promise<Client> {
      const options = id > 0 ? { id: id } : {}
      const msg = await HttpUtils.get('api/clients', options)
      if (msg.success) {
        return <Client>msg.obj.clients[0] ?? {}
      }
      return <Client>{}
    },
    async save(object: string, action: string, data: any, initUsers?: number[]): Promise<boolean> {
      const postData = {
        object: object,
        action: action,
        data: JSON.stringify(data, null, 2),
        initUsers: initUsers?.join(',') ?? undefined,
      }
      const msg = await HttpUtils.post('api/save', postData)
      if (msg.success) {
        const objectName = ['tls', 'config'].includes(object) ? object : object.substring(0, object.length - 1)
        ElMessage({
          type: 'success',
          duration: 5000,
          message: `${i18n.global.t('success')}: ${i18n.global.t('actions.' + action)} ${i18n.global.t('objects.' + objectName)}`,
        })
        this.setNewData(msg.obj)
      }
      return msg.success
    },
    // Check duplicate client name
    checkClientName(id: number, newName: string): boolean {
      const oldName = id > 0 ? this.clients.findLast((i: any) => i.id == id)?.name : null
      if (newName != oldName && this.clients.findIndex((c: any) => c.name == newName) != -1) {
        ElMessage.error(`${i18n.global.t('error.dplData')}: ${i18n.global.t('client.name')}`)
        return true
      }
      return false
    },
    // Check bulk client names
    checkBulkClientNames(names: string[]): boolean {
      const newNames = new Set(names)
      const oldNames = new Set(this.clients.map((c: any) => c.name))
      const allNames = new Set([...oldNames, ...newNames])
      if (newNames.size != names.length || oldNames.size + newNames.size != allNames.size) {
        ElMessage.error(`${i18n.global.t('error.dplData')}: ${i18n.global.t('client.name')}`)
        return true
      }
      return false
    },
    // check duplicate tag
    checkTag(object: string, id: number, tag: string): boolean {
      let objects = <any[]>[]
      switch (object) {
        case 'inbound':
          objects = this.inbounds
          break
        case 'outbound':
          objects = this.outbounds
          break
        case 'endpoint':
          objects = this.endpoints
          break
        default:
          return false
      }
      const oldObject = id > 0 ? objects.findLast((i: any) => i.id == id) : null
      if (tag != oldObject?.tag && objects.findIndex((i: any) => i.tag == tag) != -1) {
        ElMessage.error(`${i18n.global.t('error.dplData')}: ${i18n.global.t('objects.tag')}`)
        return true
      }
      return false
    },
  },
})

export default Data
