<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="closeModal"
    @opened="updateData(id)"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="$t('actions.' + title) + ' ' + $t('objects.inbound')"
    destroy-on-close
  >
    <div v-if="loading" class="modal-loading">
      <el-icon class="is-loading"><Loading /></el-icon>
    </div>

    <el-form v-else label-position="top" class="ib-form">
      <!-- 基础四件套 -->
      <div class="form-grid">
        <el-form-item :label="$t('type')" class="form-item--full">
          <div class="type-picker">
            <div class="type-group">
              <span class="type-group__label">
                <span class="type-cap cap-multi">多用户</span>
                一端口多客户(独立凭据 / 流量 / 到期)
              </span>
              <div class="type-chips">
                <button
                  v-for="t in MULTI_USER_TYPES"
                  :key="t.value"
                  type="button"
                  class="type-chip"
                  :class="{ 'is-active': inbound.type === t.value, 'is-multi': true }"
                  @click="selectType(t.value)"
                >{{ t.label }}</button>
              </div>
            </div>
            <div class="type-group">
              <span class="type-group__label">
                <span class="type-cap cap-single">单用户</span>
                端口即入口(SOCKS/HTTP 本地代理 · Direct 端口转发 · Tun 网卡级代理)
              </span>
              <div class="type-chips">
                <button
                  v-for="t in SINGLE_USER_TYPES"
                  :key="t.value"
                  type="button"
                  class="type-chip"
                  :class="{ 'is-active': inbound.type === t.value, 'is-single': true }"
                  @click="selectType(t.value)"
                >{{ t.label }}</button>
              </div>
            </div>
          </div>
        </el-form-item>
        <div class="form-row form-item--full">
          <el-form-item :label="$t('objects.tag')" class="form-row__item">
            <el-input v-model="inbound.tag" :disabled="title === 'edit'" placeholder="字母数字 . _ -" />
          </el-form-item>
          <template v-if="inbound.type !== InTypes.Tun">
            <el-form-item :label="$t('in.addr')" class="form-row__item">
              <el-input v-model="inbound.listen" placeholder="::  (全部接口)" />
            </el-form-item>
            <el-form-item :label="$t('in.port')" class="form-row__item">
              <div class="port-input">
                <el-input-number
                  v-model="inbound.listen_port"
                  :min="1"
                  :max="65535"
                  :controls="false"
                  class="port-input__num"
                  :class="{ 'is-conflict': !!portConflict }"
                />
                <el-tooltip content="重抽空闲端口" placement="top">
                  <button type="button" class="port-input__btn" @click="reseedPort">
                    <el-icon><Refresh /></el-icon>
                  </button>
                </el-tooltip>
              </div>
              <p v-if="portConflict" class="form-warn">端口 {{ inbound.listen_port }} 已被入站「{{ portConflict }}」占用</p>
            </el-form-item>
          </template>
        </div>
        <el-form-item v-if="hasTls" :label="$t('objects.tls')">
          <div class="tls-row">
            <el-select v-model="inbound.tls_id" clearable :placeholder="onlyTls ? '此协议必须启用 TLS' : $t('disable')" class="tls-row__select">
              <el-option :value="0" :label="$t('disable')" :disabled="onlyTls" />
              <el-option v-for="t in tlsConfigs" :key="t.id" :value="t.id" :label="t.name" />
            </el-select>
            <el-tooltip content="通过 Cloudflare 自动签发新证书" placement="top">
              <el-button type="primary" plain class="tls-row__btn" @click="openCfWizard">
                <el-icon><MagicStick /></el-icon>自动签发
              </el-button>
            </el-tooltip>
          </div>
          <p v-if="onlyTls && !inbound.tls_id" class="form-warn">{{ inbound.type }} 协议必须配置 TLS,点「自动签发」一键生成或去 TLS 设置页创建。</p>
        </el-form-item>
        <!-- 中转目标(虚拟字段 — 实际写入 route.rules 一条 binding) -->
        <el-form-item label="中转目标(可选)">
          <el-select v-model="defaultOutbound" filterable placeholder="本机按全局路由出公网(不中转)" style="width: 100%">
            <el-option value="inherit" label="本机按全局路由出公网(不中转)" />
            <el-option-group v-if="proxyOutbounds.length" label="转发到落地节点">
              <el-option
                v-for="o in proxyOutbounds"
                :key="o.tag"
                :value="o.tag"
                :label="o.display_name ? `→ ${o.tag} · ${o.display_name}` : `→ ${o.tag}`"
              />
            </el-option-group>
            <el-option-group v-if="endpointOutbounds.length" label="转发到虚拟网卡端点">
              <el-option v-for="e in endpointOutbounds" :key="e.tag" :value="e.tag">
                <span>→ {{ e.tag }}</span>
                <span v-if="endpointPurpose(e.type)" class="opt-suffix">{{ endpointPurpose(e.type) }}</span>
              </el-option>
            </el-option-group>
          </el-select>
          <p class="form-hint">
            选了落地出站 = <b>中转模式</b>:用户 → 本机入站 → <span class="mono">{{ defaultOutbound !== 'inherit' && defaultOutbound !== 'direct' ? defaultOutbound : '...' }}</span> → 公网。
            本机不落地,流量转给落地节点出公网,常用于前置加速 / 隐藏落地 IP。
            底层是在 route.rules 最前插一条 <span class="mono">{ inbound: ["{{ inbound.tag || '...' }}"], outbound: "..." }</span>,改名 / 删除入站自动同步。
          </p>
        </el-form-item>
      </div>

      <!-- 协议专属字段(凭据 / 参数) -->
      <div v-if="hasProtocolFields" class="form-section">
        <h4 class="form-section__title">协议参数</h4>
        <div class="form-grid">
          <!-- Shadowsocks 单用户/多用户 -->
          <template v-if="inbound.type === 'shadowsocks'">
            <el-form-item label="加密方法">
              <el-select v-model="inbound.method" filterable @change="onSsMethodChange">
                <el-option v-for="m in SS_METHODS" :key="m" :label="m" :value="m" />
              </el-select>
            </el-form-item>
            <el-form-item :label="ssIs2022 ? '密钥 (Server PSK · base64)' : '密码'">
              <el-input v-model="inbound.password" type="password" show-password autocomplete="new-password" class="mono">
                <template #append>
                  <el-tooltip
                    :content="ssIs2022 ? `生成 ${ssPskBytes} 字节随机 PSK(base64)` : '生成随机密码'"
                    placement="top"
                  >
                    <el-button @click="genSsPassword"><el-icon><Refresh /></el-icon></el-button>
                  </el-tooltip>
                </template>
              </el-input>
              <p v-if="ssIs2022" class="form-hint">2022 系算法的 password 是 base64 编码的 PSK,长度必须 = {{ ssPskBytes }} 字节(对应 base64 字符串 24 / 44 长度)。点 🔄 自动生成。</p>
            </el-form-item>
            <el-form-item label="网络">
              <el-select v-model="inbound.network" clearable placeholder="tcp + udp">
                <el-option label="tcp" value="tcp" />
                <el-option label="udp" value="udp" />
              </el-select>
            </el-form-item>
          </template>

          <!-- ShadowTLS -->
          <template v-if="inbound.type === 'shadowtls'">
            <el-form-item label="版本">
              <el-select v-model="inbound.version">
                <el-option :label="3" :value="3" />
                <el-option :label="2" :value="2" />
                <el-option :label="1" :value="1" />
              </el-select>
            </el-form-item>
            <el-form-item v-if="inbound.version < 3" label="密码">
              <el-input v-model="inbound.password" type="password" show-password />
            </el-form-item>
            <el-form-item label="握手目标 server">
              <el-input v-model="inbound.handshake.server" placeholder="例 cloud.tencent.com" />
            </el-form-item>
            <el-form-item label="握手目标端口">
              <el-input-number v-model="inbound.handshake.server_port" :min="1" :max="65535" controls-position="right" style="width: 100%" />
            </el-form-item>
          </template>

          <!-- Hysteria(v1) -->
          <template v-if="inbound.type === 'hysteria'">
            <el-form-item label="上行 Mbps">
              <el-input-number v-model="inbound.up_mbps" :min="1" controls-position="right" style="width: 100%" />
            </el-form-item>
            <el-form-item label="下行 Mbps">
              <el-input-number v-model="inbound.down_mbps" :min="1" controls-position="right" style="width: 100%" />
            </el-form-item>
            <el-form-item label="混淆密码(obfs)">
              <el-input v-model="inbound.obfs" placeholder="可选,留空 = 不混淆" />
            </el-form-item>
          </template>

          <!-- Hysteria2 -->
          <template v-if="inbound.type === 'hysteria2'">
            <el-form-item label="上行 Mbps(0 = 不限速)">
              <el-input-number v-model="inbound.up_mbps" :min="0" controls-position="right" style="width: 100%" />
            </el-form-item>
            <el-form-item label="下行 Mbps(0 = 不限速)">
              <el-input-number v-model="inbound.down_mbps" :min="0" controls-position="right" style="width: 100%" />
            </el-form-item>
            <el-form-item label="混淆密码(salamander)">
              <el-input :model-value="inbound.obfs?.password ?? ''" placeholder="可选" @input="(v: string) => setHy2Obfs(v)" />
            </el-form-item>
            <el-form-item label="忽略客户端带宽申报">
              <el-switch v-model="inbound.ignore_client_bandwidth" />
            </el-form-item>
          </template>

          <!-- Tuic -->
          <template v-if="inbound.type === 'tuic'">
            <el-form-item label="拥塞控制">
              <el-select v-model="inbound.congestion_control">
                <el-option label="cubic" value="cubic" />
                <el-option label="new_reno" value="new_reno" />
                <el-option label="bbr" value="bbr" />
              </el-select>
            </el-form-item>
            <el-form-item label="鉴权超时">
              <el-input v-model="inbound.auth_timeout" placeholder="3s" />
            </el-form-item>
            <el-form-item label="心跳间隔">
              <el-input v-model="inbound.heartbeat" placeholder="10s" />
            </el-form-item>
            <el-form-item label="0-RTT 握手">
              <el-switch v-model="inbound.zero_rtt_handshake" />
            </el-form-item>
          </template>

          <!-- Naive -->
          <template v-if="inbound.type === 'naive'">
            <el-form-item label="QUIC 拥塞控制">
              <el-select v-model="inbound.quic_congestion_control" clearable placeholder="默认 cubic">
                <el-option label="cubic" value="cubic" />
                <el-option label="bbr" value="bbr" />
                <el-option label="bbr2" value="bbr2" />
                <el-option label="reno" value="reno" />
              </el-select>
            </el-form-item>
          </template>

          <!-- Direct(端口转发到固定上游) -->
          <template v-if="inbound.type === 'direct'">
            <el-form-item label="覆盖目标地址">
              <el-input v-model="inbound.override_address" placeholder="可选,所有连接转到此地址" />
            </el-form-item>
            <el-form-item label="覆盖目标端口">
              <el-input-number v-model="inbound.override_port" :min="0" :max="65535" controls-position="right" style="width: 100%" />
            </el-form-item>
            <el-form-item label="网络">
              <el-select v-model="inbound.network" clearable placeholder="tcp + udp">
                <el-option label="tcp" value="tcp" />
                <el-option label="udp" value="udp" />
              </el-select>
            </el-form-item>
          </template>

          <!-- TUN(网卡级代理) -->
          <template v-if="inbound.type === 'tun'">
            <el-form-item label="接口地址">
              <el-select
                v-model="inbound.address"
                multiple
                filterable
                allow-create
                default-first-option
                :reserve-keyword="false"
                placeholder="如 172.19.0.1/30 fdfe:dcba:9876::1/126(回车确认每条)"
                class="mono"
              />
              <div style="font-size:12px;color:var(--el-text-color-secondary);margin-top:4px;">
                必填:tun 网卡的 IPv4 / IPv6 CIDR;sing-box 启动会校验,缺这个直接拒绝
              </div>
            </el-form-item>
            <el-form-item label="网卡名称">
              <el-input v-model="inbound.interface_name" placeholder="自动 = 留空" class="mono" />
            </el-form-item>
            <el-form-item label="MTU">
              <el-input-number v-model="inbound.mtu" :min="576" :max="65535" controls-position="right" style="width: 100%" />
            </el-form-item>
            <el-form-item label="协议栈">
              <el-select v-model="inbound.stack">
                <el-option label="system" value="system" />
                <el-option label="gvisor" value="gvisor" />
                <el-option label="mixed" value="mixed" />
              </el-select>
            </el-form-item>
            <el-form-item label="自动路由">
              <el-switch v-model="inbound.auto_route" />
            </el-form-item>
            <el-form-item v-if="inbound.auto_route" label="严格路由">
              <el-switch v-model="inbound.strict_route" />
            </el-form-item>
            <el-form-item label="UDP 超时">
              <el-input v-model="inbound.udp_timeout" placeholder="5m" />
            </el-form-item>
          </template>

          <!-- TProxy -->
          <el-form-item v-if="inbound.type === 'tproxy'" label="网络">
            <el-select v-model="inbound.network" clearable placeholder="tcp + udp">
              <el-option label="tcp" value="tcp" />
              <el-option label="udp" value="udp" />
            </el-select>
          </el-form-item>
        </div>
      </div>

      <!-- 认证用户:mixed/socks/http/naive 跟 vless/vmess/trojan 一样,
           凭证已统一走 clients 表 + InboundClients modal 管理(创建客户端时
           会自动按入站协议生成 username/password)。这里不再嵌入 inbound
           编辑器,免双入口写同一份数据。 -->
      <el-alert
        v-if="hasAuthUsers"
        type="info"
        :closable="false"
        show-icon
        class="auth-redirect-alert"
      >
        <template #title>
          <b>{{ inbound.type }}</b> 是多账号协议 — 凭证(username / password)在「客户端」管理。
          保存入站后到列表行点「客户端」按钮添加即可,创建时会按 mixed/socks/http 自动生成账号密码。
        </template>
      </el-alert>

      <!-- Transport(VLESS / VMess / Trojan) -->
      <div v-if="supportsTransport" class="form-section">
        <div class="form-section__head">
          <h4 class="form-section__title">传输层(Transport)</h4>
          <el-select v-model="transportType" size="small" style="width: 140px">
            <el-option label="(裸 TCP)" value="" />
            <el-option v-for="(v, k) in TrspTypes" :key="k" :label="k" :value="v" />
          </el-select>
        </div>

        <!-- 协议推荐套餐 — 一键应用 transport + TLS 组合 -->
        <div class="presets">
          <span class="presets__label">推荐:</span>
          <button
            v-for="p in transportPresets"
            :key="p.key"
            type="button"
            class="preset-chip"
            :class="{ 'is-active': isPresetActive(p) }"
            @click="applyPreset(p)"
          >
            <span class="preset-chip__title">{{ p.title }}</span>
            <span class="preset-chip__desc">{{ p.desc }}</span>
          </button>
        </div>
        <div v-if="transportType" class="form-grid">
          <template v-if="transportType === 'ws'">
            <el-form-item label="Path">
              <el-input v-model="inbound.transport.path" placeholder="/" class="mono" />
            </el-form-item>
            <el-form-item label="Host(WS Header,客户端验证用)">
              <el-input :model-value="(inbound.transport.headers || {}).Host" @input="(v: string) => inbound.transport.headers = v ? { Host: v } : undefined" />
            </el-form-item>
          </template>
          <template v-else-if="transportType === 'grpc'">
            <el-form-item label="serviceName">
              <el-input v-model="inbound.transport.service_name" class="mono" />
            </el-form-item>
          </template>
          <template v-else-if="transportType === 'http'">
            <el-form-item label="Path">
              <el-input v-model="inbound.transport.path" placeholder="/" class="mono" />
            </el-form-item>
            <el-form-item label="Host(逗号分隔)">
              <el-input :model-value="(inbound.transport.host || []).join(',')" @input="(v: string) => inbound.transport.host = v ? v.split(',').map((x: string) => x.trim()) : []" />
            </el-form-item>
          </template>
          <template v-else-if="transportType === 'httpupgrade'">
            <el-form-item label="Path">
              <el-input v-model="inbound.transport.path" placeholder="/" class="mono" />
            </el-form-item>
            <el-form-item label="Host">
              <el-input v-model="inbound.transport.host" />
            </el-form-item>
          </template>
        </div>
      </div>

      <!-- 高级:完整 JSON -->
      <details class="advanced">
        <summary>
          <span>高级:完整 JSON</span>
          <span class="hint-muted">— 上面字段覆盖不到的字段(fallback / multiplex / advanced TLS)直接改这里</span>
        </summary>
        <div class="advanced__body">
          <div class="advanced__head">
            <el-tooltip content="从上面字段重新生成" placement="top">
              <el-button text @click="syncFromJson"><el-icon><RefreshRight /></el-icon></el-button>
            </el-tooltip>
          </div>
          <el-input
            v-model="inboundJson"
            type="textarea"
            :rows="12"
            spellcheck="false"
            class="json-editor mono-input"
            @change="onJsonEdit"
          />
          <p v-if="jsonError" class="json-error">{{ jsonError }}</p>
        </div>
      </details>
    </el-form>

    <template #footer>
      <el-button @click="closeModal">{{ $t('actions.close') }}</el-button>
      <el-button type="primary" :loading="loading" :disabled="!validate" @click="saveChanges">
        {{ $t('actions.save') }}
      </el-button>
    </template>

    <CloudflareTls
      v-if="cfWizardVisible"
      v-model="cfWizardVisible"
      :visible="cfWizardVisible"
      @created="onCfTlsCreated"
      @close="cfWizardVisible = false"
    />
  </el-dialog>
