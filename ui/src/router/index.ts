import { route } from 'quasar/wrappers'
import {
  createMemoryHistory,
  createRouter,
  createWebHashHistory,
  createWebHistory
} from 'vue-router'

import routes from './routes'
import { LocalStorage } from 'quasar'
import { isAdminUser } from 'src/auth/adminCache'

export default route(function () {
  const createHistory = process.env.SERVER
    ? createMemoryHistory
    : (process.env.VUE_ROUTER_MODE === 'history' ? createWebHistory : createWebHashHistory)

  const Router = createRouter({
    scrollBehavior: () => ({ left: 0, top: 0 }),
    routes,
    history: createHistory(process.env.VUE_ROUTER_BASE)
  })

  Router.beforeEach(async (to, _from, next) => {
    // 不在此处 clearPending：全量取消会打断 Chat 内 router.replace 时仍在飞行的 loadMessages/loadSessions
    // 同 URL 重复请求仍由 boot/axios 拦截器按 key 取消
    if (process.env.SERVER) {
      next()
      return
    }
    if (to.meta.requireAuth && !LocalStorage.has('access')) {
      next({ path: '/login', query: { next: to.fullPath } })
      return
    }
    if (to.meta.requiresAdmin) {
      if (!LocalStorage.has('access')) {
        next({ path: '/login', query: { next: to.fullPath } })
        return
      }
      const ok = await isAdminUser()
      if (!ok) {
        next({ name: 'dashboard' })
        return
      }
    }
    next()
  })

  return Router
})
