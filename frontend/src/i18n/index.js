import { createI18n } from 'vue-i18n'
import zh from './locales/zh'
import en from './locales/en'

const saved = localStorage.getItem('vg.lang') || 'zh'

const i18n = createI18n({
  legacy: false,
  locale: saved,
  fallbackLocale: 'zh',
  messages: { zh, en },
})

export function setLocale(lang) {
  i18n.global.locale.value = lang
  localStorage.setItem('vg.lang', lang)
  document.documentElement.lang = lang
}

export default i18n