</template>

<script lang="ts" setup>
import { computed, defineAsyncComponent, ref, watch } from 'vue'
import { InTypes, createInbound } from '@/types/inbounds'
import { TrspTypes } from '@/types/transport'
import RandomUtil from '@/plugins/randomUtil'
import Data from '@/store/modules/data'
import { Loading, RefreshRight, Refresh, MagicStick, Plus, Delete } from '@element-plus/icons-vue'

const CloudflareTls = defineAsyncComponent(() => import('@/layouts/modals/CloudflareTls.vue'))

const props = defineProps<{
  visible: boolean
  id: number
  inTags: string[]
  tlsConfigs: any[]
}>()
const emit = defineEmits<{ close: []; 'update:modelValue': [v: boolean] }>()
void props.inTags

const inbound = ref<any>(createInbound('direct', { id: 0, tag: '' }))
const title = ref<'add' | 'edit'>('add')
const loading = ref(false)
const inboundJson = ref('{}')
const jsonError = ref('')

// ---------- 默认出站绑定(虚拟字段) ----------
// sing-box inbound 本身没有 outbound 字段。机场说的「该入站默认走某出站」
// 实际上由 route.rules 里一条 {inbound:[tag], outbound:tag} 实现。这里
// 在 modal 上加虚拟字段,保存时同步到 route.rules,改名/删除自动同步。
// 用 _nb_binding 标记由本逻辑管理的规则,不会覆盖用户手编规则。
const BINDING_MARKER = '_nb_binding'
const defaultOutbound = ref<string>('inherit') // 'inherit' | 'direct' | <outbound tag>
// 中转目标候选 — 只列真实代理出口 + 虚拟网卡端点。
//   - 落地节点:真实代理协议(vless/vmess/trojan/ss/hysteria/tuic 等)+ 聚合器
//   - 端点:虚拟网卡端点(warp/wireguard/tailscale)
// 不列 direct/block/dns 这种 system 出站 —— "本机出公网"统一走「不中转」选项,
// 由全局路由配置决定具体走哪个出站。中转目标只表达"转给落地"的语义。
const PROXY_OUTBOUND_TYPES = new Set([
  'vless', 'vmess', 'trojan', 'shadowsocks', 'shadowtls', 'anytls',
  'hysteria', 'hysteria2', 'tuic', 'naive',
  'socks', 'http', 'ssh',
  'selector', 'urltest',
])
// 列表 + label:label = "tag · 中转名称",中转名称空时只显示 tag。
// 下拉 :value 仍是 tag(写入 route.rules.outbound 还是 tag);只是 label
// 让用户能直接看到出站对应的中转标识,免得只看 tag 猜不出哪个落地。
const proxyOutbounds = computed((): { tag: string; display_name: string }[] =>
  ((Data().outbounds as any[]) ?? [])
    .filter((o: any) => o?.tag && PROXY_OUTBOUND_TYPES.has(o.type))
    .map((o: any) => ({ tag: o.tag, display_name: (o.display_name || '').trim() })),
)
const proxyOutTags = computed((): string[] => proxyOutbounds.value.map((o) => o.tag))
const endpointOutbounds = computed((): { tag: string; type: string }[] =>
  ((Data().endpoints as any[]) ?? [])
    .filter((e: any) => e?.tag)
    .map((e: any) => ({ tag: e.tag, type: e.type })),
)
const bindableOutTags = computed((): string[] => [
  ...proxyOutTags.value,
  ...endpointOutbounds.value.map((e) => e.tag),
])
// endpoint 类型对应的机场场景说明 — 在下拉 label 后追加
const endpointPurpose = (type: string): string => {
  switch (type) {
    case 'warp':      return 'Cloudflare Warp · 解锁 ChatGPT / Netflix 等被拉黑 IP'
    case 'wireguard': return '自建 WireGuard 落地'
    case 'tailscale': return 'Tailscale 组网'
    default:          return ''
  }
}
const findBindingRule = (cfg: any, tag: string): any | null => {
  for (const r of (cfg?.route?.rules || [])) {
    if (r?.[BINDING_MARKER] && Array.isArray(r.inbound) && r.inbound.includes(tag)) return r
  }
  return null
}
const syncDefaultOutboundFromConfig = () => {
  const r = findBindingRule(Data().config, inbound.value.tag)
  if (!r) { defaultOutbound.value = 'inherit'; return }
  // 老版本"强制本机直连"已经下线,把残留的 action:direct binding 显示为
  // inherit。下次保存会按 inherit 清掉那条规则,自动迁移到"按全局路由"。
  if (r.action === 'direct') defaultOutbound.value = 'inherit'
  else if (r.outbound) defaultOutbound.value = r.outbound
  else defaultOutbound.value = 'inherit'
}
// 保存 inbound 后调:按 defaultOutbound 同步 route.rules,有差异才推 config
const persistDefaultOutbound = async (oldTag: string) => {
  const cfg = JSON.parse(JSON.stringify(Data().config || {}))
  if (!cfg.route) cfg.route = {}
  if (!Array.isArray(cfg.route.rules)) cfg.route.rules = []
  const newTag = inbound.value.tag
  // 把 oldTag/newTag 上由本逻辑生成的 binding 全清掉,免重复
  cfg.route.rules = cfg.route.rules.filter((r: any) => {
    if (!r?.[BINDING_MARKER]) return true
    if (Array.isArray(r.inbound) && (r.inbound.includes(oldTag) || r.inbound.includes(newTag))) return false
    return true
  })
  // 按当前选择写新规则到 rules 最前(优先匹配)。
  // 'inherit' = 不写规则,本机出公网由全局路由决定(final / 其它 user 配的 rules)
  if (defaultOutbound.value && defaultOutbound.value !== 'inherit') {
    cfg.route.rules.unshift({ [BINDING_MARKER]: true, inbound: [newTag], action: 'route', outbound: defaultOutbound.value })
  }
  // 比对一下;无差异不调 save,免无谓 sing-box reload
  if (JSON.stringify((Data().config as any)?.route?.rules || []) !== JSON.stringify(cfg.route.rules)) {
    await Data().save('config', 'set', cfg)
  }
}

