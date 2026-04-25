<template>
  <q-page padding>
    <div class="text-h6 text-text2 q-mb-md">{{ t('profile') }}</div>
    <q-inner-loading :showing="loading" />
    <q-card v-if="!loading" flat bordered class="radius-sm q-mb-md" style="max-width: 520px;">
      <q-card-section class="text-subtitle2">{{ t('profile') }}</q-card-section>
      <q-card-section>
        <q-input v-model="profileForm.email" :label="t('email')" type="email" outlined dense class="q-mb-sm" />
        <q-input v-model="profileForm.full_name" :label="t('fullName')" outlined dense class="q-mb-sm" />
        <q-btn color="primary" :label="t('save')" :loading="saving" @click="saveProfile" unelevated />
      </q-card-section>
    </q-card>
    <q-card v-if="!loading && showChangePassword" flat bordered class="radius-sm" style="max-width: 520px;">
      <q-card-section class="text-subtitle2">{{ t('changePassword') }}</q-card-section>
      <q-card-section>
        <q-input v-model="pwdForm.current_password" :label="t('currentPassword')" type="password" outlined dense class="q-mb-sm" />
        <q-input v-model="pwdForm.new_password" :label="t('newPassword')" type="password" outlined dense class="q-mb-sm" />
        <q-btn color="secondary" :label="t('changePassword')" :loading="pwdSaving" @click="changePassword" outline />
      </q-card-section>
    </q-card>
  </q-page>
</template>

<script setup lang="ts">
import { useProfilePage } from './useProfilePage'

defineOptions({
  name: 'ProfilePage'
})

const { t, loading, saving, pwdSaving, showChangePassword, profileForm, pwdForm, saveProfile, changePassword } = useProfilePage()
</script>
