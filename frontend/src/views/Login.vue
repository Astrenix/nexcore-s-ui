<template>
  <div class="login-page">
    <div class="login-page__bg" aria-hidden="true">
      <div class="login-page__glow login-page__glow--a"></div>
      <div class="login-page__glow login-page__glow--b"></div>
    </div>

    <div class="login-page__content">
      <div class="login-page__brand">
        <img src="@/assets/logo.svg" alt="S-UI" class="login-page__logo" />
        <span class="login-page__brand-name">S-UI</span>
      </div>

      <div class="login-card">
        <div class="login-card__head">
          <h1 class="login-card__title">{{ $t('login.title') }}</h1>
          <p class="login-card__subtitle">{{ $t('login.subtitle', $t('login.title')) }}</p>
        </div>

        <el-form
          ref="formRef"
          :model="form"
          :rules="rules"
          label-position="top"
          @submit.prevent="onLogin"
          class="login-form"
        >
          <el-form-item :label="$t('login.username')" prop="username">
            <el-input v-model="form.username" :placeholder="$t('login.username')" autocomplete="username">
              <template #prefix><el-icon><User /></el-icon></template>
            </el-input>
          </el-form-item>

          <el-form-item :label="$t('login.password')" prop="password">
            <el-input
              v-model="form.password"
              type="password"
              show-password
              :placeholder="$t('login.password')"
              autocomplete="current-password"
              @keyup.enter="onLogin"
            >
              <template #prefix><el-icon><Lock /></el-icon></template>
            </el-input>
          </el-form-item>

          <el-button
            type="primary"
            native-type="submit"
            :loading="loading"
            class="login-form__submit"
            @click="onLogin"
          >
            {{ $t('actions.submit') }}
          </el-button>
        </el-form>

        <div class="login-card__foot">
          <el-select v-model="i18nLocale" size="small" @change="changeLocale" class="login-card__lang">
            <el-option v-for="lang in languages" :key="lang.value" :value="lang.value" :label="lang.title" />
          </el-select>
          <el-dropdown trigger="click" @command="changeTheme">
            <button class="login-card__theme-btn" type="button" :aria-label="$t('theme.system')">
              <el-icon><Sunny v-if="theme === 'light'" /><Moon v-else-if="theme === 'dark'" /><Monitor v-else /></el-icon>
            </button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="light"><el-icon style="margin-right: 8px"><Sunny /></el-icon>{{ $t('theme.light') }}</el-dropdown-item>
                <el-dropdown-item command="dark"><el-icon style="margin-right: 8px"><Moon /></el-icon>{{ $t('theme.dark') }}</el-dropdown-item>
                <el-dropdown-item command="system"><el-icon style="margin-right: 8px"><Monitor /></el-icon>{{ $t('theme.system') }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </div>

      <div class="login-page__hint">
        <span class="status-dot online"></span>
        <span>{{ $t('login.hint', 'Sing-Box Web Panel') }}</span>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { i18n, languages } from '@/locales'
import { applyEPLocale } from '@/plugins/element-plus'
import HttpUtil from '@/plugins/httputil'
import type { FormInstance, FormRules } from 'element-plus'
import { User, Lock, Sunny, Moon, Monitor } from '@element-plus/icons-vue'

const router = useRouter()
const { locale: i18nLocale } = useI18n()

const formRef = ref<FormInstance>()
const form = reactive({ username: '', password: '' })

const rules: FormRules = {
  username: [{ required: true, message: i18n.global.t('login.unRules'), trigger: 'blur' }],
  password: [{ required: true, message: i18n.global.t('login.pwRules'), trigger: 'blur' }],
}

const loading = ref(false)

const onLogin = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    loading.value = true
    try {
      const resp = await HttpUtil.post('api/login', { user: form.username, pass: form.password })
      if (resp.success) {
        setTimeout(() => router.push('/'), 350)
      }
    } finally {
      setTimeout(() => { loading.value = false }, 350)
    }
  })
}