// sing-box 1.13+ 删了 stream-cipher 系(包括 'none' / aes-*-cfb 等)。
// 也排除 '2022-blake3-chacha20-poly1305' — sing-box 1.13.x 实测 inbound
// 加载报 "invalid argument"(其它 5 个 SS-2022 / AEAD 都正常),原因待查,
// 先从下拉移除避免用户撞坑。需要时走 advanced JSON 编辑器自己写。
const SS_METHODS = [
  '2022-blake3-aes-128-gcm', '2022-blake3-aes-256-gcm',
  'aes-128-gcm', 'aes-256-gcm', 'chacha20-ietf-poly1305', 'xchacha20-ietf-poly1305',
]

// SS-2022 系算法的 password 必须是 base64 编码的固定长度 PSK,sing-box 校验
// 不通过会报 "missing psk"。普通 AEAD 任意字符串都行。
//   2022-blake3-aes-128-gcm        → 16 字节 → base64 长 24
//   2022-blake3-aes-256-gcm        → 32 字节 → base64 长 44
//   2022-blake3-chacha20-poly1305  → 32 字节 → base64 长 44
const SS_2022_PSK_BYTES: Record<string, number> = {
  '2022-blake3-aes-128-gcm': 16,
  '2022-blake3-aes-256-gcm': 32,
  '2022-blake3-chacha20-poly1305': 32,
}
const ssIs2022 = computed((): boolean => !!SS_2022_PSK_BYTES[inbound.value.method])
const ssPskBytes = computed((): number => SS_2022_PSK_BYTES[inbound.value.method] ?? 0)
const genSsPassword = () => {
  if (ssIs2022.value) {
    inbound.value.password = RandomUtil.randomShadowsocksPassword(ssPskBytes.value)
  } else {
    inbound.value.password = RandomUtil.randomSeq(16)
  }
}
// 切算法时清掉旧 password — SS-2022 系互相之间长度也不一样;旧 password
// 留着只会在 save 时撞 "missing psk"(不够 24/44)。点 🔄 自动生成新值即可。
const onSsMethodChange = () => {
  if (ssIs2022.value) {
    inbound.value.password = RandomUtil.randomShadowsocksPassword(ssPskBytes.value)
  }
}

