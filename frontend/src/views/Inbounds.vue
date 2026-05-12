<template>
  <div class="page-container">
    <div class="page-header with-actions">
      <div class="page-header-text">
        <h2 class="page-title">{{ $t('pages.inbounds') }}</h2>
        <p class="page-desc">{{ $t('inbounds.desc', '配置 Sing-Box 入站协议、监听端口与关联用户') }}</p>
      </div>
      <div class="page-header-actions">
        <el-button type="primary" @click="showModal(0)">
          <el-icon><Plus /></el-icon>{{ $t('actions.add') }}
        </el-button>
        <el-button :loading="refreshing" @click="refresh">
          <el-icon><RefreshRight /></el-icon>{{ $t('actions.refresh', '刷新') }}
        </el-button>
        <el-button v-if="inbounds.length + totalClients > 0" type="danger" plain @click="purgeAll">
          <el-icon><Delete /></el-icon>清除全部数据
        </el-button>
      </div>
    </div>

    <!-- 防火墙告警 -->
    <el-alert
      v-if="firewall?.active && blockedPorts.length"
      type="warning"
      :closable="false"
      show-icon
      class="firewall-alert"
    >
      <template #title>
        系统防火墙({{ firewall.tool.toUpperCase() }})阻挡入站端口:{{ blockedPorts.join(', ') }}
      </template>
      <template #default>
        <div class="firewall-alert-body">
          客户端从外网连不进来。在服务器上执行:
          <code class="firewall-alert-cmd">{{ firewallFix }}</code>
        </div>
      </template>
    </el-alert>

    <!-- 概览卡 -->
    <div class="overview nc-card">
      <div class="ov-stat">
        <span class="ov-stat__label">{{ $t('pages.inbounds') }}</span>
        <span class="ov-stat__value">{{ inbounds.length }}</span>
      </div>
      <div class="ov-stat">
        <span class="ov-stat__label">{{ $t('pages.clients') }}</span>
        <span class="ov-stat__value">{{ totalClients }}</span>
      </div>
      <div class="ov-stat">
        <span class="ov-stat__label">{{ $t('home.topTraffic.up', '上行') }} / {{ $t('home.topTraffic.down', '下行') }}</span>
        <span class="ov-stat__value ov-stat__value--small">
          {{ HumanReadable.sizeFormat(totalUp) }}
          <span class="muted">/</span>
          {{ HumanReadable.sizeFormat(totalDown) }}
        </span>
      </div>
      <div v-if="totalUp + totalDown > 0" class="ov-stat">
        <span class="ov-stat__label">{{ $t('stats.totalUsage', '总流量') }}</span>
        <span class="ov-stat__value">{{ HumanReadable.sizeFormat(totalUp + totalDown) }}</span>
      </div>
      <el-tooltip content="近 1.5 秒平均上行速率 — 直读 sing-box 内存累计,无延迟" placement="top">
        <div class="ov-stat ov-stat--rate">
          <span class="ov-stat__label">↑ 上行</span>
          <span class="ov-stat__value ov-stat__value--small">{{ rateUp }}/s</span>
        </div>
      </el-tooltip>
      <el-tooltip content="近 1.5 秒平均下行速率" placement="top">
        <div class="ov-stat ov-stat--rate">
          <span class="ov-stat__label">↓ 下行</span>
          <span class="ov-stat__value ov-stat__value--small">{{ rateDown }}/s</span>
        </div>
      </el-tooltip>
      <el-tooltip content="sing-box 当前活跃连接数 — TCP / UDP" placement="top">
        <div class="ov-stat ov-stat--rate">
          <span class="ov-stat__label">TCP / UDP</span>
          <span class="ov-stat__value ov-stat__value--small mono">{{ connStats.tcp }} / {{ connStats.udp }}</span>
        </div>
      </el-tooltip>
      <div class="ov-toolbar">
        <el-input
          v-model="filter"
          :placeholder="$t('actions.search', '搜索 tag / 类型 / 端口')"
          clearable
          class="ov-search"
        >
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
      </div>
    </div>

    <div v-if="inbounds.length === 0" class="empty-state nc-card">
      <el-empty :description="$t('noData', '暂无入站')">
        <el-button type="primary" @click="showModal(0)">
          <el-icon><Plus /></el-icon>{{ $t('actions.add') }}
        </el-button>
      </el-empty>
    </div>

    <!-- 多用户入站:VLESS / VMess / Trojan / Hysteria2 / Tuic / SS-2022 / AnyTLS … -->
    <el-card v-if="multiUser.length > 0" class="cat-card">
      <template #header>
        <div class="cat-head">
          <el-icon class="cat-head__icon"><User /></el-icon>
          <span class="cat-head__title">多用户入站</span>
          <span class="cat-head__sub">VLESS / VMess / Trojan / SS-2022 等 — 一个端口多个客户</span>
          <span class="cat-head__count">{{ multiUser.length }}</span>
        </div>
      </template>
      <el-table :data="filtered(multiUser)" :row-key="ibRowKey" stripe size="small" class="nc-table ib-table">
        <el-table-column :label="$t('enable', '启用')" width="68" align="center">
          <template #default="{ row }">
            <el-switch
              :model-value="row.enable !== false"
              :loading="toggling[row.id]"
              @change="(v: boolean) => toggleEnable(row, v)"
            />
          </template>
        </el-table-column>
        <el-table-column :label="$t('type')" width="120">
          <template #default="{ row }">
            <span class="proto-pill" :class="`proto-${row.type}`">{{ row.type }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('objects.tag')" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="mono">{{ row.tag }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('in.port', '端口')" width="100" align="center">
          <template #default="{ row }">
            <span class="mono">{{ row.listen_port }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('inbounds.clientCount', '客户数')" width="100" align="center">
          <template #default="{ row }">
            <button v-if="row.users?.length" class="client-pill" @click="openClients(row)">
              <el-icon><User /></el-icon>{{ row.users.length }}
            </button>
            <span v-else class="muted">0</span>
          </template>
        </el-table-column>
        <el-table-column label="在线 IP" width="100" align="center">
          <template #default="{ row }">
            <el-popover
              v-if="ipCountOf(row.tag) > 0"
              placement="top"
              trigger="click"
              :width="280"
              @show="loadInboundIps(row.tag)"
            >
              <template #reference>
                <el-tag type="success" size="small" effect="plain" class="mono ip-clickable">
                  {{ ipCountOf(row.tag) }}
                </el-tag>
              </template>
              <div class="ip-pop">
                <div class="ip-pop__head">
                  <b>{{ row.tag }}</b> · 60s 内 {{ ipCountOf(row.tag) }} 个独立 IP
                </div>
                <div v-if="inboundIpsLoading[row.tag]" class="ip-pop__loading">加载中…</div>
                <div v-else-if="inboundIpsCache[row.tag]?.length" class="ip-pop__list mono">
                  <div v-for="ip in inboundIpsCache[row.tag]" :key="ip" class="ip-pop__item">{{ ip }}</div>
                </div>
                <div v-else class="ip-pop__empty">暂无具体 IP</div>
              </div>
            </el-popover>
            <span v-else class="muted">0</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('stats.totalUsage', '总流量')" min-width="170">
          <template #default="{ row }">
            <span v-if="trafficOf(row.tag)" class="mono traffic-cell">
              {{ HumanReadable.sizeFormat(trafficOf(row.tag)) }}
              <span class="traffic-detail">↑ {{ HumanReadable.sizeFormat(trafficByTag[row.tag]?.up ?? 0) }} / ↓ {{ HumanReadable.sizeFormat(trafficByTag[row.tag]?.down ?? 0) }}</span>
            </span>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('objects.tls')" width="68" align="center">
          <template #default="{ row }">
            <el-tag :type="row.tls_id > 0 ? 'success' : 'info'" size="small" effect="plain">
              {{ row.tls_id > 0 ? 'ON' : 'OFF' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Server 来源" width="140" align="center">
          <template #default="{ row }">
            <el-tooltip :content="addrSourceTooltip(row)" placement="top">
              <el-tag :type="addrSourceTag(row).type" size="small" effect="plain">
                {{ addrSourceTag(row).label }}
              </el-tag>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column label="中转" width="220" show-overflow-tooltip>
          <template #default="{ row }">
            <el-tag v-if="relayOf(row.tag)" type="warning" size="small" effect="plain" class="mono">
              → {{ relayOf(row.tag) }}<span v-if="relayDisplayName(row.tag)" class="relay-display-name"> · {{ relayDisplayName(row.tag) }}</span>
            </el-tag>
            <span v-else class="muted">本机出站</span>
          </template>
        </el-table-column>
        <el-table-column label="链路延迟" width="110" align="center">
          <template #default="{ row }">
            <span v-if="!relayOf(row.tag)" class="muted">—</span>
            <el-tooltip v-else
              :content="checkResults[relayOf(row.tag)]?.loading ? '探测中…' : (checkResults[relayOf(row.tag)]?.errorMessage || '点击重测中转链路')"
              placement="top"
            >
              <span class="delay-cell" :class="{ 'is-clickable': !checkResults[relayOf(row.tag)]?.loading }" @click="!checkResults[relayOf(row.tag)]?.loading && manualProbe(relayOf(row.tag))">
                <el-icon v-if="checkResults[relayOf(row.tag)]?.loading" class="is-loading"><Loading /></el-icon>
                <template v-else-if="checkResults[relayOf(row.tag)]">
                  <el-tag
                    v-if="checkResults[relayOf(row.tag)].success"
                    :type="latencyType(checkResults[relayOf(row.tag)].data?.Delay ?? 0)"
                    size="small"
                    effect="plain"
                    class="mono"
                  >
                    {{ checkResults[relayOf(row.tag)].data?.Delay }} ms
                  </el-tag>
                  <el-icon v-else style="color: var(--nc-danger)"><CircleClose /></el-icon>
                </template>
                <el-icon v-else><Stopwatch /></el-icon>
              </span>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column :label="$t('actions.action')" width="180" align="center">
          <template #default="{ row }">
            <el-button size="small" type="primary" @click="openClients(row)">
              <el-icon><User /></el-icon>客户端
            </el-button>
            <el-dropdown trigger="click" @command="(cmd: string) => onMore(cmd, row)">
              <el-button size="small">
                更多<el-icon><ArrowDown /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="edit">
                    <el-icon style="margin-right: 6px"><Edit /></el-icon>{{ $t('actions.edit') }}
                  </el-dropdown-item>
                  <el-dropdown-item command="clone">
                    <el-icon style="margin-right: 6px"><CopyDocument /></el-icon>{{ $t('actions.clone') }}
                  </el-dropdown-item>
                  <el-dropdown-item v-if="Data().enableTraffic" command="stats">
                    <el-icon style="margin-right: 6px"><DataLine /></el-icon>{{ $t('stats.graphTitle', '流量图') }}
                  </el-dropdown-item>
                  <el-dropdown-item command="reset" divided>
                    <el-icon style="margin-right: 6px"><RefreshLeft /></el-icon>{{ $t('actions.resetTraffic', '重置流量') }}
                  </el-dropdown-item>
                  <el-dropdown-item command="del" divided>
                    <el-icon style="margin-right: 6px; color: var(--nc-danger)"><Delete /></el-icon>
                    <span style="color: var(--nc-danger)">{{ $t('actions.del') }}</span>
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 单用户入站:Direct / Mixed / Socks / HTTP / Tun / Naive(无 client 维度) -->
    <el-card v-if="singleUser.length > 0" class="cat-card">
      <template #header>
        <div class="cat-head">
          <el-icon class="cat-head__icon"><Connection /></el-icon>
          <span class="cat-head__title">单用户入站</span>
          <span class="cat-head__sub">Direct / Mixed / Socks / HTTP / Tun / Naive — 端口本身就是入口</span>
          <span class="cat-head__count">{{ singleUser.length }}</span>
        </div>
      </template>
      <el-table :data="filtered(singleUser)" :row-key="ibRowKey" stripe size="small" class="nc-table ib-table">
        <el-table-column :label="$t('enable', '启用')" width="68" align="center">
          <template #default="{ row }">
            <el-switch
              :model-value="row.enable !== false"
              :loading="toggling[row.id]"
              @change="(v: boolean) => toggleEnable(row, v)"
            />
          </template>
        </el-table-column>
        <el-table-column :label="$t('type')" width="120">
          <template #default="{ row }">
            <span class="proto-pill" :class="`proto-${row.type}`">{{ row.type }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('objects.tag')" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="mono">{{ row.tag }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('in.addr', '监听')" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="mono">{{ row.listen || '0.0.0.0' }}<span class="port">:{{ row.listen_port }}</span></span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('stats.totalUsage', '总流量')" min-width="170">
          <template #default="{ row }">
            <span v-if="trafficOf(row.tag)" class="mono traffic-cell">
              {{ HumanReadable.sizeFormat(trafficOf(row.tag)) }}
              <span class="traffic-detail">↑ {{ HumanReadable.sizeFormat(trafficByTag[row.tag]?.up ?? 0) }} / ↓ {{ HumanReadable.sizeFormat(trafficByTag[row.tag]?.down ?? 0) }}</span>
            </span>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column label="Server 来源" width="140" align="center">
          <template #default="{ row }">
            <el-tooltip :content="addrSourceTooltip(row)" placement="top">
              <el-tag :type="addrSourceTag(row).type" size="small" effect="plain">
                {{ addrSourceTag(row).label }}
              </el-tag>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column label="中转" width="220" show-overflow-tooltip>
          <template #default="{ row }">
            <el-tag v-if="relayOf(row.tag)" type="warning" size="small" effect="plain" class="mono">
              → {{ relayOf(row.tag) }}<span v-if="relayDisplayName(row.tag)" class="relay-display-name"> · {{ relayDisplayName(row.tag) }}</span>
            </el-tag>
            <span v-else class="muted">本机出站</span>
          </template>
        </el-table-column>
        <el-table-column label="链路延迟" width="110" align="center">
          <template #default="{ row }">
            <span v-if="!relayOf(row.tag)" class="muted">—</span>
            <el-tooltip v-else
              :content="checkResults[relayOf(row.tag)]?.loading ? '探测中…' : (checkResults[relayOf(row.tag)]?.errorMessage || '点击重测中转链路')"
              placement="top"
            >
              <span class="delay-cell" :class="{ 'is-clickable': !checkResults[relayOf(row.tag)]?.loading }" @click="!checkResults[relayOf(row.tag)]?.loading && manualProbe(relayOf(row.tag))">
                <el-icon v-if="checkResults[relayOf(row.tag)]?.loading" class="is-loading"><Loading /></el-icon>
                <template v-else-if="checkResults[relayOf(row.tag)]">
                  <el-tag
                    v-if="checkResults[relayOf(row.tag)].success"
                    :type="latencyType(checkResults[relayOf(row.tag)].data?.Delay ?? 0)"
                    size="small"
                    effect="plain"
                    class="mono"
                  >
                    {{ checkResults[relayOf(row.tag)].data?.Delay }} ms
                  </el-tag>
                  <el-icon v-else style="color: var(--nc-danger)"><CircleClose /></el-icon>
                </template>
                <el-icon v-else><Stopwatch /></el-icon>
              </span>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column :label="$t('actions.action')" width="180" align="center">
          <template #default="{ row }">
            <!-- 单用户卡现在只剩 direct/tun/redirect/tproxy 等"端口=入口"协议,
                 没有凭证概念,主按钮就是编辑 -->
            <el-button size="small" @click="showModal(row.id)">
              <el-icon><Edit /></el-icon>编辑
            </el-button>
            <el-dropdown trigger="click" @command="(cmd: string) => onMore(cmd, row)">
              <el-button size="small">
                更多<el-icon><ArrowDown /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="clone">
                    <el-icon style="margin-right: 6px"><CopyDocument /></el-icon>{{ $t('actions.clone') }}
                  </el-dropdown-item>
                  <el-dropdown-item v-if="Data().enableTraffic" command="stats">
                    <el-icon style="margin-right: 6px"><DataLine /></el-icon>{{ $t('stats.graphTitle', '流量图') }}
                  </el-dropdown-item>
                  <el-dropdown-item command="reset" divided>
                    <el-icon style="margin-right: 6px"><RefreshLeft /></el-icon>{{ $t('actions.resetTraffic', '重置流量') }}
                  </el-dropdown-item>
                  <el-dropdown-item command="del" divided>
                    <el-icon style="margin-right: 6px; color: var(--nc-danger)"><Delete /></el-icon>
                    <span style="color: var(--nc-danger)">{{ $t('actions.del') }}</span>
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <InboundVue
      v-model="modal.visible"
      :visible="modal.visible"
      :id="modal.id"
      :inTags="inTags"
      :tlsConfigs="tlsConfigs"
      @close="closeModal"
    />
    <InboundClients
      v-if="clientsModal.visible"
      v-model="clientsModal.visible"
      :visible="clientsModal.visible"
      :inbound="clientsModal.inbound"
      @close="clientsModal.visible = false"
    />
    <Stats
      v-model="stats.visible"
      :visible="stats.visible"
      :resource="stats.resource"
      :tag="stats.tag"
      @close="closeStats"
    />
  </div>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import { Config } from '@/types/config'
import { computed, defineAsyncComponent, onBeforeUnmount, onMounted, ref } from 'vue'
import HttpUtils from '@/plugins/httputil'
import { HumanReadable } from '@/plugins/utils'
import { ElMessage, ElMessageBox } from 'element-plus'
import { i18n } from '@/locales'

const InboundVue = defineAsyncComponent(() => import('@/layouts/modals/Inbound.vue'))
const InboundClients = defineAsyncComponent(() => import('@/layouts/modals/InboundClients.vue'))
const Stats = defineAsyncComponent(() => import('@/layouts/modals/Stats.vue'))
import { createInbound, Inbound } from '@/types/inbounds'
import RandomUtil from '@/plugins/randomUtil'
import {
  Plus, Edit, Delete, CopyDocument, DataLine, RefreshRight, RefreshLeft,
  Search, User, Connection, ArrowDown, Loading, CircleClose, Stopwatch,
} from '@element-plus/icons-vue'

const appConfig = computed((): Config => <Config>Data().config)
void appConfig

const filter = ref('')

const inbounds = computed((): any[] => <any[]>(Data().inbounds ?? []))
const tlsConfigs = computed((): any[] => <any[]>(Data().tlsConfigs ?? []))

// 多用户 = 一端口多账号语义(N 组凭证可并发独立使用),包含两种凭证形态:
//  - UUID 类(vless/vmess/trojan/ss/shadowtls/hysteria/tuic/anytls):
//    凭证存 clients 表,管理走 InboundClients modal
//  - Basic Auth 类(mixed/socks/http/naive):users 是 inbound 自己的
//    [{username,password}] 数组,管理走 InboundCreds 信息卡片
// 单用户 = 端口=入口、无凭证概念(direct/tun/redirect/tproxy)
const UUID_MULTI_USER_TYPES = ['vless', 'vmess', 'trojan', 'shadowsocks', 'shadowtls', 'hysteria', 'hysteria2', 'tuic', 'anytls']
const BASIC_AUTH_TYPES = ['mixed', 'socks', 'http', 'naive']
const TRUE_MULTI_USER_TYPES = [...UUID_MULTI_USER_TYPES, ...BASIC_AUTH_TYPES]
const isBasicAuth = (type: string) => BASIC_AUTH_TYPES.includes(type)
const isMultiUserInbound = (i: any) => TRUE_MULTI_USER_TYPES.includes(i?.type) && Array.isArray(i.users)
const multiUser = computed(() => inbounds.value.filter(isMultiUserInbound))
const singleUser = computed(() => inbounds.value.filter((i: any) => !isMultiUserInbound(i)))

// el-table 用稳定 key 做 row diff,流量轮询/启用切换不会重建 DOM
const ibRowKey = (row: any) => row?.id ?? row?.tag ?? ''

// 60s 窗口内此入站的活跃 source IP 数(由 sing-box StatsTracker 上报)
const ipCountOf = (tag: string): number => Data().onlines?.inbound_ips?.[tag] ?? 0

// popover 按需拉具体 IP 列表 — 不点不发请求
const inboundIpsCache = ref<Record<string, string[]>>({})
const inboundIpsLoading = ref<Record<string, boolean>>({})
const loadInboundIps = async (tag: string) => {
  if (inboundIpsLoading.value[tag]) return
  inboundIpsLoading.value[tag] = true
  try {
    const r = await HttpUtils.get('api/onlineIps', { resource: 'inbound', tag })
    if (r.success) inboundIpsCache.value[tag] = (r.obj?.ips || []).sort()
  } finally {
    delete inboundIpsLoading.value[tag]
  }
}

// 中转目标:从 route.rules 里找带 _nb_binding 标记、inbound 包含此 tag 的规则,
// 拿出 outbound 字段。老版本残留的 action:direct binding 视为"本机出站"
// (空字符串)— 用户下次编辑保存会自动清那条规则。
const relayOf = (tag: string): string => {
  const cfg = Data().config as any
  for (const r of (cfg?.route?.rules || [])) {
    if (r?._nb_binding && Array.isArray(r.inbound) && r.inbound.includes(tag)) {
      if (r.action === 'direct') return ''
      return r.outbound || ''
    }
  }
  return ''
}

// relayDisplayName 给入站列表的「中转」列追加出站的「中转名称」(display_name)。
// 例如出站 tag="hk-relay" + display_name="香港落地" → 列里显示 "→ hk-relay · 香港落地",
// 没填中转名称时只显示 tag。
// addrSourceTag — 给入站列表「Server 来源」列出显示标签 + tag 颜色。
// 优先级:row.link_addr_source(入站级)→ effective_addr_source(后端 resolve)→ panel(兜底)
const ADDR_SRC_LABEL: Record<string, string> = {
  panel: '面板域名',
  ip: '服务器 IP',
  tls: 'TLS server_name',
}
const ADDR_SRC_TYPE: Record<string, 'success' | 'warning' | 'danger' | 'info' | 'primary'> = {
  panel: 'success',
  ip: 'warning',
  tls: 'info',
}
const addrSourceTag = (row: any): { label: string; type: 'success' | 'warning' | 'danger' | 'info' | 'primary' } => {
  const inb = (row?.link_addr_source ?? '').trim()
  const eff = (row?.effective_addr_source ?? 'panel').trim()
  if (!inb) {
    // 跟随全局 — 标签前缀 "全局·" 让用户明确这是 fallback
    return { label: '全局·' + (ADDR_SRC_LABEL[eff] ?? eff), type: 'primary' }
  }
  return { label: ADDR_SRC_LABEL[inb] ?? inb, type: ADDR_SRC_TYPE[inb] ?? 'info' }
}
const addrSourceTooltip = (row: any): string => {
  const inb = (row?.link_addr_source ?? '').trim()
  const eff = (row?.effective_addr_source ?? 'panel').trim()
  if (!inb) return `跟随全局设置(当前生效:${ADDR_SRC_LABEL[eff] ?? eff});改入站编辑里的「分享链接 server 字段来源」覆盖`
  return `本入站已单独覆盖为:${ADDR_SRC_LABEL[inb] ?? inb}`
}

// v1.7.26:tag 命名空间合并查找,先 outbounds 表(用户手配),再 pool_outbounds(订阅池)。
// 没有这一步入站列表里绑了 pool-jp 的行只会显示「→ pool-jp」,丢失中转名称。
const relayDisplayName = (inboundTag: string): string => {
  const ot = relayOf(inboundTag)
  if (!ot) return ''
  const ob = ((Data().outbounds as any[]) ?? []).find((o: any) => o?.tag === ot)
  if (ob?.display_name) return String(ob.display_name).trim()
  const po = ((Data().poolOutbounds as any[]) ?? []).find((o: any) => o?.tag === ot)
  return (po?.display_name || '').trim()
}

const filtered = (list: any[]) => {
  const q = filter.value.trim().toLowerCase()
  if (!q) return list
  return list.filter((i) =>
    (i.tag || '').toLowerCase().includes(q) ||
    (i.type || '').toLowerCase().includes(q) ||
    String(i.listen_port || '').includes(q),
  )
}

const inTags = computed((): string[] => [
  ...(inbounds.value?.map((i) => i.tag) ?? []),
  ...(Data().endpoints?.filter((e: any) => e.listen_port > 0).map((e: any) => e.tag) ?? []),
])

// 注:不再展示 onlines —— 跟出站页同样原因,onlines 是"近期有流量"不是
// 连通性指标,在机场场景下会误导(没人在用 = 离线 ≠ 不能用)。
const totalClients = computed(() => (Data().clients ?? []).length)

const totalUp = computed(() => Object.values(traffic.value).reduce((s, t) => s + (t.up || 0), 0))
const totalDown = computed(() => Object.values(traffic.value).reduce((s, t) => s + (t.down || 0), 0))

const modal = ref({ visible: false, id: 0 })
const showModal = (id: number) => { modal.value.id = id; modal.value.visible = true }
const closeModal = () => { modal.value.visible = false }

const clientsModal = ref<{ visible: boolean; inbound: any | null }>({ visible: false, inbound: null })
const openClients = (inbound: any) => {
  clientsModal.value.inbound = inbound
  clientsModal.value.visible = true
}

// 「更多」下拉的统一处理 — 收口在一个 switch 里,代码块复用
const onMore = (cmd: string, row: any) => {
  switch (cmd) {
    case 'edit':  showModal(row.id); break
    case 'clone': clone(row.id); break
    case 'stats': showStats(row.tag); break
    case 'reset': resetTraffic(row); break
    case 'del':   confirmDel(row); break
  }
}

const confirmDel = async (row: any) => {
  try {
    await ElMessageBox.confirm(
      `确认删除入站 ${row.tag}(${row.type})?引用此入站的客户会被解绑。`,
      i18n.global.t('actions.del'),
      { type: 'warning', confirmButtonText: i18n.global.t('yes'), cancelButtonText: i18n.global.t('no') },
    )
  } catch { return }
  const ok = await Data().save('inbounds', 'del', row.tag)
  if (ok) await cleanupInboundBinding(row.tag)
}

// 删除入站时,把 route.rules 里由「默认出站」字段写入的 binding(_nb_binding 标记)
// 一起清掉,免得变成"指向不存在 inbound tag"的悬空规则导致 sing-box 启动失败。
const cleanupInboundBinding = async (tag: string) => {
  const cfg = JSON.parse(JSON.stringify(Data().config || {}))
  const rules = cfg?.route?.rules
  if (!Array.isArray(rules) || rules.length === 0) return
  const filtered = rules.filter((r: any) => {
    if (!r?._nb_binding) return true
    if (Array.isArray(r.inbound) && r.inbound.includes(tag)) return false
    return true
  })
  if (filtered.length !== rules.length) {
    cfg.route.rules = filtered
    await Data().save('config', 'set', cfg)
  }
}

const purgeAll = async () => {
  const ibCount = inbounds.value.length
  const cliCount = (Data().clients ?? []).length
  try {
    await ElMessageBox.confirm(
      `确认清除全部 ${ibCount} 个入站 + ${cliCount} 个客户端?\n` +
      `sing-box 重启后所有连接立即失效,流量历史一并删除,无法恢复。`,
      '清除全部数据',
      { type: 'error', confirmButtonText: '确认清除', cancelButtonText: i18n.global.t('no') },
    )
  } catch { return }

  // 先删客户端 — 客户端被入站引用,先解绑再删入站可避免一致性中间态被 sing-box
  // 短暂下发(虽然事务包裹,但保险起见)。两段都是按 id/tag 串行调 del。
  for (const c of [...(Data().clients ?? [])]) {
    await Data().save('clients', 'del', (c as any).id)
  }
  for (const i of [...inbounds.value]) {
    await Data().save('inbounds', 'del', (i as any).tag)
  }

  // 同时把流量样本一起清掉 — 上面的 del 不会主动清 stats 表,
  // 不清的话"总流量"列还会保留旧 tag 的数字
  for (const tag of Object.keys(traffic.value)) {
    await HttpUtils.post('api/resetTraffic', { resource: 'inbound', tag })
  }
  await loadTraffic()

  ElMessage.success(`已清除 ${ibCount} 个入站 + ${cliCount} 个客户端`)
}

const resetTraffic = async (row: any) => {
  try {
    await ElMessageBox.confirm(
      `重置入站「${row.tag}」的累计流量?(关联客户的 per-user 流量不受影响)`,
      i18n.global.t('actions.resetTraffic', '重置流量'),
      { type: 'warning', confirmButtonText: '重置', cancelButtonText: i18n.global.t('no') },
    )
  } catch { return }
  const r = await HttpUtils.post('api/resetTraffic', { resource: 'inbound', tag: row.tag })
  if (r.success) {
    ElMessage.success('已重置')
    await loadTraffic()
  }
}

const cloneLoading = ref(false)
const clone = async (id: number) => {
  cloneLoading.value = true
  try {
    const inboundArray = await Data().loadInbounds([id])
    const inbound = inboundArray[0]
    const newTag = inbound.type + '-' + RandomUtil.randomSeq(3)
    const newInbound = createInbound(inbound.type, {
      ...inbound,
      id: 0,
      tag: newTag,
      listen_port: RandomUtil.randomIntRange(10000, 60000),
    })
    await Data().save('inbounds', 'new', newInbound)
  } finally {
    cloneLoading.value = false
  }
}

const refreshing = ref(false)
const refresh = async () => {
  refreshing.value = true
  try {
    Data().lastLoad = 0
    await Promise.all([Data().loadData(), loadTraffic(), loadFirewall()])
  } finally {
    refreshing.value = false
  }
}

const stats = ref({ visible: false, resource: 'inbound', tag: '' })
const showStats = (tag: string) => { stats.value.tag = tag; stats.value.visible = true }
const closeStats = () => { stats.value.visible = false }

// ---------- 启用/禁用 ----------
const toggling = ref<Record<number, boolean>>({})
const toggleEnable = async (row: any, v: boolean) => {
  // 乐观更新(同 InboundClients):立即把 row.enable=v,switch 视觉马上锁定。
  // 否则 await save 期间 :model-value 没变,EP 会把 switch 视觉回退,用户
  // 体感"开关切了又自己跳回去"。失败时回滚。
  const original = row.enable
  row.enable = v
  toggling.value[row.id] = true
  try {
    const inboundArray = await Data().loadInbounds([row.id])
    const inbound = inboundArray[0]
    if (!inbound) {
      row.enable = original
      return
    }
    inbound.enable = v
    const ok = await Data().save('inbounds', 'edit', inbound)
    if (!ok) row.enable = original
  } finally {
    delete toggling.value[row.id]
  }
}

// ---------- 总流量 ----------
const traffic = ref<Record<string, { up: number; down: number }>>({})
// 入站流量 = 本入站所有绑定 client 的 up+down 之和 — 单 client 时跟 client
// 流量数字完全一致(用户期待);多 client 时是各 client 累加。
// 不用 stats 表 SUM(那是 sing-box 内部分桶,会包含已删 client / sentinel
// 阶段的样本,跟 client 总和对不齐,引发"为什么差几十 MB"的疑惑)。
const trafficByTag = computed<Record<string, { up: number; down: number }>>(() => {
  const idToTag: Record<number, string> = {}
  for (const ib of (Data().inbounds || [])) {
    if (ib?.id != null && ib?.tag) idToTag[ib.id] = ib.tag
  }
  const out: Record<string, { up: number; down: number }> = {}
  for (const c of (Data().clients || [])) {
    const ids: number[] = Array.isArray(c?.inbounds) ? c.inbounds
      : (typeof c?.inbounds === 'string' ? JSON.parse(c.inbounds || '[]') : [])
    const up = Number(c?.up || 0) + Number(c?.totalUp || 0)
    const down = Number(c?.down || 0) + Number(c?.totalDown || 0)
    for (const id of ids) {
      const tag = idToTag[id]
      if (!tag) continue
      const cur = out[tag] || { up: 0, down: 0 }
      cur.up += up
      cur.down += down
      out[tag] = cur
    }
  }
  return out
})
const trafficOf = (tag: string) => {
  const t = trafficByTag.value[tag]
  if (!t) return 0
  return (t.up || 0) + (t.down || 0)
}
// 速率采样:每次 loadTraffic 后跟上次的总和比对,算 KB/s。
// 第一次没有 baseline 显示 0,从第二次开始有真实速率。
const lastSample = ref<{ up: number; down: number; ts: number } | null>(null)
const rateUp = ref('0 B')
const rateDown = ref('0 B')

// fmtRate: sizeFormat 在 0/负数时返 "-",但速率为 0 应该显示 "0 B" 让用户
// 看到"在测但当前没流量",不要显示 "-/s" 让人以为坏了。
const fmtRate = (n: number) => {
  if (!n || n < 0 || !isFinite(n)) return '0 B'
  return HumanReadable.sizeFormat(n)
}

const loadTraffic = async () => {
  // 用 liveTotals(直读 sing-box 内存累计 — 单调递增,1.5s diff 永远 > 0
  // 当有流量时);列表行的"已用流量"仍走 statsTotals 拿 DB 累计。
  const [live, dbTotals] = await Promise.all([
    HttpUtils.get('api/liveTotals', { resource: 'inbound' }),
    HttpUtils.get('api/statsTotals', { resource: 'inbound' }),
  ])
  if (dbTotals.success) {
    const incoming = dbTotals.obj || {}
    for (const k of Object.keys(traffic.value)) {
      if (!(k in incoming)) delete traffic.value[k]
    }
    Object.assign(traffic.value, incoming)
  }

  // 算速率 — 基于 live(无周期延迟)
  if (live.success && live.obj) {
    const liveMap: Record<string, { up: number; down: number }> = live.obj
    const nowUp = Object.values(liveMap).reduce((s, t) => s + (t.up || 0), 0)
    const nowDown = Object.values(liveMap).reduce((s, t) => s + (t.down || 0), 0)
    const nowTs = Date.now()
    if (lastSample.value && nowTs > lastSample.value.ts) {
      const dt = (nowTs - lastSample.value.ts) / 1000
      const dUp = Math.max(0, nowUp - lastSample.value.up) / dt
      const dDown = Math.max(0, nowDown - lastSample.value.down) / dt
      rateUp.value = fmtRate(dUp)
      rateDown.value = fmtRate(dDown)
    }
    lastSample.value = { up: nowUp, down: nowDown, ts: nowTs }
  }
}

// 当前活跃连接数 — 5s 拉一次(开销小,实时性强)
const connStats = ref({ tcp: 0, udp: 0 })
const loadConnStats = async () => {
  const r = await HttpUtils.get('api/connStats')
  if (r.success && r.obj) connStats.value = { tcp: r.obj.tcp || 0, udp: r.obj.udp || 0 }
}

// ---------- 链路延迟探测(只对绑了中转的入站,跨入站按 outbound tag 去重) ----------
interface CheckResult {
  loading?: boolean
  success: boolean
  data?: { OK?: boolean; Delay?: number; Error?: string } | null
  errorMessage?: string
}
const checkResults = ref<Record<string, CheckResult>>({})
const setCheckResult = (tag: string, r: CheckResult) => { checkResults.value[tag] = r }
const probeOutbound = async (tag: string) => {
  if (!tag) return
  setCheckResult(tag, { loading: true, success: false })
  const msg = await HttpUtils.get('api/checkOutbound', { tag })
  const success = msg.success && msg.obj?.OK
  const errorMessage = success ? undefined : (msg.obj?.Error ?? msg.msg ?? '')
  setCheckResult(tag, { loading: false, success, data: msg.obj ?? null, errorMessage })
}
const manualProbe = (tag: string) => {
  if (!tag) return
  if (checkResults.value[tag]?.loading) return
  probeOutbound(tag)
}
const latencyType = (ms: number): 'success' | 'warning' | 'danger' => {
  if (ms < 200) return 'success'
  if (ms < 600) return 'warning'
  return 'danger'
}

const PROBE_INTERVAL_MS = 10000
const PROBE_CONCURRENCY = 4
let probeRunning = false
let probeTimer: any
const probeOnce = async () => {
  if (probeRunning) return
  probeRunning = true
  try {
    // 收集所有"中转目标 outbound tag"(去重),只测在用的
    const targets = new Set<string>()
    for (const i of inbounds.value) {
      const r = relayOf((i as any).tag)
      if (r && !checkResults.value[r]?.loading) targets.add(r)
    }
    if (targets.size === 0) return
    const arr = [...targets]
    let idx = 0
    const next = async () => {
      while (idx < arr.length) await probeOutbound(arr[idx++])
    }
    await Promise.all(Array.from({ length: Math.min(PROBE_CONCURRENCY, arr.length) }, () => next()))
  } finally {
    probeRunning = false
  }
}

// ---------- 防火墙状态 ----------
interface FirewallStatus {
  active: boolean
  tool: string
  openPorts: number[] | null
  openRanges: { lo: number; hi: number }[] | null
}
const firewall = ref<FirewallStatus | null>(null)
const blockedPorts = computed<number[]>(() => {
  const fw = firewall.value
  if (!fw || !fw.active) return []
  const isOpen = (port: number): boolean => {
    if ((fw.openPorts || []).includes(port)) return true
    for (const r of fw.openRanges || []) {
      if (port >= r.lo && port <= r.hi) return true
    }
    return false
  }
  const blocked = new Set<number>()
  for (const i of inbounds.value) {
    const p = (i as any).listen_port
    if (typeof p === 'number' && (i as any).enable !== false && !isOpen(p)) blocked.add(p)
  }
  return [...blocked].sort((a, b) => a - b)
})
const firewallFix = computed(() => {
  const fw = firewall.value
  if (!fw) return ''
  const ports = blockedPorts.value
  if (fw.tool === 'firewalld') {
    return ports.map((p) => `firewall-cmd --permanent --add-port=${p}/tcp`).join(' && ') + ' && firewall-cmd --reload'
  }
  return ports.map((p) => `ufw allow ${p}/tcp`).join(' && ')
})
const loadFirewall = async () => {
  const r = await HttpUtils.get('api/firewallStatus')
  if (r.success) firewall.value = r.obj
}

// ---------- lifecycle ----------
let trafficTimer: any
let connTimer: any
onMounted(async () => {
  await Promise.all([loadTraffic(), loadFirewall(), loadConnStats()])
  // 流量 + TCP/UDP 都 1.5s 一次 — 用户要看实时网速;后端两个端点都很轻
  // (流量是内存快照,connStats 读 /proc/net,几 ms 完成),1.5s 不会压到 panel
  trafficTimer = setInterval(() => { if (!document.hidden) loadTraffic() }, 1500)
  connTimer = setInterval(() => { if (!document.hidden) loadConnStats() }, 1500)
  // 进页面立刻先来一轮链路延迟探测,免得用户干等 10s 才有数据
  probeOnce()
  probeTimer = setInterval(() => { if (!document.hidden) probeOnce() }, PROBE_INTERVAL_MS)
})
onBeforeUnmount(() => {
  if (trafficTimer) clearInterval(trafficTimer)
  if (connTimer) clearInterval(connTimer)
  if (probeTimer) clearInterval(probeTimer)
})
</script>

<style scoped>
.overview {
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 14px 18px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.ov-stat {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 60px;
}
.ov-stat__label {
  font-size: 11px;
  color: var(--nc-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.ov-stat__value {
  font-family: var(--font-display);
  font-size: 20px;
  font-weight: 700;
  color: var(--nc-text-1);
}
.ov-stat__value--small { font-size: 14px; font-weight: 600; }
.ov-stat--ok .ov-stat__value { color: var(--nc-success); }
.ov-toolbar { margin-left: auto; }
.ov-search { width: 240px; max-width: 100%; }

.cat-card {
  margin-bottom: 12px;
  border: 1px solid var(--nc-border);
  border-radius: var(--radius-lg);
}
.cat-card :deep(.el-card__header) {
  padding: 12px 16px;
  background: #f8fafc;
  border-bottom: 1px solid var(--nc-border);
}
.cat-card :deep(.el-card__body) { padding: 0; }

.cat-head {
  display: flex;
  align-items: center;
  gap: 10px;
}
.cat-head__icon { color: var(--nc-primary); font-size: 16px; }
.cat-head__title { font-size: 13.5px; font-weight: 600; color: var(--nc-text-1); }
.cat-head__sub { font-size: 12px; color: var(--nc-text-muted); flex: 1; }
.cat-head__count {
  font-family: var(--font-display);
  font-size: 12px;
  font-weight: 600;
  color: var(--nc-primary);
  background: var(--nc-primary-soft);
  padding: 2px 10px;
  border-radius: var(--radius-pill);
}

.ib-table { background: var(--nc-surface); }
.ib-table .mono { color: var(--nc-text-1); font-weight: 500; }
.ib-table .port { color: var(--nc-text-muted); font-weight: 400; }

.proto-pill {
  display: inline-block;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: var(--radius-pill);
  letter-spacing: 0.04em;
  text-transform: uppercase;
  background: var(--nc-primary-soft);
  color: var(--nc-primary);
}
.proto-pill.proto-direct,
.proto-pill.proto-mixed   { color: #475569; background: #e2e8f0; }
.proto-pill.proto-vless,
.proto-pill.proto-vmess   { color: #2563eb; background: #dbeafe; }
.proto-pill.proto-trojan  { color: #d97706; background: #fef3c7; }
.proto-pill.proto-shadowsocks { color: #16a34a; background: #dcfce7; }
.proto-pill.proto-hysteria,
.proto-pill.proto-hysteria2 { color: #7c3aed; background: #ede9fe; }
.proto-pill.proto-tuic    { color: #0d9488; background: #ccfbf1; }
.proto-pill.proto-naive   { color: #db2777; background: #fce7f3; }
.proto-pill.proto-anytls  { color: #4f46e5; background: #e0e7ff; }
.proto-pill.proto-tun     { color: #0891b2; background: #cffafe; }
.proto-pill.proto-socks,
.proto-pill.proto-http    { color: #0ea5e9; background: #e0f2fe; }
.proto-pill.proto-shadowtls { color: #d97706; background: #fef3c7; }

.client-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: var(--nc-primary-soft);
  color: var(--nc-primary);
  border: none;
  padding: 2px 10px;
  border-radius: var(--radius-pill);
  font-family: var(--font-mono);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: background var(--t-fast);
}
.client-pill:hover { background: var(--nc-primary); color: #fff; }

.muted { color: var(--nc-text-faint); }
.relay-display-name { color: var(--nc-text-muted); font-weight: 400; }

.status-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--nc-text-faint);
}
.status-dot.online {
  background: var(--nc-success);
  box-shadow: 0 0 0 2px rgba(34, 197, 94, 0.18);
}

.empty-state { padding: 40px 16px; }

.firewall-alert { margin-bottom: 12px; }
.firewall-alert-body { margin-top: 4px; font-size: 12.5px; }
.firewall-alert-cmd {
  display: inline-block;
  margin-left: 6px;
  padding: 2px 8px;
  background: rgba(0, 0, 0, 0.06);
  border-radius: 4px;
  font-family: var(--font-mono);
  font-size: 12px;
  word-break: break-all;
}

.traffic-cell {
  display: inline-flex;
  align-items: baseline;
  gap: 8px;
}
.traffic-detail {
  font-size: 11px;
  color: var(--nc-text-muted);
  font-weight: 400;
}

/* 在线 IP popover */
.ip-clickable { cursor: pointer; transition: transform 0.1s; }
.ip-clickable:hover { transform: scale(1.05); }
.ip-pop { display: flex; flex-direction: column; gap: 8px; }
.ip-pop__head { font-size: 12.5px; color: var(--nc-text-1); }
.ip-pop__loading, .ip-pop__empty { font-size: 12px; color: var(--nc-text-muted); padding: 8px 0; text-align: center; }
.ip-pop__list { max-height: 200px; overflow-y: auto; border: 1px solid var(--nc-border-soft); border-radius: var(--radius-sm); padding: 6px 8px; }
.ip-pop__item { font-size: 12px; line-height: 1.7; color: var(--nc-text-1); }

/* 链路延迟 cell:整格可点触发重测,跟出站页样式一致 */
.delay-cell {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 56px;
  height: 24px;
  padding: 0 4px;
  border-radius: var(--radius-sm);
  color: var(--nc-text-muted);
}
.delay-cell.is-clickable { cursor: pointer; transition: background-color 0.12s; }
.delay-cell.is-clickable:hover { background-color: var(--nc-primary-soft); color: var(--nc-primary); }
</style>
