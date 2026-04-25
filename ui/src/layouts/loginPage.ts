import { reactive, ref, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { pickRandomSlogan } from 'src/data/loginSlogans'
import { LocalStorage, useQuasar } from 'quasar'
import { useRoute, useRouter } from 'vue-router'
import { api, fetchAuthConfig } from 'boot/axios'
import type { TokenResponse, UserResponse } from 'src/api/types'
import { clearAdminCache, setAdminCache } from 'src/auth/adminCache'

function firstQuery (q: unknown): string | undefined {
  if (Array.isArray(q)) {
    return typeof q[0] === 'string' ? q[0] : undefined
  }
  return typeof q === 'string' && q.length > 0 ? q : undefined
}

/** Safe in-app path after login; avoids loop or external URLs breaking vue-router. */
function resolvePostLoginPath (next: unknown): string {
  const raw = Array.isArray(next) ? next[0] : next
  if (typeof raw !== 'string' || raw.length === 0) {
    return '/dashboard'
  }
  const noHash = raw.split('#')[0]
  if (noHash.startsWith('http://') || noHash.startsWith('https://')) {
    try {
      const u = new URL(noHash)
      const path = u.pathname + u.search
      if (path.startsWith('/')) {
        const base = path.split('?')[0]
        if (base !== '/login' && base !== '/register') {
          return path
        }
      }
    } catch {
      /* ignore */
    }
    return '/dashboard'
  }
  if (!noHash.startsWith('/')) {
    return '/dashboard'
  }
  const base = noHash.split('?')[0]
  if (base === '/login' || base === '/register') {
    return '/dashboard'
  }
  return noHash
}

export function useLoginPage () {
  const { t, locale } = useI18n()
  const slogan = ref(pickRandomSlogan(String(locale.value)))
  watch(locale, (v) => {
    slogan.value = pickRandomSlogan(String(v))
  })
  const $q = useQuasar()
  const route = useRoute()
  const router = useRouter()
  const loading = ref(false)
  const captchaLoading = ref(false)
  const captchaToken = ref('')
  const captchaImage = ref('')
  const authType = ref('password') // password | lark | dingtalk | wecom | telegram
  const captchaDisabled = ref(false)
  const form = reactive({
    username: '',
    password: '',
    captcha_code: ''
  })

  onMounted(async () => {
    // SSO 回调：后端把 JWT 附在 SSO_OAUTH_SUCCESS_REDIRECT 的 query（access_token / refresh_token）
    const accessFromUrl = firstQuery(route.query.access_token)
    if (accessFromUrl) {
      const refreshFromUrl = firstQuery(route.query.refresh_token)
      LocalStorage.set('access', accessFromUrl)
      if (refreshFromUrl) {
        LocalStorage.set('refresh', refreshFromUrl)
      }
      try {
        const { data } = await api.get<{ user: UserResponse }>('/auth/me')
        setAdminCache(data.user.is_admin === true)
        $q.notify({ type: 'positive', message: t('loginSuccess') })
        const next = resolvePostLoginPath(route.query.next)
        await router.replace(next)
        return
      } catch {
        LocalStorage.remove('access')
        LocalStorage.remove('refresh')
        clearAdminCache()
        $q.notify({ type: 'negative', message: t('loginFailed') })
      }
    }

    try {
      const cfg = await fetchAuthConfig()
      authType.value = cfg.auth_type
      captchaDisabled.value = cfg.captcha_disabled
      if (!captchaDisabled.value) {
        loadCaptcha()
      }
    } catch {
      // ignore
    }
  })

  async function loadCaptcha () {
    captchaLoading.value = true
    captchaImage.value = ''
    try {
      const { data } = await api.get<{ token: string; image: string }>('/auth/captcha')
      captchaToken.value = data.token
      captchaImage.value = data.image
      form.captcha_code = ''
    } catch {
      captchaToken.value = ''
      $q.notify({ type: 'negative', message: t('captchaLoadFailed') })
    } finally {
      captchaLoading.value = false
    }
  }

  async function onSubmit () {
    loading.value = true
    try {
      const { data } = await api.post<TokenResponse>('/auth/login', {
        username: form.username,
        password: form.password,
        captcha_token: captchaToken.value,
        captcha_code: form.captcha_code.trim()
      })
      LocalStorage.set('access', data.access_token)
      LocalStorage.set('refresh', data.refresh_token)
      setAdminCache(data.user.is_admin === true)
      $q.notify({ type: 'positive', message: t('loginSuccess') })
      const next = resolvePostLoginPath(route.query.next)
      await router.replace(next)
    } catch (e: unknown) {
      const err = e as { response?: { data?: { error?: string } } }
      const msg = err.response?.data?.error ?? t('loginFailed')
      $q.notify({ type: 'negative', message: msg })
      void loadCaptcha()
    } finally {
      loading.value = false
    }
  }

  return {
    t,
    slogan,
    form,
    loading,
    captchaLoading,
    captchaImage,
    authType,
    captchaDisabled,
    onSubmit,
    loadCaptcha
  }
}