// 协议分组(类型按钮组用):
//   多用户 = 一端口 N 账号,凭证统一走 clients 表(InboundClients modal 管)
//     - UUID 系:vless/vmess/trojan/ss/shadowtls/hysteria/hysteria2/tuic/anytls
//     - Basic Auth 系:mixed/socks/http/naive(N 组 username/password)
//   单用户 = 端口本身就是入口,无凭证概念(无法/无需创建客户端)
const MULTI_USER_TYPES = [
  { value: 'vless',       label: 'VLESS' },
  { value: 'vmess',       label: 'VMess' },
  { value: 'trojan',      label: 'Trojan' },
  { value: 'shadowsocks', label: 'Shadowsocks' },
  { value: 'shadowtls',   label: 'ShadowTLS' },
  { value: 'hysteria',    label: 'Hysteria' },
  { value: 'hysteria2',   label: 'Hysteria2' },
  { value: 'tuic',        label: 'TUIC' },
  { value: 'anytls',      label: 'AnyTLS' },
  { value: 'naive',       label: 'Naive' },
  { value: 'mixed',       label: 'Mixed (SOCKS+HTTP)' },
  { value: 'socks',       label: 'SOCKS' },
  { value: 'http',        label: 'HTTP' },
]
const SINGLE_USER_TYPES = [
  { value: 'direct',   label: 'Direct' },
  { value: 'tun',      label: 'Tun' },
  { value: 'redirect', label: 'Redirect' },
  { value: 'tproxy',   label: 'TProxy' },
]
const isMultiUser = (t: string) => MULTI_USER_TYPES.some((x) => x.value === t)
const typeCap = (t: string) => isMultiUser(t) ? '多用户' : '单用户'
const HasTls = [InTypes.HTTP, InTypes.VMess, InTypes.Trojan, InTypes.Naive, InTypes.Hysteria, InTypes.TUIC, InTypes.Hysteria2, InTypes.VLESS, InTypes.AnyTls]
const OnlyTLS = [InTypes.Hysteria, InTypes.Hysteria2, InTypes.TUIC, InTypes.Naive, InTypes.AnyTls]
const HasInData = [InTypes.SOCKS, InTypes.HTTP, InTypes.Mixed, InTypes.Shadowsocks, InTypes.VMess, InTypes.ShadowTLS, InTypes.Trojan, InTypes.Hysteria, InTypes.VLESS, InTypes.AnyTls, InTypes.TUIC, InTypes.Hysteria2, InTypes.Naive]
const HasProtocolFields = ['shadowsocks', 'shadowtls', 'hysteria', 'hysteria2', 'tuic', 'naive', 'direct', 'tun', 'tproxy']
const HasTransport = ['vmess', 'trojan', 'vless']

