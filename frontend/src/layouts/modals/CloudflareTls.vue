<template>
  <el-dialog
    :model-value="visible"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="onClose"
    class="constrained-dialog is-medium"
    :align-center="false"
    :title="mode === 'panel' ? $t('tls.cf.panelTitle') : $t('tls.cf.title')"
    destroy-on-close
  >
    <el-steps :active="step" finish-status="success" simple class="cf-steps">
      <el-step :title="$t('tls.cf.step1')" />
      <el-step :title="$t('tls.cf.step2')" />
      <el-step :title="$t('tls.cf.step3')" />
    </el-steps>

    <el-form label-position="top" v-if="step === 0">
      <el-alert v-if="savedHint" type="success" :closable="false" show-icon class="cf-alert">
        <template #title>{{ $t('tls.cf.tokenLoaded') }}</template>
      </el-alert>
      <el-alert v-else :title="$t('tls.cf.tokenHint')" type="info" :closable="false" show-icon class="cf-alert" />
      <el-form-item :label="$t('tls.cf.token')">
        <el-input
          v-model="form.token"
          show-password
          type="password"
          :placeholder="$t('tls.cf.tokenPlaceholder')"
        />
      </el-form-item>
      <el-form-item :label="$t('tls.cf.email')">
        <el-input v-model="form.email" :placeholder="`admin@${form.fqdn || 'example.com'}`" />
      </el-form-item>
      <el-form-item>
        <el-checkbox v-model="form.persist">{{ $t('tls.cf.persistLabel') }}</el-checkbox>
        <p class="form-hint">{{ $t('tls.cf.persistHint') }}</p>
        <el-button v-if="savedHint" size="small" link type="danger" @click="clearSaved">{{ $t('tls.cf.clearSaved') }}</el-button>
      </el-form-item>
    </el-form>

    <el-form label-position="top" v-if="step === 1">
      <el-form-item :label="$t('tls.cf.zone')">
        <el-select v-model="form.zoneId" filterable :placeholder="$t('tls.cf.zonePlaceholder')" style="width:100%">
          <el-option v-for="z in zones" :key="z.id" :label="z.name" :value="z.id">
            <span class="zone-row">
              <span>{{ z.name }}</span>
              <el-tag size="small" :type="z.status === 'active' ? 'success' : 'warning'" effect="plain">{{ z.status }}</el-tag>
            </span>
          </el-option>
        </el-select>
      </el-form-item>
      <el-form-item :label="$t('tls.cf.prefixMode')">
        <el-radio-group v-model="form.prefixMode">
          <el-radio v-if="mode !== 'panel'" value="wildcard">{{ $t('tls.cf.prefixWildcard') }}</el-radio>
          <el-radio value="random">{{ $t('tls.cf.prefixRandom') }}</el-radio>
          <el-radio value="custom">{{ $t('tls.cf.prefixCustom') }}</el-radio>
          <el-radio v-if="mode !== 'panel'" value="apex">{{ $t('tls.cf.prefixApex') }}</el-radio>
        </el-radio-group>
        <p v-if="mode === 'panel'" class="form-hint">{{ $t('tls.cf.panelSubdomainHint') }}</p>
      </el-form-item>
      <el-form-item v-if="form.prefixMode === 'wildcard'" :label="$t('tls.cf.wildcardLabel')">
        <el-input v-model="form.wildcardLabel" :placeholder="$t('tls.cf.wildcardLabelPlaceholder')">
          <template #append>.{{ zoneNameOf(form.zoneId) }}</template>
        </el-input>
        <p class="form-hint">{{ $t('tls.cf.wildcardLabelHint') }}</p>
        <p v-if="form.wildcardLabel" class="form-hint mono">→ *.{{ form.wildcardLabel }}.{{ zoneNameOf(form.zoneId) }}</p>
      </el-form-item>
      <el-form-item v-if="form.prefixMode === 'random'" :label="$t('tls.cf.prefixSeed')">
        <el-input v-model="form.prefix" :placeholder="$t('tls.cf.prefixSeedPlaceholder')" />
        <p class="form-hint">{{ $t('tls.cf.prefixSeedHint') }}</p>
      </el-form-item>
      <el-form-item v-if="form.prefixMode === 'custom'" :label="$t('tls.cf.subdomain')">
        <el-input v-model="form.customName" :placeholder="`api.${zoneNameOf(form.zoneId)}`" />
      </el-form-item>
      <el-form-item :label="$t('tls.cf.ip')">
        <el-input v-model="form.ip" :placeholder="$t('tls.cf.ipPlaceholder')">
          <template #append>
            <el-button @click="detectIp">{{ $t('tls.cf.detectIp') }}</el-button>
          </template>
        </el-input>
      </el-form-item>
      <el-form-item v-if="form.prefixMode !== 'wildcard'">
        <el-checkbox v-model="form.proxied">{{ $t('tls.cf.proxied') }}</el-checkbox>
        <p class="form-hint">{{ $t('tls.cf.proxiedHint') }}</p>
      </el-form-item>
    </el-form>

    <el-form label-position="top" v-if="step === 2">
      <!-- inbound 模式:让用户给 TLS 配置起个名字 -->
      <el-form-item v-if="mode !== 'panel'" :label="$t('tls.cf.tlsName')">
        <el-input v-model="form.tlsName" :placeholder="result.fqdn || 'cf-auto'" />
      </el-form-item>
      <el-alert v-if="result.fqdn" type="success" :closable="false" show-icon class="cf-alert">
        <template #title>
          <span>{{ $t('tls.cf.dnsReady') }}: <span class="mono">{{ result.fqdn }} → {{ form.ip }}</span></span>
        </template>
      </el-alert>
      <p v-if="mode === 'panel'" class="form-hint">
        {{ $t('tls.cf.panelIssueExplain') }}
        {{ $t('tls.cf.panelIssueRedirect') }}<code class="mono">https://{{ result.fqdn }}</code>
      </p>
      <p v-else class="form-hint">{{ $t('tls.cf.issueExplain') }}</p>
    </el-form>

    <template #footer>
      <el-button @click="onClose">{{ $t('actions.close') }}</el-button>
      <el-button v-if="step > 0" @click="step--">{{ $t('tls.cf.back') }}</el-button>
      <el-button v-if="step === 0" type="primary" :loading="loading" :disabled="!form.token || !form.email" @click="onVerify">
        {{ $t('tls.cf.verifyContinue') }}
      </el-button>
      <el-button v-if="step === 1" type="primary" :loading="loading" :disabled="!canApplyDns" @click="onApplyDns">
        {{ $t('tls.cf.applyDns') }}
      </el-button>
      <el-button v-if="step === 2 && mode !== 'panel'" type="primary" :loading="loading" :disabled="!result.fqdn" @click="onIssue">
        {{ $t('tls.cf.issue') }}
      </el-button>
      <el-button v-if="step === 2 && mode === 'panel'" type="primary" :loading="loading" :disabled="!result.fqdn" @click="onIssuePanel">
        {{ $t('tls.cf.issueEnableHttps') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, computed, onMounted, watch } from 'vue'
import HttpUtils from '@/plugins/httputil'
import { ElMessage } from 'element-plus'
import { i18n } from '@/locales'
import Data from '@/store/modules/data'

// mode='inbound'(默认):为 sing-box 入站签证书,生成 model.Tls 行
// mode='panel': 为面板自己签 SSL,自动写 webCertFile/webKeyFile/webDomain + 重启
const props = withDefaults(defineProps<{ visible: boolean; mode?: 'inbound' | 'panel' }>(), {
  mode: 'inbound',
})
const emit = defineEmits<{ close: []; 'update:modelValue': [v: boolean]; created: [id: number] }>()

const step = ref(0)
const loading = ref(false)

interface Form {
  token: string
  email: string
  zoneId: string
  prefixMode: 'wildcard' | 'random' | 'custom' | 'apex'
  prefix: string
  customName: string
  wildcardLabel: string
  ip: string
  proxied: boolean
  fqdn: string
  tlsName: string
  persist: boolean
}

const initialForm = (): Form => ({
  token: '',
  email: '',
  zoneId: '',
  // 默认通配符 — 机场场景一证多入站,后续不用再调 CF
  prefixMode: 'wildcard',
  prefix: '',
  customName: '',
  wildcardLabel: '',
  ip: '',
  proxied: false,
  fqdn: '',
  tlsName: '',
  persist: true,
})

const savedHint = ref(false)

const loadSavedCreds = async () => {
  const r = await HttpUtils.get('api/cfCredentials')
  if (r.success && r.obj?.saved) {
    form.value.token = r.obj.token || ''
    form.value.email = r.obj.email || ''
    savedHint.value = true
  } else {
    savedHint.value = false
  }
}

const clearSaved = async () => {
  await HttpUtils.post('api/cfSetCredentials', { clear: '1' })
  form.value.token = ''
  form.value.email = ''
  savedHint.value = false
  ElMessage.success(i18n.global.t('tls.cf.savedCleared'))
}

const persistIfChecked = async () => {
  if (!form.value.persist) return
  await HttpUtils.post('api/cfSetCredentials', {
    token: form.value.token,
    email: form.value.email,
  })
  savedHint.value = true
}

const form = ref<Form>(initialForm())
const zones = ref<any[]>([])
const result = ref<{ fqdn: string; recordId: string }>({ fqdn: '', recordId: '' })

const zoneNameOf = (id: string) => zones.value.find((z) => z.id === id)?.name || 'example.com'

const canApplyDns = computed(() => {
  if (!form.value.zoneId || !form.value.ip) return false
  if (form.value.prefixMode === 'custom' && !form.value.customName) return false
  if (form.value.prefixMode === 'wildcard' && !form.value.wildcardLabel.trim()) return false
  return true
})

const onClose = () => {
  emit('close')
  emit('update:modelValue', false)
}

// CF Dashboard 复制粘贴 token 时常带空格 / 换行 / 引号,后端虽然兜底
// 清洗,但前端先净一遍能让 v-model 显示的就是真实送达 CF 的内容,
// 用户能立刻看出是否粘错。
const cleanToken = (raw: string) => raw.trim().replace(/^[Bb]earer\s+/, '').replace(/^["'“”‘’]+|["'“”‘’]+$/g, '').trim()

const onVerify = async () => {
  form.value.token = cleanToken(form.value.token)
  loading.value = true
  const r = await HttpUtils.post('api/cfListZones', { token: form.value.token })
  loading.value = false
  if (r.success) {
    zones.value = r.obj || []
    if (zones.value.length === 0) {
      ElMessage.warning(i18n.global.t('tls.cf.noZone'))
      return
    }
    // token 验证成功才入库,避免存到错误的 token
    await persistIfChecked()
    if (zones.value.length === 1) form.value.zoneId = zones.value[0].id
    step.value = 1
  }
}

const detectIp = async () => {
  // 必须走后端 — 前端 fetch 拿到的是用户浏览器的出口 IP(家宽/跳板/CDN),
  // 不是面板服务器的公网 IP。后端 /api/cfDetectIp 在服务器侧并发查多个
  // echo-IP 服务,返回首个成功的。
  loading.value = true
  const r = await HttpUtils.get('api/cfDetectIp')
  loading.value = false
  if (r.success && r.obj?.ip) {
    form.value.ip = r.obj.ip
  } else {
    ElMessage.error(i18n.global.t('tls.cf.detectIpFailed'))
  }
}

const onApplyDns = async () => {
  form.value.token = cleanToken(form.value.token)
  loading.value = true
  const payload: any = {
    token: form.value.token,
    zoneId: form.value.zoneId,
    ip: form.value.ip,
    proxied: form.value.proxied,
  }
  if (form.value.prefixMode === 'random') {
    payload.random = true
    payload.prefix = form.value.prefix
  } else if (form.value.prefixMode === 'custom') {
    payload.name = form.value.customName
  } else if (form.value.prefixMode === 'wildcard') {
    // wildcard A 记录:name = *.<label>。CF 不允许 wildcard 记录走橙色云朵反代,
    // 所以这里强制 proxied=false 覆盖用户勾选(UI 层已隐藏复选框)。
    payload.name = '*.' + form.value.wildcardLabel.trim()
    payload.proxied = false
  } else {
    payload.name = '@'
  }
  const r = await HttpUtils.post('api/cfUpsertA', payload)
  loading.value = false
  if (r.success) {
    result.value = r.obj
    form.value.fqdn = r.obj.fqdn
    if (!form.value.tlsName) form.value.tlsName = r.obj.fqdn
    step.value = 2
  }
}

const onIssue = async () => {
  form.value.token = cleanToken(form.value.token)
  loading.value = true
  const r = await HttpUtils.post('api/cfIssueTls', {
    name: form.value.tlsName,
    fqdn: result.value.fqdn,
    email: form.value.email,
    token: form.value.token,
  })
  loading.value = false
  if (r.success) {
    ElMessage.success(i18n.global.t('tls.cf.issueSuccess'))
    emit('created', r.obj.id)
    // 立即重新加载,前端立即看到新的 TLS 卡片
    await Data().loadData(0)
    onClose()
  }
}

// 面板 SSL 签发 — 后端 panelSslIssue 一站式:
// 跑 ACME DNS-01(CF Token 已存)+ 写 webCertFile/webKeyFile/webDomain
// + 异步重启面板。签发后跳转 https。
const onIssuePanel = async () => {
  loading.value = true
  const tip = ElMessage({
    type: 'info',
    duration: 0,
    showClose: false,
    message: i18n.global.t('tls.cf.panelIssuing', { fqdn: result.value.fqdn }),
  })
  try {
    const r = await HttpUtils.post('api/panelSslIssue', { domain: result.value.fqdn })
    if (r.success) {
      ElMessage.success(i18n.global.t('tls.cf.panelIssueSuccess', { fqdn: result.value.fqdn }))
      // 让用户看一会成功提示再跳。port/path 从当前 URL 推 — 仍是同一台机器
      const port = window.location.port || '3095'
      const path = window.location.pathname.startsWith('/app') ? '/app/' : '/'
      setTimeout(() => {
        window.location.href = `https://${result.value.fqdn}:${port}${path}`
      }, 4000)
      onClose()
    }
  } finally {
    tip.close()
    loading.value = false
  }
}

watch(() => props.visible, (v) => {
  if (v) {
    step.value = 0
    form.value = initialForm()
    zones.value = []
    result.value = { fqdn: '', recordId: '' }
    loadSavedCreds()
  }
})

// onMounted 兜底:调用方用 v-if 渲染 wizard 时(如入站编辑里的「自动签发」),
// 组件首次挂载时 props.visible 已经是 true,但 watch 没有 immediate,
// loadSavedCreds 永远不会跑 → 表单显示空白。这里 onMounted 强制跑一次。
onMounted(() => {
  if (props.visible) loadSavedCreds()
})
</script>

<style scoped>
.cf-steps { margin-bottom: 18px; }
.cf-alert { margin-bottom: 12px; }
.zone-row { display: flex; align-items: center; justify-content: space-between; gap: 8px; width: 100%; }
.form-hint { margin: 4px 0 0; font-size: 12px; color: var(--nc-text-muted); }
.mono { font-family: var(--font-mono); }
</style>
