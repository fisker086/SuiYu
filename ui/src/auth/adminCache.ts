import axios from 'axios'
import { LocalStorage } from 'quasar'
import { api } from 'boot/axios'
import type { UserResponse } from 'src/api/types'

const KEY = 'is_admin'

export function setAdminCache (isAdmin: boolean): void {
  LocalStorage.set(KEY, isAdmin ? '1' : '0')
}

export function clearAdminCache (): void {
  LocalStorage.remove(KEY)
}

/** 路由守卫用：始终请求 /auth/me，避免本地曾缓存 is_admin=0 而后端已升为管理员时仍进不去 */
export async function isAdminUser (): Promise<boolean> {
  if (!LocalStorage.has('access')) return false
  try {
    const { data } = await api.get<{ user: UserResponse }>('/auth/me')
    const admin = data.user.is_admin === true
    setAdminCache(admin)
    return admin
  } catch (e) {
    // 与 layout 里并发的 GET /auth/me 会触发 axios 去重取消，不应当作未登录/非管理员
    if (axios.isCancel(e)) {
      return LocalStorage.getItem(KEY) === '1'
    }
    return false
  }
}
