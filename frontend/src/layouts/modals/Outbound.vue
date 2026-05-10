<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="closeModal"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="$t('actions.' + title) + ' ' + $t('objects.outbound')"
    destroy-on-close
  >
    <!-- 顶部:分享链接一键导入(参考 nexcore-x-ui 出站管理) -->
    <div class="import-box">
      <div class="import-label">
        <el-icon><Link /></el-icon>
        从分享链接导入(支持 vmess / vless / trojan / ss / hysteria2 / tuic 等)
      </div>
      <el-input
        v-model="link"
        type="textarea"
        :rows="2"
        spellcheck="false"
        class="mono-input"
        placeholder="vless://uuid@host:port?type=tcp...   或者 vmess://eyJ...   或者 trojan://...   或者 ss://..."
      />
      <div class="import-actions">
        <el-button size="small" type="primary" :disabled="!link.trim() || loading" :loading="loading" @click="linkConvert">
          解析并填入下面字段
        </el-button>
        <span class="hint-muted">导入后请确认 tag / TLS / transport 字段是否符合预期</span>
      </div>
    </div>

    <el-form :model="outbound" label-position="top" class="ob-form">
      <!-- 快速套餐:只在「新增」时显示。一键填全 type+TLS+transport+ALPN+utls 指纹 -->
      <div v-if="title === 'add'" class="quick-presets">
        <span class="quick-presets__label">
          <el-icon><MagicStick /></el-icon>快速套餐
        </span>
        <el-button size="small" plain @click="applyPreset('vless-vision')">VLESS+Vision+TLS</el-button>
        <el-button size="small" plain @click="applyPreset('vmess-ws-tls')">VMess+WS+TLS(套 CDN)</el-button>
        <el-button size="small" plain @click="applyPreset('trojan-grpc-tls')">Trojan+gRPC+TLS</el-button>
        <el-button size="small" plain @click="applyPreset('hysteria2')">Hysteria2(QUIC)</el-button>
      </div>

      <!-- 基础四件套 -->
      <div class="form-grid">
        <el-form-item :label="$t('type')">
          <el-select v-model="outbound.type" filterable @change="changeType">
            <el-option v-for="(v, k) in OutTypes" :key="k" :label="k" :value="v" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('objects.tag')">
          <el-input v-model="outbound.tag" :disabled="title === 'edit'" placeholder="字母数字 . _ -;路由用此 tag 引用" />
        </el-form-item>
        <el-form-item label="中转名称(分享链接 ps 前缀)">
          <el-input v-model="outbound.display_name" placeholder="如「香港落地」、「东京 BGP」(留空回退到 tag)" />
        </el-form-item>
        <template v-if="!NoServer.includes(outbound.type)">
          <el-form-item :label="$t('out.addr')">
            <el-input v-model="outbound.server" placeholder="远端域名或 IP" />
          </el-form-item>
          <el-form-item :label="$t('out.port')">
            <el-input-number v-model="outbound.server_port" :min="1" :max="65535" controls-position="right" style="width: 100%" />
          </el-form-item>
        </template>
      </div>

      <!-- 凭据(按协议显示对应字段) -->
      <div v-if="hasCredFields" class="form-section">
        <h4 class="form-section__title">凭据</h4>
        <div class="form-grid">
          <!-- UUID 类协议 -->
          <el-form-item v-if="['vless','vmess','tuic'].includes(outbound.type)" label="UUID">
            <el-input v-model="outbound.uuid" placeholder="36 位 UUID" class="mono">
              <template #append>
                <el-button @click="genUuid"><el-icon><Refresh /></el-icon></el-button>
              </template>
            </el-input>
          </el-form-item>

          <!-- 密码类协议 -->
          <el-form-item v-if="['trojan','tuic','hysteria2','anytls','shadowtls'].includes(outbound.type)" label="密码">
            <el-input v-model="outbound.password" type="password" show-password autocomplete="new-password" />
          </el-form-item>

          <!-- shadowsocks -->
          <template v-if="outbound.type === 'shadowsocks'">
            <el-form-item label="加密方法">
              <el-select v-model="outbound.method" filterable>
                <el-option v-for="m in SS_METHODS" :key="m" :label="m" :value="m" />
              </el-select>
            </el-form-item>
            <el-form-item label="密码">
              <el-input v-model="outbound.password" type="password" show-password />
            </el-form-item>
          </template>

          <!-- vless flow -->
          <el-form-item v-if="outbound.type === 'vless'" label="Flow">
            <el-select v-model="outbound.flow" clearable placeholder="不填 = 普通 vless">
              <el-option label="(空)" value="" />
              <el-option label="xtls-rprx-vision" value="xtls-rprx-vision" />
            </el-select>
          </el-form-item>

          <!-- vmess 安全 -->
          <template v-if="outbound.type === 'vmess'">
            <el-form-item label="加密(security)">
              <el-select v-model="outbound.security">
                <el-option v-for="s in VMESS_SECURITY" :key="s" :label="s" :value="s" />
              </el-select>
            </el-form-item>
            <el-form-item label="alter_id">
              <el-input-number v-model="outbound.alter_id" :min="0" controls-position="right" style="width: 100%" />
            </el-form-item>
          </template>

          <!-- socks / http / naive 用户名密码可选 -->
          <template v-if="['socks','http','naive'].includes(outbound.type)">
            <el-form-item label="用户名(可选)">
              <el-input v-model="outbound.username" autocomplete="off" />
            </el-form-item>
            <el-form-item label="密码(可选)">
              <el-input v-model="outbound.password" type="password" show-password autocomplete="new-password" />
            </el-form-item>
            <el-form-item v-if="outbound.type === 'socks'" label="SOCKS 版本">
              <el-select v-model="outbound.version">
                <el-option label="5" value="5" />
                <el-option label="4a" value="4a" />
                <el-option label="4" value="4" />
              </el-select>
            </el-form-item>
          </template>

          <!-- ssh -->
          <template v-if="outbound.type === 'ssh'">
            <el-form-item label="用户">
              <el-input v-model="outbound.user" placeholder="root" />
            </el-form-item>
            <el-form-item label="密码">
              <el-input v-model="outbound.password" type="password" show-password />
            </el-form-item>
            <el-form-item label="私钥路径(可选)">
              <el-input v-model="outbound.private_key_path" placeholder="/root/.ssh/id_ed25519" class="mono" />
            </el-form-item>
          </template>

          <!-- shadowtls 版本 -->
          <el-form-item v-if="outbound.type === 'shadowtls'" label="版本">
            <el-select v-model="outbound.version">
              <el-option :label="3" :value="3" />
              <el-option :label="2" :value="2" />
              <el-option :label="1" :value="1" />
            </el-select>
          </el-form-item>

          <!-- hysteria 系列 -->
          <template v-if="['hysteria','hysteria2'].includes(outbound.type)">
            <el-form-item label="上行 Mbps">
              <el-input-number v-model="outbound.up_mbps" :min="0" controls-position="right" style="width: 100%" />
            </el-form-item>
            <el-form-item label="下行 Mbps">
              <el-input-number v-model="outbound.down_mbps" :min="0" controls-position="right" style="width: 100%" />
            </el-form-item>
            <el-form-item v-if="outbound.type === 'hysteria'" label="auth_str">
              <el-input v-model="outbound.auth_str" />
            </el-form-item>
          </template>

          <!-- tuic 拥塞控制 -->
          <el-form-item v-if="outbound.type === 'tuic'" label="拥塞控制">
            <el-select v-model="outbound.congestion_control">
              <el-option label="cubic" value="cubic" />
              <el-option label="new_reno" value="new_reno" />
              <el-option label="bbr" value="bbr" />
            </el-select>
          </el-form-item>

          <!-- selector / urltest 子出站 -->
          <template v-if="['selector','urltest'].includes(outbound.type)">
            <el-form-item label="子出站(下游 tag,逗号或多选)" class="form-item--full">
              <el-select v-model="outbound.outbounds" multiple filterable allow-create placeholder="选择或输入">
                <el-option v-for="t in tags" :key="t" :label="t" :value="t" />
              </el-select>
            </el-form-item>
            <el-form-item v-if="outbound.type === 'urltest'" label="默认出站(测速失败时回退)">
              <el-select v-model="outbound.default" clearable filterable>
                <el-option v-for="t in tags" :key="t" :label="t" :value="t" />
              </el-select>
            </el-form-item>
          </template>
        </div>
      </div>

      <!-- TLS -->
      <div v-if="supportsTls" class="form-section">
        <div class="form-section__head">
          <div class="form-section__head-left">
            <h4 class="form-section__title">TLS</h4>
            <p class="form-section__hint">加密层 · VLESS+Vision / Trojan 必开 · VMess 套 CDN 必开 · 裸 VMess/SS 可关</p>
          </div>
          <el-switch v-model="tlsEnabled" />
        </div>
        <div v-if="tlsEnabled" class="form-grid">
          <el-form-item label="SNI(server_name)">
            <el-input v-model="outbound.tls.server_name" placeholder="留空 = 用 server 字段" />
          </el-form-item>
          <el-form-item label="ALPN(逗号分隔)">
            <el-input :model-value="(outbound.tls.alpn || []).join(',')" @input="(v: string) => outbound.tls.alpn = v ? v.split(',').map((x) => x.trim()) : []" placeholder="h2,http/1.1" />
          </el-form-item>
          <el-form-item label="允许不安全(insecure)">
            <el-switch v-model="outbound.tls.insecure" />
          </el-form-item>
          <el-form-item label="uTLS 指纹(伪装)">
            <el-select v-model="utlsFp" clearable placeholder="不启用">
              <el-option v-for="fp in UTLS_FPS" :key="fp" :label="fp" :value="fp" />
            </el-select>
          </el-form-item>
        </div>

        <div v-if="tlsEnabled" class="form-subsection">
          <div class="form-subsection__head">
            <span>Reality(留空 = 不启用)</span>
            <el-switch v-model="realityEnabled" />
          </div>
          <div v-if="realityEnabled" class="form-grid">
            <el-form-item label="Public Key">
              <el-input v-model="outbound.tls.reality.public_key" class="mono" />
            </el-form-item>
            <el-form-item label="Short ID">
              <el-input v-model="outbound.tls.reality.short_id" class="mono" />
            </el-form-item>
          </div>
        </div>
      </div>

      <!-- Transport(永远显示,不支持的协议显示解释卡片而不是隐藏) -->
      <div class="form-section">
        <div class="form-section__head">
          <div class="form-section__head-left">
            <h4 class="form-section__title">传输层(Transport)</h4>
            <p class="form-section__hint">
              v2ray 风格的伪装层 — 把代理流量再包一层 WS / gRPC / HTTP,墙看起来就是普通 web。
              <b>WS+TLS</b> 可套 Cloudflare CDN 隐藏源 IP;<b>gRPC+TLS</b> 走 h2 长连接抗丢包。不开 = 裸 TCP,易被 SNI 阻断。
            </p>
          </div>
          <el-select v-if="supportsTransport" v-model="transportType" size="small" style="width: 140px">
            <el-option label="(裸 TCP)" value="" />
            <el-option v-for="(v, k) in TrspTypes" :key="k" :label="k" :value="v" />
          </el-select>
        </div>
        <!-- 不支持 transport 的协议:显示原因,不让用户误以为 UI 缺失 -->
        <div v-if="!supportsTransport" class="transport-unsupported">
          <el-icon><InfoFilled /></el-icon>
          <span v-if="['hysteria','hysteria2','tuic'].includes(outbound.type)">
            <b>{{ outbound.type }}</b> 是 QUIC 协议,传输层已经内置在 QUIC/TLS 1.3 里,不需要额外的 v2ray transport
          </span>
          <span v-else-if="outbound.type === 'shadowtls'">
            <b>shadowtls</b> 自身就是一层 TLS 伪装包装,不再叠加 transport
          </span>
          <span v-else-if="outbound.type === 'naive'">
            <b>naive</b> 自己用 HTTP/2 + TLS 包装,不需要额外 transport
          </span>
          <span v-else-if="['shadowsocks','socks','http','ssh','wireguard'].includes(outbound.type)">
            <b>{{ outbound.type }}</b> 协议不支持 v2ray transport(sing-box 上游限制)。要套 CDN 请改用 vmess / vless / trojan
          </span>
          <span v-else>
            该协议类型不支持 v2ray transport
          </span>
        </div>
        <div v-if="supportsTransport && transportType" class="form-grid">
          <template v-if="transportType === 'ws'">
            <el-form-item label="Path">
              <el-input v-model="outbound.transport.path" placeholder="/" class="mono" />
            </el-form-item>
            <el-form-item label="Host(WS Header)">
              <el-input :model-value="(outbound.transport.headers || {}).Host" @input="(v: string) => outbound.transport.headers = v ? { Host: v } : undefined" />
            </el-form-item>
          </template>
          <template v-else-if="transportType === 'grpc'">
            <el-form-item label="serviceName">
              <el-input v-model="outbound.transport.service_name" class="mono" />
            </el-form-item>
          </template>
          <template v-else-if="transportType === 'http'">
            <el-form-item label="Path">
              <el-input v-model="outbound.transport.path" placeholder="/" class="mono" />
            </el-form-item>
            <el-form-item label="Host(逗号分隔)">
              <el-input :model-value="(outbound.transport.host || []).join(',')" @input="(v: string) => outbound.transport.host = v ? v.split(',').map((x) => x.trim()) : []" />
            </el-form-item>
          </template>
          <template v-else-if="transportType === 'httpupgrade'">
            <el-form-item label="Path">
              <el-input v-model="outbound.transport.path" placeholder="/" class="mono" />
            </el-form-item>
            <el-form-item label="Host">
              <el-input v-model="outbound.transport.host" />
            </el-form-item>
          </template>
        </div>
      </div>

      <!-- 高级:JSON 编辑器(逃生通道) -->
      <details class="advanced">
        <summary>
          <span>高级:完整 JSON</span>
          <span class="hint-muted">— 上面字段覆盖不到的可以直接改这里</span>
        </summary>
        <div class="advanced__body">
          <div class="advanced__head">
            <el-tooltip content="从上面字段重新生成" placement="top">
              <el-button text @click="syncFromJson"><el-icon><RefreshRight /></el-icon></el-button>
            </el-tooltip>
          </div>
          <el-input
            v-model="outboundJson"
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
      <el-button type="primary" :loading="loading" @click="saveChanges">{{ $t('actions.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue'
import { OutTypes, createOutbound } from '@/types/outbounds'
import { TrspTypes } from '@/types/transport'
import RandomUtil from '@/plugins/randomUtil'
import HttpUtils from '@/plugins/httputil'
import Data from '@/store/modules/data'
import { ElMessage } from 'element-plus'
import { Refresh, RefreshRight, Link, InfoFilled, MagicStick } from '@element-plus/icons-vue'

const props = defineProps<{ visible: boolean; data: string; id: number; tags: string[] }>()
const emit = defineEmits<{ close: []; 'update:modelValue': [v: boolean] }>()

const outbound = ref<any>(createOutbound('direct', { tag: '' }))
const title = ref<'add' | 'edit'>('add')
const link = ref('')
const loading = ref(false)
const outboundJson = ref('{}')
const jsonError = ref('')

const NoServer = [OutTypes.Direct, OutTypes.Selector, OutTypes.URLTest, OutTypes.Tor]

// 常见 SS 加密方法 — 覆盖 SS-2022 + 经典 AEAD。
// sing-box 1.13+ 不再支持 'none' / stream-cipher 系,从下拉里移除。
const SS_METHODS = [
  '2022-blake3-aes-128-gcm', '2022-blake3-aes-256-gcm', '2022-blake3-chacha20-poly1305',
  'aes-128-gcm', 'aes-256-gcm', 'chacha20-ietf-poly1305', 'xchacha20-ietf-poly1305',
]
const VMESS_SECURITY = ['auto', 'none', 'aes-128-gcm', 'chacha20-poly1305', 'zero']
const UTLS_FPS = ['chrome', 'firefox', 'safari', 'ios', 'android', 'edge', 'random', 'randomized']

const hasCredFields = computed(() =>
  ['vless','vmess','trojan','tuic','hysteria2','anytls','shadowtls','shadowsocks','socks','http','naive','ssh','hysteria','selector','urltest'].includes(outbound.value.type),
)
const supportsTls = computed(() =>
  ['vless','vmess','trojan','http','naive','hysteria','hysteria2','tuic','shadowtls','anytls'].includes(outbound.value.type),
)
const supportsTransport = computed(() =>
  ['vless','vmess','trojan'].includes(outbound.value.type),
)

const tlsEnabled = computed({
  get: () => !!outbound.value.tls?.enabled,
  set: (v: boolean) => {
    if (!outbound.value.tls) outbound.value.tls = {}
    outbound.value.tls.enabled = v
    if (v && !outbound.value.tls.alpn) outbound.value.tls.alpn = ['h2', 'http/1.1']
    refreshJson()
  },
})

const realityEnabled = computed({
  get: () => !!outbound.value.tls?.reality?.enabled,
  set: (v: boolean) => {
    if (!outbound.value.tls) outbound.value.tls = {}
    if (v) {
      outbound.value.tls.reality = outbound.value.tls.reality || { enabled: true, public_key: '', short_id: '' }
      outbound.value.tls.reality.enabled = true
    } else {
      delete outbound.value.tls.reality
    }
    refreshJson()
  },
})

const utlsFp = computed({
  get: () => outbound.value.tls?.utls?.fingerprint || '',
  set: (v: string) => {
    if (!outbound.value.tls) outbound.value.tls = {}
    if (v) outbound.value.tls.utls = { enabled: true, fingerprint: v }
    else delete outbound.value.tls.utls
    refreshJson()
  },
})

const transportType = computed({
  get: () => outbound.value.transport?.type || '',
  set: (v: string) => {
    if (!v) {
      delete outbound.value.transport
    } else {
      outbound.value.transport = { type: v }
    }
    refreshJson()
  },
})

const refreshJson = () => {
  outboundJson.value = JSON.stringify(outbound.value, null, 2)
  jsonError.value = ''
}

const syncFromJson = () => refreshJson()

const onJsonEdit = () => {
  try {
    const parsed = JSON.parse(outboundJson.value)
    if (typeof parsed === 'object' && parsed !== null) {
      outbound.value = parsed
      jsonError.value = ''
    }
  } catch (e: any) {
    jsonError.value = `JSON: ${e.message}`
  }
}

watch(() => outbound.value.type, refreshJson)
watch(() => outbound.value.tag, refreshJson)
watch(() => outbound.value.display_name, refreshJson)
watch(() => outbound.value.server, refreshJson)
watch(() => outbound.value.server_port, refreshJson)
watch(() => outbound.value.uuid, refreshJson)
watch(() => outbound.value.password, refreshJson)
watch(() => outbound.value.method, refreshJson)
watch(() => outbound.value.flow, refreshJson)
watch(() => outbound.value.security, refreshJson)
watch(() => outbound.value.username, refreshJson)
watch(() => outbound.value.up_mbps, refreshJson)
watch(() => outbound.value.down_mbps, refreshJson)

const updateData = (id: number) => {
  if (id > 0) {
    const newData = JSON.parse(props.data)
    outbound.value = createOutbound(newData.type, newData)
    title.value = 'edit'
  } else {
    // 默认 vless 而不是 direct:机场场景几乎不在「出站管理」加 direct
    // (direct 由 ensureDirectOutbound 自动补);默认 vless 让用户打开就
    // 看到 TLS / Transport 区域,免得以为面板缺字段。
    outbound.value = createOutbound('vless', { tag: 'vless-' + RandomUtil.randomSeq(3) })
    title.value = 'add'
  }
  link.value = ''
  refreshJson()
}

const changeType = () => {
  const tag = props.id > 0 ? outbound.value.tag : outbound.value.type + '-' + RandomUtil.randomSeq(3)
  const prev = {
    id: outbound.value.id,
    tag,
    listen: outbound.value.listen,
    listen_port: outbound.value.listen_port,
  }
  outbound.value = createOutbound(outbound.value.type, prev)
  refreshJson()
}

// 快速套餐:一键写好 type + TLS + transport + ALPN + utls 指纹,
// 用户只剩 server / port / uuid / password 这些必填项。
// 跟 CF wizard 的默认参数对齐(utls fingerprint 用 chrome,ALPN 按协议匹配)
const applyPreset = (key: string) => {
  const tag = key + '-' + RandomUtil.randomSeq(3)
  switch (key) {
    case 'vless-vision':
      outbound.value = createOutbound('vless', {
        tag,
        flow: 'xtls-rprx-vision',
        // VLESS+Vision 主流方案 ALPN 一般不写;sing-box 跟随
        tls: { enabled: true, utls: { enabled: true, fingerprint: 'chrome' } },
        transport: undefined,
      } as any)
      break
    case 'vmess-ws-tls':
      outbound.value = createOutbound('vmess', {
        tag,
        security: 'auto',
        tls: { enabled: true, alpn: ['h2', 'http/1.1'], utls: { enabled: true, fingerprint: 'chrome' } },
        transport: { type: 'ws', path: '/ws' },
      } as any)
      break
    case 'trojan-grpc-tls':
      outbound.value = createOutbound('trojan', {
        tag,
        tls: { enabled: true, alpn: ['h2'], utls: { enabled: true, fingerprint: 'chrome' } },
        transport: { type: 'grpc', service_name: 'grpc' },
      } as any)
      break
    case 'hysteria2':
      outbound.value = createOutbound('hysteria2', {
        tag,
        // QUIC 自带传输层,不需要 transport;ALPN 由 hysteria2 协议自管
        tls: { enabled: true },
      } as any)
      break
  }
  refreshJson()
  ElMessage.success(`已套用「${key}」预设,补完 server / port / uuid / password 即可保存`)
}

const genUuid = () => {
  const u = crypto.randomUUID?.() ?? RandomUtil.randomSeq(8) + '-' + RandomUtil.randomSeq(4) + '-' + RandomUtil.randomSeq(4) + '-' + RandomUtil.randomSeq(4) + '-' + RandomUtil.randomSeq(12)
  outbound.value.uuid = u
  refreshJson()
}

const closeModal = () => {
  updateData(0)
  emit('close')
}

const saveChanges = async () => {
  if (!props.visible) return
  if (jsonError.value) {
    ElMessage.error(jsonError.value)
    return
  }
  try { outbound.value = JSON.parse(outboundJson.value) } catch { /* ignore */ }
  if (Data().checkTag('outbound', props.id, outbound.value.tag)) return
  loading.value = true
  const success = await Data().save('outbounds', props.id == 0 ? 'new' : 'edit', outbound.value)
  if (success) closeModal()
  loading.value = false
}

const linkConvert = async () => {
  if (!link.value.trim()) return
  loading.value = true
  const msg = await HttpUtils.post('api/linkConvert', { link: link.value.trim() })
  loading.value = false
  if (msg.success) {
    outbound.value = msg.obj
    if (props.id > 0) outbound.value.id = props.id
    link.value = ''
    refreshJson()
    ElMessage.success('已导入,请检查 tag / TLS / transport 字段')
  }
}

watch(() => props.visible, (v) => {
  if (v) updateData(props.id)
})
</script>

<style scoped>
.import-box {
  background: linear-gradient(180deg, var(--nc-primary-soft), transparent);
  border: 1px dashed var(--nc-primary);
  border-radius: var(--radius-md);
  padding: 12px 14px;
  margin-bottom: 16px;
}
.import-label {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--nc-primary-deep);
  margin-bottom: 8px;
  display: flex;
  align-items: center;
  gap: 6px;
}
.import-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 8px;
}
.hint-muted { font-size: 11.5px; color: var(--nc-text-muted); }

.ob-form { display: flex; flex-direction: column; gap: 14px; }

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 6px 16px;
}
.form-grid :deep(.form-item--full) { grid-column: 1 / -1; }

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
.form-section__head-left { display: flex; flex-direction: column; gap: 4px; flex: 1; min-width: 0; }
.form-section__hint {
  margin: 0;
  font-size: 11.5px;
  color: var(--nc-text-muted);
  line-height: 1.5;
}
.form-section__hint b { color: var(--nc-text-1); font-weight: 600; }

.transport-unsupported {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 10px 12px;
  background: var(--nc-bg-2, #f8fafc);
  border: 1px dashed var(--nc-border-soft);
  border-radius: var(--radius-sm);
  font-size: 12.5px;
  color: var(--nc-text-muted);
  line-height: 1.55;
}
.transport-unsupported .el-icon { color: var(--nc-primary); flex-shrink: 0; margin-top: 2px; }
.transport-unsupported b { color: var(--nc-text-1); font-weight: 600; }

.quick-presets {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  background: linear-gradient(180deg, rgba(124, 58, 237, 0.06), transparent);
  border: 1px dashed rgba(124, 58, 237, 0.4);
  border-radius: var(--radius-md);
  margin-bottom: 4px;
}
.quick-presets__label {
  font-size: 12.5px;
  font-weight: 600;
  color: #7c3aed;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-right: 4px;
}

.form-subsection {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px dashed var(--nc-border-soft);
}
.form-subsection__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
  font-size: 12px;
  color: var(--nc-text-muted);
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
.advanced__body { padding: 12px 14px; position: relative; }
.advanced__head {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 6px;
}

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
</style>