const hasTls = computed(() => HasTls.includes(inbound.value.type))
const onlyTls = computed(() => OnlyTLS.includes(inbound.value.type))

const hasProtocolFields = computed(() => HasProtocolFields.includes(inbound.value.type))
const supportsTransport = computed(() => HasTransport.includes(inbound.value.type))

// hasAuthUsers 仅给上面 alert 用 — mixed/socks/http/naive 协议显示一段
// "凭证去客户端管理"的提示。凭证本身已统一走 clients 表 + InboundClients。
const hasAuthUsers = computed(() => ['socks', 'http', 'mixed', 'naive'].includes(inbound.value.type))

// 协议传输层推荐套餐 — 一键应用 transport + tls 组合
// vless: tcp+Reality(直连无证书最优) / ws+TLS(走 CDN 隐藏 IP)
// vmess: ws+TLS(主流方案) / 裸 TCP+TLS(开销最小)
// trojan: 裸 TCP+TLS(协议设计就是这样) / ws+TLS(套 CDN)
interface Preset {
  key: string
  title: string
  desc: string
  transport: any
  needsTls: boolean
}
const transportPresets = computed<Preset[]>(() => {
  const t = inbound.value.type
  if (t === 'vless') {
    return [
      { key: 'tcp-reality', title: 'TCP + Reality', desc: '抗探测最优 · 直连服务器 · 无需证书', transport: {}, needsTls: true },
      { key: 'ws-tls', title: 'WS + TLS', desc: '可走 Cloudflare CDN · 隐藏服务器真实 IP', transport: { type: 'ws', path: '/ws' }, needsTls: true },
      { key: 'grpc-tls', title: 'gRPC + TLS', desc: 'h2 长连接 · 抗丢包 · 支持 CDN', transport: { type: 'grpc', service_name: 'grpc' }, needsTls: true },
    ]
  }
  if (t === 'vmess') {
    return [
      { key: 'ws-tls', title: 'WS + TLS', desc: '主流方案 · 兼容 CDN', transport: { type: 'ws', path: '/ws' }, needsTls: true },
      { key: 'tcp-tls', title: '裸 TCP + TLS', desc: '握手开销最小 · 直连场景', transport: {}, needsTls: true },
    ]
  }
  if (t === 'trojan') {
    return [
      { key: 'tcp-tls', title: '裸 TCP + TLS', desc: '协议原生方案 · 性能最佳', transport: {}, needsTls: true },
      { key: 'ws-tls', title: 'WS + TLS', desc: '套 CDN · 隐藏 IP', transport: { type: 'ws', path: '/ws' }, needsTls: true },
    ]
  }
  return []
})

