import { boot } from 'quasar/wrappers'
import axios, { AxiosInstance, AxiosRequestConfig, AxiosResponse, AxiosError, Canceler, InternalAxiosRequestConfig } from 'axios'
import { LocalStorage } from 'quasar'
import qs from 'qs'

declare module '@vue/runtime-core' {
  interface ComponentCustomProperties {
    $axios: AxiosInstance
    $api: AxiosInstance
  }
}

const apiRoot = (process.env.API || '').replace(/\/$/, '')
const baseURL = apiRoot ? `${apiRoot}/api/v1` : '/api/v1'
const timeOut = 120000
const pending: Map<string, Canceler> = new Map()

/** 会话历史 GET：同 URL 并发时勿取消上一次（否则 DevTools 显示 canceled，loadMessages 丢结果） */
function isChatSessionMessagesGet (config: AxiosRequestConfig): boolean {
  const m = (config.method || 'get').toLowerCase()
  const u = config.url || ''
  return m === 'get' && /\/chat\/sessions\/[^/]+\/messages(?:\?|$)/.test(u)
}

/** 会话列表 GET /chat/sessions?agent_id=...：watch(agentId)/loadSessions 易并发，勿取消上一次 */
function isChatSessionsListGet (config: AxiosRequestConfig): boolean {
  const m = (config.method || 'get').toLowerCase()
  const u = config.url || ''
  if (m !== 'get') return false
  if (u.startsWith('/chat/sessions?')) return true
  return u === '/chat/sessions'
}

/** GET /auth/me：布局 loadMe 与路由守卫 isAdminUser 几乎同时发起，取消上一次会导致先 await 的那路拿不到 body（表现为刷新后偶发无用户信息） */
function isAuthMeGet (config: AxiosRequestConfig): boolean {
  const m = (config.method || 'get').toLowerCase()
  const u = config.url || ''
  return m === 'get' && u === '/auth/me'
}

const addPending = (config: AxiosRequestConfig) => {
  const url = [
    config.method,
    config.url,
    qs.stringify(config.params),
    qs.stringify(config.data)
  ].join('&')
  config.cancelToken = config.cancelToken || new axios.CancelToken(cancel => {
    if (!pending.has(url)) {
      pending.set(url, cancel)
    }
  })
}

const removePending = (config: AxiosRequestConfig) => {
  const url = [
    config.method,
    config.url,
    qs.stringify(config.params),
    qs.stringify(config.data)
  ].join('&')
  if (pending.has(url)) {
    const cancel = pending.get(url)
    cancel && cancel(url)
    pending.delete(url)
  }
}

export const clearPending = () => {
  for (const [url, cancel] of pending) {
    cancel(url)
  }
  pending.clear()
}

const getLoginRedirectUrl = () => {
  const nextPath = `${window.location.pathname}${window.location.search}${window.location.hash}`
  return `/login?next=${encodeURIComponent(nextPath)}`
}

const jsonFunction = (config: InternalAxiosRequestConfig): InternalAxiosRequestConfig => {
  if (!isChatSessionMessagesGet(config) && !isChatSessionsListGet(config) && !isAuthMeGet(config)) {
    removePending(config)
    addPending(config)
  }
  const token = LocalStorage.getItem('access') as string | null
  if (config.data != null && typeof config.data === 'object' && !(config.data instanceof FormData)) {
    config.data = JSON.stringify(config.data) as unknown as InternalAxiosRequestConfig['data']
  }
  if (!config.headers['Content-Type']) {
    config.headers['Content-Type'] = 'application/json'
  }
  if (token && token.length > 2) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
}

const finishFunction = (response: AxiosResponse): AxiosResponse => {
  removePending(response.config as AxiosRequestConfig)
  return response
}

const errorFunction = (error: AxiosError): Promise<AxiosError> => {
  const res = error.response
  if (res !== undefined) {
    switch (res.status) {
      case 401:
        LocalStorage.remove('access')
        LocalStorage.remove('refresh')
        window.location.href = getLoginRedirectUrl()
        break
      case 502:
        window.location.href = getLoginRedirectUrl()
        break
      default:
        break
    }
  }
  return Promise.reject(error)
}

const api = axios.create({ baseURL, timeout: timeOut })

api.interceptors.request.use(jsonFunction)
api.interceptors.response.use(finishFunction, errorFunction)

interface AuthConfig {
  auth_type: string
  captcha_disabled: boolean
}

let cachedAuthConfig: AuthConfig | null = null

export async function fetchAuthConfig (): Promise<AuthConfig> {
  if (cachedAuthConfig) return cachedAuthConfig
  try {
    const resp = await api.get<AuthConfig & { data?: AuthConfig }>('/auth/config')
    const body = resp.data
    const data = body.data ?? body
    cachedAuthConfig = data
    return data
  } catch {
    return { auth_type: 'password', captcha_disabled: false }
  }
}

export default boot(({ app }) => {
  app.config.globalProperties.$axios = axios
  app.config.globalProperties.$api = api
})

export { api }
