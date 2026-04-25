<template>
  <q-layout>
    <q-page-container>
      <q-page class="auth-shell login-page row items-center justify-center">
        <div class="auth-blob auth-blob--a" aria-hidden="true" />
        <div class="auth-blob auth-blob--b" aria-hidden="true" />

        <q-card flat class="auth-card radius-lg q-pa-lg q-pt-xl q-pb-lg" style="width: 100%; max-width: 420px;">
          <q-card-section class="text-center q-pt-none q-pb-md">
            <div class="brand-mark row items-center justify-center q-mx-auto q-mb-md">
              <q-icon name="smart_toy" size="36px" color="white" />
            </div>
            <div class="text-h5 text-weight-bold text-text2">{{ t('appTitle') }}</div>
            <div class="text-body2 text-text3 q-mt-sm auth-sub">{{ slogan }}</div>
          </q-card-section>

          <!-- SSO：仅单点登录，不展示账号密码 -->
          <q-card-section v-if="authType !== 'password'" class="q-pb-md">
            <q-btn
              :label="ssoLoginLabel"
              color="primary"
              class="full-width"
              size="lg"
              unelevated
              @click="onSsoLogin"
            />
          </q-card-section>
          <q-card-section v-if="authType === 'password'" class="q-pt-none">
            <q-form @submit="onSubmit" class="q-gutter-md">
              <q-input
                v-model="form.username"
                :label="t('username')"
                outlined
                dense
                autocomplete="username"
                :rules="[val => !!val || t('username')]"
                lazy-rules
                class="auth-input"
              >
                <template #prepend>
                  <q-icon name="person" color="primary" />
                </template>
              </q-input>
              <q-input
                v-model="form.password"
                type="password"
                :label="t('password')"
                outlined
                dense
                autocomplete="current-password"
                :rules="[val => !!val || t('password')]"
                lazy-rules
                class="auth-input"
              >
                <template #prepend>
                  <q-icon name="lock" color="primary" />
                </template>
              </q-input>

              <div class="captcha-row" v-if="!captchaDisabled">
                <q-input
                  v-model="form.captcha_code"
                  class="captcha-input auth-input"
                  outlined
                  dense
                  maxlength="4"
                  inputmode="numeric"
                  pattern="[0-9]*"
                  autocomplete="one-time-code"
                  :label="t('captchaLabel')"
                  mask="####"
                  :rules="[val => (val && String(val).length === 4) || t('captchaRequired')]"
                  lazy-rules
                >
                  <template #prepend>
                    <q-icon name="shield" color="primary" />
                  </template>
                  <template #append>
                    <q-btn
                      flat
                      dense
                      round
                      icon="refresh"
                      color="primary"
                      :loading="captchaLoading"
                      :title="t('refreshCaptcha')"
                      @click.stop="loadCaptcha"
                    />
                  </template>
                </q-input>
                <div
                  class="captcha-box radius-sm cursor-pointer"
                  :title="t('refreshCaptcha')"
                  role="button"
                  tabindex="0"
                  @click="loadCaptcha"
                  @keyup.enter="loadCaptcha"
                >
                  <q-img
                    v-if="captchaImage && !captchaLoading"
                    :src="captchaImage"
                    fit="cover"
                    height="40px"
                    width="112px"
                    no-spinner
                    class="captcha-img radius-sm"
                  />
                  <div
                    v-else
                    class="captcha-placeholder row items-center justify-center radius-sm"
                    style="width: 112px; height: 40px;"
                  >
                    <q-spinner v-if="captchaLoading" color="primary" size="24px" />
                    <q-icon v-else name="image_not_supported" color="grey-5" size="24px" />
                  </div>
                </div>
              </div>

              <q-btn
                type="submit"
                class="login-btn full-width q-mt-sm"
                unelevated
                rounded
                :loading="loading"
                :label="t('login')"
              />
              <div class="text-center q-pt-sm">
                <router-link class="auth-link text-caption" to="/register">{{ t('register') }}</router-link>
              </div>
            </q-form>
          </q-card-section>
        </q-card>
      </q-page>
    </q-page-container>
  </q-layout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useLoginPage } from './loginPage'

defineOptions({
  name: 'LoginPage'
})

const { t, slogan, form, loading, captchaLoading, captchaImage, authType, captchaDisabled, onSubmit, loadCaptcha } = useLoginPage()

/** LARK / DINGTALK / WECOM / TELEGRAM + 固定后缀「 SSO 登录」 */
const ssoLoginLabel = computed(() => {
  const m: Record<string, string> = {
    lark: 'LARK',
    dingtalk: 'DINGTALK',
    wecom: 'WECOM',
    telegram: 'TELEGRAM'
  }
  const prefix = m[authType.value]
  return prefix ? `${prefix} SSO 登录` : 'SSO 登录'
})

function onSsoLogin () {
  const apiRoot = (process.env.API || '').replace(/\/$/, '')
  window.location.href = `${apiRoot}/api/v1/auth/sso/${authType.value}`
}
</script>

<style scoped>
.auth-shell {
  min-height: 100vh;
  position: relative;
  overflow: hidden;
  background: linear-gradient(145deg, #ecfeff 0%, #f0fdfa 32%, #f8fafc 68%, #e0f2fe 100%);
}

.auth-blob {
  position: absolute;
  border-radius: 50%;
  filter: blur(72px);
  pointer-events: none;
}

.auth-blob--a {
  width: min(420px, 90vw);
  height: min(420px, 90vw);
  background: rgba(45, 212, 191, 0.28);
  top: -120px;
  right: -100px;
}

.auth-blob--b {
  width: min(360px, 85vw);
  height: min(360px, 85vw);
  background: rgba(14, 165, 233, 0.22);
  bottom: -140px;
  left: -90px;
}

.auth-card {
  position: relative;
  z-index: 1;
  backdrop-filter: blur(14px);
  background: rgba(255, 255, 255, 0.9) !important;
  border: 1px solid rgba(255, 255, 255, 0.75) !important;
  box-shadow:
    0 25px 50px -12px rgba(15, 23, 42, 0.12),
    0 0 0 1px rgba(15, 23, 42, 0.04);
}

.radius-lg {
  border-radius: 16px;
}

.brand-mark {
  width: 72px;
  height: 72px;
  border-radius: 18px;
  background: linear-gradient(145deg, var(--q-primary), #0d9488);
  box-shadow: 0 14px 32px rgba(13, 148, 136, 0.35);
}

.auth-sub {
  line-height: 1.55;
  max-width: 300px;
  margin-left: auto;
  margin-right: auto;
}

.auth-link {
  color: var(--q-primary);
  text-decoration: none;
  font-weight: 500;
  border-bottom: 1px solid transparent;
  transition: border-color 0.2s;
}

.auth-link:hover {
  border-bottom-color: currentColor;
}

/* 输入框与右侧验证码图同一行；顶部对齐，避免校验错误撑高左侧列时把输入框相对图片「顶上去」 */
.captcha-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 112px;
  align-items: flex-start;
  column-gap: 10px;
}

.captcha-input {
  min-width: 0;
}

.captcha-box {
  border: 1px solid #e4e4e7;
  background: #fafafa;
  overflow: hidden;
  flex-shrink: 0;
  line-height: 0;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.captcha-box:hover {
  border-color: var(--q-primary);
  box-shadow: 0 0 0 1px rgba(13, 148, 136, 0.25);
}

.captcha-img {
  display: block;
}

.captcha-placeholder {
  background: #f4f4f5;
  border: 1px dashed #d4d4d8;
}
</style>
