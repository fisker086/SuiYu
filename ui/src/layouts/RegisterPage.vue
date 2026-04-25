<template>
  <q-layout>
    <q-page-container>
      <q-page class="auth-shell row items-center justify-center register-page">
        <div class="auth-blob auth-blob--a" aria-hidden="true" />
        <div class="auth-blob auth-blob--b" aria-hidden="true" />

        <q-card flat class="auth-card radius-lg q-pa-lg q-pt-xl q-pb-lg" style="width: 100%; max-width: 440px;">
          <q-card-section class="text-center q-pt-none q-pb-md">
            <div class="brand-mark brand-mark--alt row items-center justify-center q-mx-auto q-mb-md">
              <q-icon name="person_add" size="34px" color="white" />
            </div>
            <div class="text-h5 text-weight-bold text-text2">{{ t('register') }}</div>
            <div class="text-body2 text-text3 q-mt-sm auth-sub">{{ t('registerSubtitle') }}</div>
          </q-card-section>
          <q-card-section class="q-pt-none">
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
                v-model="form.email"
                type="email"
                :label="t('email')"
                outlined
                dense
                autocomplete="email"
                :rules="[val => !!val || t('email')]"
                lazy-rules
                class="auth-input"
              >
                <template #prepend>
                  <q-icon name="email" color="primary" />
                </template>
              </q-input>
              <q-input
                v-model="form.full_name"
                :label="t('fullName')"
                outlined
                dense
                autocomplete="name"
                class="auth-input"
              >
                <template #prepend>
                  <q-icon name="badge" color="primary" />
                </template>
              </q-input>
              <q-input
                v-model="form.password"
                type="password"
                :label="t('password')"
                outlined
                dense
                autocomplete="new-password"
                :rules="[val => (!!val && String(val).length >= 6) || t('passwordMin')]"
                lazy-rules
                class="auth-input"
              >
                <template #prepend>
                  <q-icon name="lock" color="primary" />
                </template>
              </q-input>

              <div class="captcha-row">
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
                :label="t('register')"
              />
              <div class="text-center q-pt-sm">
                <router-link class="auth-link text-caption" to="/login">{{ t('login') }}</router-link>
              </div>
            </q-form>
          </q-card-section>
        </q-card>
      </q-page>
    </q-page-container>
  </q-layout>
</template>

<script setup lang="ts">
import { useRegisterPage } from './registerPage'

defineOptions({
  name: 'RegisterPage'
})

const { t, form, loading, captchaLoading, captchaImage, onSubmit, loadCaptcha } = useRegisterPage()
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

.brand-mark--alt {
  background: linear-gradient(145deg, #0d9488, #0e7490);
}

.auth-sub {
  line-height: 1.55;
  max-width: 320px;
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
