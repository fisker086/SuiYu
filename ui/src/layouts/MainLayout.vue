<template>
  <q-layout view="hHh Lpr lFf">
    <q-header elevated class="bg-primary text-white">
      <q-toolbar>
        <q-btn flat dense round icon="menu" aria-label="Menu" @click="leftDrawer = !leftDrawer" />
        <q-toolbar-title>
          {{ t('appTitle') }}
        </q-toolbar-title>
        <q-space />
        <q-btn-dropdown
          flat
          dense
          round
          text-color="white"
          icon="language"
          :aria-label="t('language')"
        >
          <q-list dense style="min-width: 180px">
            <q-item
              clickable
              v-close-popup
              :active="locale === 'en-US'"
              active-class="text-primary"
              @click="setLocale('en-US')"
            >
              <q-item-section>{{ t('langLocaleEn') }}</q-item-section>
              <q-item-section v-if="locale === 'en-US'" side>
                <q-icon name="check" size="xs" color="primary" />
              </q-item-section>
            </q-item>
            <q-item
              clickable
              v-close-popup
              :active="locale === 'zh-CN'"
              active-class="text-primary"
              @click="setLocale('zh-CN')"
            >
              <q-item-section>{{ t('langLocaleZh') }}</q-item-section>
              <q-item-section v-if="locale === 'zh-CN'" side>
                <q-icon name="check" size="xs" color="primary" />
              </q-item-section>
            </q-item>
          </q-list>
        </q-btn-dropdown>
      </q-toolbar>
    </q-header>

    <q-drawer v-model="leftDrawer" show-if-above bordered class="bg-grey-1 main-layout-drawer" :width="240">
      <div class="column no-wrap fit">
        <q-scroll-area class="main-layout-drawer-scroll col">
          <q-list padding>
            <q-item clickable v-ripple :to="{ name: 'dashboard' }" exact>
              <q-item-section avatar>
                <q-icon name="dashboard" />
              </q-item-section>
              <q-item-section>{{ t('dashboard') }}</q-item-section>
            </q-item>
            <q-item clickable v-ripple :to="{ name: 'chat' }">
              <q-item-section avatar>
                <q-icon name="chat" />
              </q-item-section>
              <q-item-section>{{ t('chat') }}</q-item-section>
            </q-item>
            <q-item clickable v-ripple :to="{ name: 'channels' }">
              <q-item-section avatar>
                <q-icon name="notifications" />
              </q-item-section>
              <q-item-section>{{ t('channels') }}</q-item-section>
            </q-item>
            <q-item clickable v-ripple :to="{ name: 'approvals' }">
              <q-item-section avatar>
                <q-icon name="approval" />
              </q-item-section>
              <q-item-section>{{ t('approvals') }}</q-item-section>
            </q-item>
            <q-item clickable v-ripple :to="{ name: 'schedules' }">
              <q-item-section avatar>
                <q-icon name="schedule" />
              </q-item-section>
              <q-item-section>{{ t('schedules') }}</q-item-section>
            </q-item>

            <template v-if="meLoaded && isAdmin">
              <q-separator class="q-my-sm" />
              <q-item clickable v-ripple :to="{ name: 'agents' }">
                <q-item-section avatar>
                  <q-icon name="smart_toy" />
                </q-item-section>
                <q-item-section>{{ t('agents') }}</q-item-section>
              </q-item>
              <q-item clickable v-ripple :to="{ name: 'skills' }">
                <q-item-section avatar>
                  <q-icon name="build" />
                </q-item-section>
                <q-item-section>{{ t('skills') }}</q-item-section>
              </q-item>
              <q-item clickable v-ripple :to="{ name: 'mcp' }">
                <q-item-section avatar>
                  <q-icon name="hub" />
                </q-item-section>
                <q-item-section>{{ t('mcp') }}</q-item-section>
              </q-item>
              <q-item clickable v-ripple :to="{ name: 'bots' }">
                <q-item-section avatar>
                  <q-icon name="hub" />
                </q-item-section>
                <q-item-section>{{ t('botTitle') }}</q-item-section>
              </q-item>
              <q-item clickable v-ripple :to="{ name: 'workflows' }">
                <q-item-section avatar>
                  <q-icon name="account_tree" />
                </q-item-section>
                <q-item-section>{{ t('workflows') }}</q-item-section>
              </q-item>
              <q-item clickable v-ripple :to="{ name: 'audit-logs' }">
                <q-item-section avatar>
                  <q-icon name="receipt_long" />
                </q-item-section>
                <q-item-section>{{ t('auditLogs') }}</q-item-section>
              </q-item>
              <q-item clickable v-ripple :to="{ name: 'roles' }">
                <q-item-section avatar>
                  <q-icon name="supervisor_account" />
                </q-item-section>
                <q-item-section>{{ t('roles') }}</q-item-section>
              </q-item>
              <q-item clickable v-ripple :to="{ name: 'users' }">
                <q-item-section avatar>
                  <q-icon name="people" />
                </q-item-section>
                <q-item-section>{{ t('users') }}</q-item-section>
              </q-item>
              <q-item clickable v-ripple :to="{ name: 'usage' }">
                <q-item-section avatar>
                  <q-icon name="insights" />
                </q-item-section>
                <q-item-section>{{ t('usageStats') }}</q-item-section>
              </q-item>
            </template>
          </q-list>
        </q-scroll-area>

        <q-separator />
        <div class="main-layout-drawer-user q-pa-md">
          <div class="row items-center no-wrap q-gutter-sm">
            <q-avatar color="primary" text-color="white" size="40px" class="cursor-pointer" @click="goProfile">
              <img v-if="avatarUrl" :src="avatarUrl" alt="">
              <q-icon v-else name="person" />
            </q-avatar>
            <div class="col main-layout-drawer-user-text cursor-pointer" @click="goProfile">
              <div class="text-body2 text-weight-medium ellipsis">{{ userLabel || '—' }}</div>
              <div class="text-caption text-grey-7">{{ t('profile') }}</div>
            </div>
            <q-btn flat dense round icon="logout" :aria-label="t('logout')" @click="onLogout" />
          </div>
        </div>
      </div>
    </q-drawer>

    <q-page-container class="main-layout-page-container" style="background-color: #f8f7f4;">
      <router-view />
    </q-page-container>
  </q-layout>
</template>

<script setup lang="ts">
import { useMainLayout } from './mainLayout'

defineOptions({
  name: 'MainLayout'
})

const { t, locale, setLocale, leftDrawer, userLabel, avatarUrl, meLoaded, isAdmin, onLogout, goProfile } = useMainLayout()
</script>

<style scoped>
.main-layout-drawer-scroll {
  flex: 1 1 0;
  min-height: 0;
}
.main-layout-drawer-user-text {
  min-width: 0;
}
</style>