const isPresetActive = (p: Preset) => {
  const cur = inbound.value.transport?.type || ''
  const want = p.transport.type || ''
  return cur === want
}

const applyPreset = (p: Preset) => {
  inbound.value.transport = { ...p.transport }
  // 套餐需要 TLS 但当前没启用 → 自动选第一个 TLS 配置(没有就提示去自动签发)
  if (p.needsTls && !inbound.value.tls_id && props.tlsConfigs.length > 0) {
    inbound.value.tls_id = props.tlsConfigs[0].id
  }
  refreshJson()
}

// CF 自动签发 wizard 入口
const cfWizardVisible = ref(false)
const openCfWizard = () => { cfWizardVisible.value = true }
const onCfTlsCreated = (tlsId: number) => {
  // 签发成功后,新的 tls_id 自动套到当前 inbound 上,关闭 wizard
  inbound.value.tls_id = tlsId
  cfWizardVisible.value = false
  refreshJson()
}

const transportType = computed({
  get: () => inbound.value.transport?.type || '',
  set: (v: string) => {
    if (!v) {
      inbound.value.transport = {}
    } else {
      inbound.value.transport = { type: v }
    }
    refreshJson()
  },
})

const setHy2Obfs = (v: string) => {
  if (!v) {
    delete inbound.value.obfs
  } else {
    inbound.value.obfs = { type: 'salamander', password: v }
  }
  refreshJson()
}

// 已被其它入站占用的端口集合(编辑模式排除自己)
const usedPorts = computed<Map<number, string>>(() => {
  const m = new Map<number, string>()
  for (const i of (Data().inbounds ?? []) as any[]) {
    if (props.id > 0 && i.id === props.id) continue
    if (typeof i.listen_port === 'number') m.set(i.listen_port, i.tag)
  }
  return m
})

const portConflict = computed(() => {
  if (inbound.value.type === InTypes.Tun) return ''
  const p = inbound.value.listen_port
  return typeof p === 'number' ? (usedPorts.value.get(p) || '') : ''
})

// 抽一个空闲端口:在 [10000, 60000] 区间随机,直到撞不到 usedPorts。
// 重试 100 次足够 — 5 万空间 vs 用户撑死几十个入站。
const pickFreePort = (): number => {
  const used = usedPorts.value
  for (let i = 0; i < 100; i++) {
    const p = RandomUtil.randomIntRange(10000, 60000)
    if (!used.has(p)) return p
  }
  return RandomUtil.randomIntRange(10000, 60000)
}
const reseedPort = () => {
  inbound.value.listen_port = pickFreePort()
  // tag 跟着改,免得保留上一个端口的字面量
  if (title.value === 'add' && inbound.value.tag?.match(/-\d+$/)) {
    inbound.value.tag = inbound.value.type + '-' + inbound.value.listen_port
  }
  refreshJson()
}

