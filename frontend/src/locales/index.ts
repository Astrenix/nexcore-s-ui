import { createI18n } from 'vue-i18n'
import en from './en'

// 仅当前语言 + en fallback 同步加载;其它语言通过 loadLocale() 按需异步加载,
// 显著减小初始 bundle(每个语言 ~20-25 KB)。
const initial = localStorage.getItem('locale') ?? 'en'

const messages: Record<string, any> = { en }

export const i18n = createI18n({
  legacy: false,
  locale: initial,
  fallbackLocale: 'en',
  messages,
})

const loaders: Record<string, () => Promise<any>> = {
  en: () => Promise.resolve({ default: en }),
  fa: () => import('./fa'),
  vi: () => import('./vi'),
  zhHans: () => import('./zhcn'),
  zhHant: () => import('./zhtw'),
  ru: () => import('./ru'),
}

export async function loadLocale(lang: string) {
  if (i18n.global.availableLocales.includes(lang)) return
  const loader = loaders[lang]
  if (!loader) return
  const m = await loader()
  i18n.global.setLocaleMessage(lang, m.default || m)
}

// 启动时:若 storage 设的不是 en,先把对应语言加载好
if (initial !== 'en') {
  loadLocale(initial).then(() => {
    // 仍设置一次 locale 以触发 reactive 更新
    i18n.global.locale.value = initial
  })
}

export const locale = (() => {
  const l = i18n.global.locale.value
  switch (l) {
    case 'zhHans':
      return 'zh-cn'
    case 'zhHant':
      return 'zh-tw'
    default:
      return l
  }
})()

export const languages = [
  { title: 'English', value: 'en' },
  { title: 'فارسی', value: 'fa' },
  { title: 'Tiếng Việt', value: 'vi' },
  { title: '简体中文', value: 'zhHans' },
  { title: '繁體中文', value: 'zhHant' },
  { title: 'Русский', value: 'ru' },
]
