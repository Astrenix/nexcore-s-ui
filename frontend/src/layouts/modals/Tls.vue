<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="closeModal"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="$t('actions.' + title) + ' ' + $t('objects.tls')"
    destroy-on-close
  >
    <el-form label-position="top" class="tls-form">
      <!-- 顶部:名称 + 三个能力开关 -->
      <div class="form-grid">
        <el-form-item :label="$t('client.name')">
          <el-input v-model="config.name" :placeholder="$t('tls.ui.namePlaceholder')" />
        </el-form-item>
        <el-form-item label="SNI · server_name">
          <el-input v-model="config.server.server_name" :placeholder="$t('tls.ui.sniPlaceholder')" />
        </el-form-item>
      </div>

      <div class="caps">
        <label class="cap-toggle">
          <el-switch v-model="hasAcme" />
          <div>
            <span class="cap-name">{{ $t('tls.ui.acmeCapName') }}</span>
            <span class="cap-hint">{{ $t('tls.ui.acmeCapHint') }}</span>
          </div>
        </label>
        <label class="cap-toggle">
          <el-switch v-model="hasReality" />
          <div>
            <span class="cap-name">Reality</span>
            <span class="cap-hint">{{ $t('tls.ui.realityCapHint') }}</span>
          </div>
        </label>
        <label class="cap-toggle">
          <el-switch v-model="hasEch" />
          <div>
            <span class="cap-name">ECH</span>
            <span class="cap-hint">{{ $t('tls.ui.echCapHint') }}</span>
          </div>
        </label>
      </div>

      <el-tabs v-model="tab" class="tls-tabs">
        <!-- 基础(版本 / ALPN / 证书) -->
        <el-tab-pane :label="$t('tls.ui.basicTab')" name="basic">
          <div class="form-grid">
            <el-form-item :label="$t('tls.ui.minTlsVer')">
              <el-select v-model="config.server.min_version" clearable :placeholder="$t('tls.ui.defaultVer12')">
                <el-option v-for="v in TLS_VERSIONS" :key="v" :label="`TLS ${v}`" :value="v" />
              </el-select>
            </el-form-item>
            <el-form-item :label="$t('tls.ui.maxTlsVer')">
              <el-select v-model="config.server.max_version" clearable :placeholder="$t('tls.ui.defaultVer13')">
                <el-option v-for="v in TLS_VERSIONS" :key="v" :label="`TLS ${v}`" :value="v" />
              </el-select>
            </el-form-item>
            <el-form-item :label="$t('tls.ui.alpnLabel')" class="form-item--full">
              <el-input
                :model-value="(config.server.alpn || []).join(',')"
                placeholder="h2,http/1.1"
                @input="(v: string) => config.server.alpn = v ? v.split(',').map((x: string) => x.trim()) : []"
              />
              <p class="form-hint">{{ $t('tls.ui.alpnHintA') }}<code>h2,http/1.1</code>{{ $t('tls.ui.alpnHintB') }}</p>
            </el-form-item>
            <el-form-item :label="$t('tls.ui.cipherLabel')" class="form-item--full">
              <el-input
                :model-value="(config.server.cipher_suites || []).join(',')"
                :placeholder="$t('tls.ui.cipherPlaceholder')"
                @input="(v: string) => config.server.cipher_suites = v ? v.split(',').map((x: string) => x.trim()) : []"
              />
            </el-form-item>
          </div>

          <div v-if="!hasAcme && !hasReality" class="form-section">
            <h4 class="form-section__title">{{ $t('tls.ui.certManualTitle') }}</h4>
            <p class="form-hint">{{ $t('tls.ui.certManualHint') }}</p>
            <div class="form-grid">
              <el-form-item :label="$t('tls.certPath')">
                <el-input v-model="config.server.certificate_path" class="mono" placeholder="/etc/ssl/example.com.fullchain.pem" />
              </el-form-item>
              <el-form-item :label="$t('tls.keyPath')">
                <el-input v-model="config.server.key_path" class="mono" placeholder="/etc/ssl/example.com.key" />
              </el-form-item>
            </div>
            <div class="cert-inline">
              <el-form-item :label="$t('tls.ui.certContentLabel')">
                <el-input
                  :model-value="(config.server.certificate || []).join('\n')"
                  type="textarea"
                  :rows="4"
                  spellcheck="false"
                  class="mono-input"
                  placeholder="-----BEGIN CERTIFICATE-----…"
                  @input="(v: string) => config.server.certificate = v ? v.split('\n').filter(Boolean) : []"
                />
              </el-form-item>
              <el-form-item :label="$t('tls.ui.keyContentLabel')">
                <el-input
                  :model-value="(config.server.key || []).join('\n')"
                  type="textarea"
                  :rows="4"
                  spellcheck="false"
                  class="mono-input"
                  placeholder="-----BEGIN PRIVATE KEY-----…"
                  @input="(v: string) => config.server.key = v ? v.split('\n').filter(Boolean) : []"
                />
              </el-form-item>
            </div>
          </div>
        </el-tab-pane>

        <!-- ACME -->
        <el-tab-pane v-if="hasAcme" label="ACME" name="acme">
          <div class="form-grid">
            <el-form-item :label="$t('tls.ui.acmeDomainLabel')" class="form-item--full">
              <el-input
                :model-value="(config.server.acme.domain || []).join(',')"
                placeholder="api.example.com,*.example.com"
                @input="(v: string) => config.server.acme.domain = v ? v.split(',').map((x: string) => x.trim()) : []"
              />
              <p v-if="hasWildcardDomain" class="form-hint hint-info">
                {{ $t('tls.ui.wildcardHintA') }}<code>sni</code>{{ $t('tls.ui.wildcardHintB') }}<code>*</code>{{ $t('tls.ui.wildcardHintC') }}<code>vless-15414.example.com</code>{{ $t('tls.ui.wildcardHintD') }}<code>server</code>{{ $t('tls.ui.wildcardHintE') }}
              </p>
            </el-form-item>
            <el-form-item :label="$t('tls.ui.contactEmail')">
              <el-input v-model="config.server.acme.email" placeholder="admin@example.com" />
            </el-form-item>
            <el-form-item :label="$t('tls.ui.acmeProvider')">
              <el-select v-model="config.server.acme.provider" clearable :placeholder="$t('tls.ui.defaultLetsencrypt')">
                <el-option :label="$t('tls.ui.providerLE')" value="letsencrypt" />
                <el-option :label="$t('tls.ui.providerLEStaging')" value="letsencrypt-staging" />
                <el-option label="ZeroSSL" value="zerossl" />
                <el-option label="Buypass" value="buypass" />
              </el-select>
            </el-form-item>
            <el-form-item :label="$t('tls.ui.certDataDir')">
              <el-input v-model="config.server.acme.data_directory" class="mono" :placeholder="$t('tls.ui.auto')" />
            </el-form-item>
          </div>
          <div class="form-grid">
            <el-form-item :label="$t('tls.ui.disableHttp01')">
              <el-switch v-model="config.server.acme.disable_http_challenge" />
            </el-form-item>
            <el-form-item :label="$t('tls.ui.disableTlsAlpn')">
              <el-switch v-model="config.server.acme.disable_tls_alpn_challenge" />
            </el-form-item>
            <el-form-item :label="$t('tls.ui.httpAltPort')">
              <el-input-number v-model="config.server.acme.alternative_http_port" :min="0" :max="65535" controls-position="right" style="width: 100%" />
            </el-form-item>
            <el-form-item :label="$t('tls.ui.tlsAltPort')">
              <el-input-number v-model="config.server.acme.alternative_tls_port" :min="0" :max="65535" controls-position="right" style="width: 100%" />
            </el-form-item>
          </div>
          <div class="form-section">
            <h4 class="form-section__title">{{ $t('tls.ui.dns01Title') }}</h4>
            <p class="form-hint">{{ $t('tls.ui.dns01HintA') }}<code>cloudflare_api_token</code>{{ $t('tls.ui.dns01HintB') }}</p>
            <el-form-item>
              <el-input
                :model-value="config.server.acme.dns01_challenge ? JSON.stringify(config.server.acme.dns01_challenge, null, 2) : ''"
                type="textarea"
                :rows="6"
                spellcheck="false"
                class="mono-input"
                placeholder='{"provider":"cloudflare","cloudflare_api_token":"..."}'
                @input="(v: string) => setDnsChallenge(v)"
              />
            </el-form-item>
          </div>
        </el-tab-pane>

        <!-- Reality -->
        <el-tab-pane v-if="hasReality" label="Reality" name="reality">
          <div class="form-grid">
            <el-form-item :label="$t('tls.ui.realityHandshakeServer')">
              <el-input v-model="config.server.reality.handshake.server" :placeholder="$t('inbound.handshakeServerPlaceholder')" />
              <p class="form-hint">{{ $t('tls.ui.realityHandshakeHint') }}</p>
            </el-form-item>
            <el-form-item :label="$t('inbound.handshakePort')">
              <el-input-number v-model="config.server.reality.handshake.server_port" :min="1" :max="65535" controls-position="right" style="width: 100%" />
            </el-form-item>
            <el-form-item :label="$t('tls.ui.realityPrivKey')">
              <el-input v-model="config.server.reality.private_key" class="mono">
                <template #append>
                  <el-button @click="genReality"><el-icon><Refresh /></el-icon>{{ $t('actions.generate') }}</el-button>
                </template>
              </el-input>
              <p v-if="lastPub" class="form-hint">
                {{ $t('tls.ui.realityPubKeyHint') }}<code class="mono select-all">{{ lastPub }}</code>
              </p>
            </el-form-item>
            <el-form-item :label="$t('tls.ui.shortIdLabel')">
              <el-input
                :model-value="(config.server.reality.short_id || []).join(',')"
                :placeholder="$t('tls.ui.shortIdPlaceholder')"
                @input="(v: string) => config.server.reality.short_id = v ? v.split(',').map((x: string) => x.trim()) : []"
              />
            </el-form-item>
            <el-form-item :label="$t('tls.ui.maxTimeDiff')">
              <el-input v-model="config.server.reality.max_time_difference" :placeholder="$t('tls.ui.maxTimeDiffPlaceholder')" />
            </el-form-item>
          </div>
        </el-tab-pane>

        <!-- ECH -->
        <el-tab-pane v-if="hasEch" label="ECH" name="ech">
          <div class="form-grid">
            <el-form-item :label="$t('tls.ui.enableEch')" class="form-item--full">
              <el-switch v-model="config.server.ech.enabled" />
            </el-form-item>
            <el-form-item :label="$t('tls.ui.echKeyPath')">
              <el-input v-model="config.server.ech.key_path" class="mono" placeholder="/etc/sing-box/ech.key" />
            </el-form-item>
          </div>
          <el-form-item :label="$t('tls.ui.echKeyContent')">
            <el-input
              :model-value="(config.server.ech.key || []).join('\n')"
              type="textarea"
              :rows="4"
              spellcheck="false"
              class="mono-input"
              @input="(v: string) => config.server.ech.key = v ? v.split('\n').filter(Boolean) : []"
            />
          </el-form-item>
        </el-tab-pane>

        <!-- 客户端校验(出站引用此 TLS 时拷贝过去) -->
        <el-tab-pane :label="$t('tls.ui.clientTab')" name="client">
          <p class="form-hint">{{ $t('tls.ui.clientHint') }}</p>
          <div class="form-grid">
            <el-form-item :label="$t('tls.ui.clientSni')">
              <el-input v-model="config.client.server_name" :placeholder="$t('tls.ui.clientSniPlaceholder')" />
            </el-form-item>
            <el-form-item :label="$t('tls.ui.allowInsecure')">
              <el-switch v-model="config.client.insecure" />
            </el-form-item>
            <el-form-item :label="$t('tls.ui.clientAlpn')" class="form-item--full">
              <el-input
                :model-value="(config.client.alpn || []).join(',')"
                @input="(v: string) => config.client.alpn = v ? v.split(',').map((x: string) => x.trim()) : []"
              />
            </el-form-item>
            <el-form-item :label="$t('tls.ui.utlsFingerprint')">
              <el-select :model-value="config.client.utls?.fingerprint" clearable :placeholder="$t('tls.ui.notEnabled')" @change="(v: string) => setUtls(v)">
                <el-option v-for="fp in UTLS_FPS" :key="fp" :label="fp" :value="fp" />
              </el-select>
            </el-form-item>
          </div>
        </el-tab-pane>

        <!-- 高级:JSON -->
        <el-tab-pane label="JSON" name="json">
          <p class="form-hint">{{ $t('tls.ui.jsonHint') }}</p>
          <JsonEditorBlock :data="config" :rows="20" @update:data="(v: any) => (config = v)" />
        </el-tab-pane>
      </el-tabs>
    </el-form>

    <template #footer>
      <el-button @click="closeModal">{{ $t('actions.close') }}</el-button>
      <el-button type="primary" :loading="loading" @click="saveChanges">{{ $t('actions.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, watch, computed } from 'vue'
import type { tls } from '@/types/tls'
import JsonEditorBlock from '@/components/JsonEditorBlock.vue'
import HttpUtils from '@/plugins/httputil'
import { Refresh } from '@element-plus/icons-vue'

const props = defineProps<{ visible: boolean; id: number; data: string }>()
const emit = defineEmits<{ close: []; save: [data: tls]; 'update:modelValue': [v: boolean] }>()

const config = ref<any>({ name: '', server: { server_name: '' }, client: {} })
const title = ref<'add' | 'edit'>('add')
const loading = ref(false)
const tab = ref('basic')
const lastPub = ref('') // 最近一次生成的 Reality public_key,展示给用户复制给客户端

const TLS_VERSIONS = ['1.0', '1.1', '1.2', '1.3']
const UTLS_FPS = ['chrome', 'firefox', 'safari', 'ios', 'android', 'edge', 'random', 'randomized']

const hasAcme = computed({
  get: () => !!config.value.server?.acme,
  set: (v: boolean) => {
    if (!config.value.server) config.value.server = {}
    if (v) config.value.server.acme = { domain: [], email: '', provider: 'letsencrypt' }
    else delete config.value.server.acme
    if (v) tab.value = 'acme'
  },
})
// 通配符提示:绑定此 TLS 的入站生成的分享链接,sni 字段会被后端
// (util/genLink.go::wildcardSniFromAcme)用 inbound.tag 替换 *。
const hasWildcardDomain = computed((): boolean =>
  ((config.value.server?.acme?.domain as string[] | undefined) || []).some((d: string) => d.startsWith('*.')),
)
const hasReality = computed({
  get: () => !!config.value.server?.reality,
  set: (v: boolean) => {
    if (!config.value.server) config.value.server = {}
    if (v) config.value.server.reality = { enabled: true, handshake: { server: '', server_port: 443 }, private_key: '', short_id: [''] }
    else delete config.value.server.reality
    if (v) tab.value = 'reality'
  },
})
const hasEch = computed({
  get: () => !!config.value.server?.ech,
  set: (v: boolean) => {
    if (!config.value.server) config.value.server = {}
    if (v) config.value.server.ech = { enabled: true }
    else delete config.value.server.ech
    if (v) tab.value = 'ech'
  },
})

const setDnsChallenge = (raw: string) => {
  if (!raw.trim()) {
    delete config.value.server.acme.dns01_challenge
    return
  }
  try {
    config.value.server.acme.dns01_challenge = JSON.parse(raw)
  } catch {
    /* 实时键入时静默,等用户写完才会被合法解析 */
  }
}

const setUtls = (v: string) => {
  if (!v) {
    delete config.value.client.utls
  } else {
    config.value.client.utls = { enabled: true, fingerprint: v }
  }
}

// 一键调后端 keypairs 接口生成 X25519 keypair,private_key 写到 server.reality,
// public_key 暂存到 lastPub 让用户复制给客户端。
const genReality = async () => {
  const r = await HttpUtils.get('api/keypairs', { k: 'reality' })
  if (r.success && r.obj) {
    const obj = r.obj
    config.value.server.reality.private_key = obj.PrivateKey || obj.private_key || ''
    lastPub.value = obj.PublicKey || obj.public_key || ''
  }
}

const updateData = (id: number) => {
  if (id > 0) {
    config.value = JSON.parse(props.data || '{}')
    if (!config.value.server) config.value.server = {}
    if (!config.value.client) config.value.client = {}
    title.value = 'edit'
  } else {
    config.value = {
      name: 'tls-' + Math.random().toString(36).slice(2, 6),
      server: { server_name: '', alpn: ['h2', 'http/1.1'], min_version: '1.2', max_version: '1.3' },
      client: { alpn: ['h2', 'http/1.1'] },
    }
    title.value = 'add'
  }
  lastPub.value = ''
  tab.value = 'basic'
}

const closeModal = () => emit('close')

const saveChanges = async () => {
  loading.value = true
  emit('save', config.value as tls)
  loading.value = false
}

watch(() => props.visible, (v) => { if (v) updateData(props.id) })
</script>

<style scoped>
.tls-form { display: flex; flex-direction: column; gap: 14px; }

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 6px 16px;
}
.form-grid :deep(.form-item--full) { grid-column: 1 / -1; }

.caps {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 8px;
}
.cap-toggle {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid var(--nc-border-soft);
  border-radius: var(--radius-md);
  background: #fafbfc;
  cursor: pointer;
}
.cap-toggle div { display: flex; flex-direction: column; gap: 2px; }
.cap-name { font-size: 13px; font-weight: 600; color: var(--nc-text-1); }
.cap-hint { font-size: 11.5px; color: var(--nc-text-muted); line-height: 1.4; }

.tls-tabs :deep(.el-tabs__header) { margin-bottom: 12px; }

.form-section {
  margin-top: 8px;
  padding: 12px 14px;
  background: var(--nc-bg-3);
  border: 1px solid var(--nc-border-soft);
  border-radius: var(--radius-md);
}
.form-section__title {
  margin: 0 0 8px;
  font-size: 12.5px;
  font-weight: 600;
  color: var(--nc-text-1);
}

.form-hint {
  font-size: 12px;
  color: var(--nc-text-muted);
  margin: 4px 0;
  line-height: 1.5;
}
.form-hint.hint-info {
  color: var(--nc-text-1);
  background: var(--nc-primary-soft);
  border-left: 3px solid var(--nc-primary);
  padding: 8px 10px;
  border-radius: 4px;
}
.form-hint code {
  font-family: var(--font-mono);
  font-size: 11.5px;
  background: var(--nc-bg-3);
  padding: 1px 6px;
  border-radius: 4px;
  color: var(--nc-text-1);
}

.cert-inline {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 8px;
}

.mono-input :deep(.el-textarea__inner),
.mono-input :deep(.el-input__inner) {
  font-family: var(--font-mono);
  font-size: 12px;
}

.select-all { user-select: all; }
</style>