const validate = computed(() => {
  if (!inbound.value || !inbound.value.tag) return false
  if (inbound.value.type !== InTypes.Tun) {
    if (inbound.value.listen_port > 65535 || inbound.value.listen_port < 1) return false
    if (portConflict.value) return false
  }
  if (OnlyTLS.includes(inbound.value.type) && !inbound.value.tls_id) return false
  return true
})

// 选协议:既切类型也兜底端口冲突 — 切完类型后如果当前端口已被占,重新随机一个。
const selectType = (v: string) => {
  inbound.value.type = v
  changeType()
  if (inbound.value.type !== InTypes.Tun && portConflict.value) {
    inbound.value.listen_port = pickFreePort()
    inbound.value.tag = v + '-' + inbound.value.listen_port
    refreshJson()
  }
}

const refreshJson = () => {
  inboundJson.value = JSON.stringify(inbound.value, null, 2)
  jsonError.value = ''
}

const syncFromJson = () => refreshJson()

const onJsonEdit = () => {
  try {
    const parsed = JSON.parse(inboundJson.value)
    if (typeof parsed === 'object' && parsed !== null) {
      inbound.value = parsed
      jsonError.value = ''
    }
  } catch (e: any) {
    jsonError.value = `JSON: ${e.message}`
  }
}

watch(() => inbound.value.type, refreshJson)
watch(() => inbound.value.tag, refreshJson)
watch(() => inbound.value.listen, refreshJson)
watch(() => inbound.value.listen_port, refreshJson)
watch(() => inbound.value.tls_id, refreshJson)
watch(() => inbound.value.method, refreshJson)
watch(() => inbound.value.password, refreshJson)
watch(() => inbound.value.up_mbps, refreshJson)
watch(() => inbound.value.down_mbps, refreshJson)
watch(() => inbound.value.version, refreshJson)
watch(() => inbound.value.congestion_control, refreshJson)

const loadData = async (id: number) => {
  loading.value = true
  const arr = await Data().loadInbounds([id])
  inbound.value = arr[0]
  if (HasInData.includes(inbound.value.type) && inbound.value.out_json == null) {
    inbound.value.out_json = {}
  }
  // 把现有 route.rules 里指向此入站的 binding 反向同步到 select
  syncDefaultOutboundFromConfig()
  refreshJson()
  loading.value = false
}

const updateData = (id: number) => {
  if (id > 0) {
    loadData(id)
    title.value = 'edit'
  } else {
    const port = pickFreePort()
    inbound.value = createInbound('direct', { id: 0, tag: 'direct-' + port, listen: '::', listen_port: port })
    if (HasInData.includes(inbound.value.type)) {
      inbound.value.addrs = []
      inbound.value.out_json = {}
    } else {
      delete inbound.value.addrs
      delete inbound.value.out_json
    }
    title.value = 'add'
    loading.value = false
    defaultOutbound.value = 'inherit'
    refreshJson()
  }
}

const changeType = () => {
  if (!inbound.value.listen_port) inbound.value.listen_port = pickFreePort()
  const tag = props.id > 0 ? inbound.value.tag : inbound.value.type + '-' + inbound.value.listen_port
  const prev = { id: inbound.value.id, tag, listen: inbound.value.listen ?? '::', listen_port: inbound.value.listen_port }
  inbound.value = createInbound(inbound.value.type, inbound.value.type !== InTypes.Tun ? prev : { tag })
  if (HasInData.includes(inbound.value.type)) {
    inbound.value.addrs = []
    inbound.value.out_json = {}
  } else {
    delete inbound.value.addrs
    delete inbound.value.out_json
  }
  // 凭证(SOCKS/HTTP/Mixed/Naive 的 username/password,以及 vless/vmess 等
  // 的 UUID)统一在 InboundClients modal 创建,不再在入站编辑器里直接塞。
  refreshJson()
}

const closeModal = () => {
  updateData(0)
  emit('close')
}

const saveChanges = async () => {
  if (!props.visible) return
  if (jsonError.value) return
  try { inbound.value = JSON.parse(inboundJson.value) } catch { /* ignore */ }
  if (Data().checkTag('inbound', inbound.value.id, inbound.value.tag)) return
  loading.value = true
  // 创建/编辑入站不再做"初始客户绑定"——保存后用户去入站列表点「客户端」按钮
  // 走 InboundClients modal 单独管理。这条路径更清晰、避免双入口配同一份数据。
  // 编辑场景下,先记下旧 tag,免得改名后 binding 找不到旧规则去清
  let oldTag = ''
  if (props.id > 0) {
    const old = (Data().inbounds as any[])?.find((i: any) => i.id === props.id)
    oldTag = old?.tag || ''
  }
  const success = await Data().save('inbounds', props.id == 0 ? 'new' : 'edit', inbound.value, [])
  if (success) {
    // 入站保存成功后,同步 route.rules binding(无差异不会触发 reload)
    try { await persistDefaultOutbound(oldTag) } catch (e) { /* 不阻塞关闭 */ }
    closeModal()
  }
  loading.value = false
}

watch(() => props.visible, (v) => {
  if (v) loading.value = true
})
</script>

<style scoped>
.modal-loading {
  display: flex;
  justify-content: center;
  padding: 60px 0;
  font-size: 32px;
  color: var(--nc-primary);
}

.ib-form { display: flex; flex-direction: column; gap: 14px; }

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 6px 16px;
}
/* 关键:form-item--full 强制跨满整行 grid 列 */
.form-grid > .form-item--full,
.form-grid :deep(.form-item--full) { grid-column: 1 / -1; }

.form-row {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  width: 100%;
}
.form-row__item {
  flex: 1 1 0;
  min-width: 0;
}
.form-row :deep(.el-form-item) { margin-bottom: 0; width: 100%; }

