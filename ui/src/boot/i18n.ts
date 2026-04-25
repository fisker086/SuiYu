import { boot } from 'quasar/wrappers'
import { LocalStorage } from 'quasar'
import { createI18n } from 'vue-i18n'

import messages from 'src/i18n'

const LOCALE_KEY = 'locale'

export type MessageLanguages = keyof typeof messages
export type MessageSchema = typeof messages['zh-CN']

declare module 'vue-i18n' {
  export interface DefineLocaleMessage extends MessageSchema {}
  export interface DefineDateTimeFormat {}
  export interface DefineNumberFormat {}
}

function readStoredLocale (): MessageLanguages {
  const raw = LocalStorage.getItem<string>(LOCALE_KEY)
  if (raw === 'zh-CN' || raw === 'en-US') return raw
  return 'zh-CN'
}

export default boot(({ app }) => {
  const i18n = createI18n({
    locale: readStoredLocale(),
    fallbackLocale: 'zh-CN',
    legacy: false,
    messages
  })
  app.use(i18n)
})
