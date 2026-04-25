/// <reference types="@quasar/app-vite" />

import 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    requireAuth?: boolean
    requiresAdmin?: boolean
  }
}

declare namespace NodeJS {
  interface ProcessEnv {
    API: string
  }
}