/* 端口输入框 + 重抽按钮整体 */
.port-input {
  display: flex;
  width: 100%;
  border: 1px solid var(--nc-border);
  border-radius: var(--radius-md);
  background: #fff;
  overflow: hidden;
  transition: border-color var(--t-fast);
}
.port-input:focus-within {
  border-color: var(--nc-primary);
}
.port-input__num {
  flex: 1;
  width: auto !important;
}
.port-input__num :deep(.el-input__wrapper) {
  box-shadow: none !important;
  background: transparent;
  padding-left: 12px;
}
.port-input__num :deep(.el-input__inner) {
  text-align: left;
  font-family: var(--font-mono);
  font-size: 13px;
}
.port-input__num.is-conflict :deep(.el-input__inner) {
  color: var(--nc-danger);
}
.port-input__btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  border: none;
  border-left: 1px solid var(--nc-border);
  background: var(--nc-bg-3);
  color: var(--nc-text-muted);
  cursor: pointer;
  transition: background var(--t-fast), color var(--t-fast);
}
.port-input__btn:hover {
  background: var(--nc-primary-soft);
  color: var(--nc-primary);
}

.form-section {
  background: var(--nc-bg-3);
  border: 1px solid var(--nc-border-soft);
  border-radius: var(--radius-md);
  padding: 12px 14px;
}
.form-section__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.form-section__title {
  margin: 0;
  font-size: 12.5px;
  font-weight: 600;
  color: var(--nc-text-1);
  letter-spacing: 0.02em;
}

.form-warn {
  margin: 4px 0 0;
  font-size: 11.5px;
  color: var(--nc-warning, #d97706);
}

.advanced {
  border: 1px solid var(--nc-border-soft);
  border-radius: var(--radius-md);
  background: #fafbfc;
}
.advanced > summary {
  padding: 10px 14px;
  cursor: pointer;
  user-select: none;
  font-size: 12.5px;
  color: var(--nc-text-muted);
  list-style: none;
}
.advanced > summary::-webkit-details-marker { display: none; }
.advanced[open] > summary { border-bottom: 1px solid var(--nc-border-soft); }
.advanced__body { padding: 12px 14px; }
.advanced__head { display: flex; justify-content: flex-end; margin-bottom: 6px; }

.hint-muted { font-size: 11.5px; color: var(--nc-text-muted); }

.json-editor :deep(.el-textarea__inner) {
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.55;
  background: #f8fafc;
  border-color: var(--nc-border);
}
.mono-input :deep(.el-textarea__inner),
.mono-input :deep(.el-input__inner) {
  font-family: var(--font-mono);
}
.json-error {
  margin: 6px 0 0;
  font-size: 11.5px;
  color: var(--nc-danger);
  font-family: var(--font-mono);
}

/* 多用户 / 单用户 标签 */
.type-cap {
  display: inline-block;
  font-size: 10.5px;
  font-weight: 600;
  letter-spacing: 0.04em;
  padding: 1px 8px;
  border-radius: var(--radius-pill);
  vertical-align: middle;
  white-space: nowrap;
  flex-shrink: 0;
}
.type-cap.cap-multi { color: #2563eb; background: #dbeafe; }
.type-cap.cap-single { color: #475569; background: #e2e8f0; }

.type-picker {
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: 100%;
}
.type-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.type-group__label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11.5px;
  color: var(--nc-text-muted);
  flex-wrap: nowrap;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.type-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.type-chip {
  font: inherit;
  font-size: 13px;
  font-weight: 500;
  padding: 5px 14px;
  border-radius: var(--radius-pill);
  border: 1px solid var(--nc-border);
  background: #fff;
  color: var(--nc-text-1);
  cursor: pointer;
  transition: background var(--t-fast), color var(--t-fast), border-color var(--t-fast);
}
.type-chip:hover {
  border-color: var(--nc-primary);
  color: var(--nc-primary);
}
.type-chip.is-active.is-multi {
  background: var(--nc-primary);
  border-color: var(--nc-primary);
  color: #fff;
}
.type-chip.is-active.is-single {
  background: #475569;
  border-color: #475569;
  color: #fff;
}

.form-hint {
  font-size: 12px;
  color: var(--nc-text-muted);
  margin: 4px 0 0;
}
/* el-option 里的灰色后缀说明:让标签 + 用途说明在同一行,后缀小字 */
.opt-suffix {
  margin-left: 8px;
  color: var(--nc-text-muted);
  font-size: 11.5px;
}

/* TLS 行:select + 自动签发按钮 */
.tls-row {
  display: flex;
  gap: 8px;
  width: 100%;
}
.tls-row__select { flex: 1; min-width: 0; }
.tls-row__btn { flex-shrink: 0; }

/* 协议推荐套餐 */
.presets {
  display: flex;
  align-items: stretch;
  flex-wrap: wrap;
  gap: 8px;
  margin: 10px 0 12px;
}
.presets__label {
  display: flex;
  align-items: center;
  font-size: 11.5px;
  color: var(--nc-text-muted);
  flex-shrink: 0;
}
.preset-chip {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  padding: 6px 12px;
  border: 1px solid var(--nc-border);
  border-radius: var(--radius-md);
  background: #fff;
  cursor: pointer;
  text-align: left;
  font: inherit;
  transition: all var(--t-fast);
  min-width: 180px;
}
.preset-chip:hover {
  border-color: var(--nc-primary);
}
.preset-chip.is-active {
  border-color: var(--nc-primary);
  background: var(--nc-primary-soft);
}
.preset-chip__title {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--nc-text-1);
}
.preset-chip__desc {
  font-size: 11px;
  color: var(--nc-text-muted);
  line-height: 1.4;
}
.preset-chip.is-active .preset-chip__title { color: var(--nc-primary); }

/* mixed/socks/http/naive 引导跳到 InboundClients 的提示条 */
.auth-redirect-alert { margin: 12px 0; }
</style>