const changeLocale = (l: any) => {
  i18nLocale.value = l ?? 'en'
  localStorage.setItem('locale', i18nLocale.value)
  applyEPLocale(i18nLocale.value)
  window.location.reload()
}

const theme = ref(localStorage.getItem('theme') ?? 'system')
const changeTheme = (th: string) => {
  theme.value = th
  localStorage.setItem('theme', th)
  const html = document.documentElement
  if (th === 'dark') html.classList.add('dark')
  else if (th === 'light') html.classList.remove('dark')
  else html.classList.toggle('dark', window.matchMedia('(prefers-color-scheme: dark)').matches)
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  overflow: hidden;
  padding: 24px;
  background: linear-gradient(160deg, #f8fafc 0%, #eff6ff 60%, #ffffff 100%);
}

.login-page__bg {
  position: absolute;
  inset: 0;
  pointer-events: none;
  overflow: hidden;
}

.login-page__glow {
  position: absolute;
  width: 520px;
  height: 520px;
  border-radius: 50%;
  filter: blur(90px);
  opacity: 0.42;
}

.login-page__glow--a {
  background: radial-gradient(circle, #3b82f6 0%, transparent 70%);
  top: -160px;
  left: -120px;
}

.login-page__glow--b {
  background: radial-gradient(circle, #60a5fa 0%, transparent 70%);
  bottom: -180px;
  right: -160px;
}

.login-page__content {
  position: relative;
  width: 100%;
  max-width: 400px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 24px;
}

.login-page__brand {
  display: flex;
  align-items: center;
  gap: 10px;
}

.login-page__logo {
  width: 36px;
  height: 36px;
}

.login-page__brand-name {
  font-family: var(--font-display);
  font-weight: 700;
  font-size: 20px;
  letter-spacing: -0.01em;
  color: var(--nc-text-1);
}

.login-card {
  width: 100%;
  background: #fff;
  border: 1px solid var(--nc-border);
  border-radius: 14px;
  box-shadow: var(--shadow-md);
  padding: 28px 28px 20px;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.login-card__head {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.login-card__title {
  font-size: 22px;
  font-weight: 700;
  font-family: var(--font-display);
  color: var(--nc-text-1);
  letter-spacing: -0.02em;
  margin: 0;
}

.login-card__subtitle {
  font-size: 13px;
  color: var(--nc-text-muted);
  margin: 0;
}

.login-form :deep(.el-form-item) {
  margin-bottom: 14px;
}
.login-form :deep(.el-form-item__label) {
  font-size: 12.5px;
  color: var(--nc-text-3);
  padding-bottom: 4px;
}
.login-form :deep(.el-input__wrapper) {
  border-radius: 10px;
  height: 42px;
}
.login-form :deep(.el-input__inner) {
  font-size: 14px;
}

.login-form__submit {
  width: 100%;
  height: 44px;
  font-size: 14px;
  font-weight: 600;
  border-radius: 10px;
  letter-spacing: 0.02em;
}

.login-card__foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-top: 4px;
  border-top: 1px solid var(--nc-border-soft);
  margin-top: 4px;
  padding-top: 12px;
}

.login-card__lang {
  width: 140px;
}

.login-card__theme-btn {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-md);
  background: transparent;
  border: 1px solid var(--nc-border);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--nc-text-3);
  transition: background var(--t-fast);
}

.login-card__theme-btn:hover {
  background: var(--nc-border-soft);
}

.login-page__hint {
  font-size: 12px;
  color: var(--nc-text-muted);
  display: flex;
  align-items: center;
  gap: 6px;
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  display: inline-block;
}
.status-dot.online {
  background: var(--nc-success);
  box-shadow: 0 0 0 3px rgba(22, 163, 74, 0.15);
}

@media (max-width: 480px) {
  .login-card {
    padding: 22px 20px 16px;
  }
  .login-card__title { font-size: 19px; }
}
</style>
