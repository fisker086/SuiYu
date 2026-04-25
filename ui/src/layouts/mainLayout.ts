import axios from 'axios'
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { LocalStorage, useQuasar } from 'quasar'
import { useRouter } from 'vue-router'
import { api } from 'boot/axios'
import type { UserResponse } from 'src/api/types'
import { clearAdminCache, setAdminCache } from 'src/auth/adminCache'

const LOCALE_KEY = 'locale'

export function useMainLayout () {
  const { t, locale } = useI18n()
  const $q = useQuasar()
  const router = useRouter()
  const leftDrawer = ref(false)
  const me = ref<UserResponse | null>(null)
  const meLoaded = ref(false)

  const userLabel = computed(() => me.value?.username ?? '')
  const isAdmin = computed(() => me.value?.is_admin === true)

  /** Lark 等 SSO 同步的头像 URL，来自 /auth/me */
  const avatarUrl = computed(() => {
    const u = me.value?.avatar_url
    return typeof u === 'string' && u.trim().length > 0 ? u.trim() : ''
  })

  async function loadMe () {
    try {
      const { data } = await api.get<{ user: UserResponse }>('/auth/me')
      me.value = data.user
      setAdminCache(data.user.is_admin === true)
    } catch (e) {
      if (axios.isCancel(e)) {
        /* 另一路并发的 /auth/me 完成即可；勿清空身份 */
      } else {
        me.value = null
        clearAdminCache()
      }
    } finally {
      meLoaded.value = true
    }
  }

  function onLogout () {
    LocalStorage.remove('access')
    LocalStorage.remove('refresh')
    clearAdminCache()
    void router.replace({ path: '/login' })
  }

  function goProfile () {
    void router.push({ name: 'profile' })
  }

  onMounted(() => {
    void loadMe().catch(() => {
      $q.notify({ type: 'negative', message: t('loginFailed') })
    })
  })

  watch(locale, (v) => {
    LocalStorage.set(LOCALE_KEY, v)
    if (typeof document !== 'undefined') {
      document.documentElement.lang = v === 'zh-CN' ? 'zh-CN' : 'en'
    }
  }, { immediate: true })

  function setLocale (lang: 'zh-CN' | 'en-US') {
    locale.value = lang
  }

  return { t, locale, setLocale, leftDrawer, userLabel, avatarUrl, meLoaded, isAdmin, onLogout, goProfile }
}
