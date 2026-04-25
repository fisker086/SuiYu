import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { api, fetchAuthConfig } from 'boot/axios'
import type { UserResponse } from 'src/api/types'

export function useProfilePage () {
  const { t } = useI18n()
  const $q = useQuasar()
  const loading = ref(false)
  const saving = ref(false)
  const pwdSaving = ref(false)
  const profileForm = reactive({
    email: '',
    full_name: ''
  })

  const pwdForm = reactive({
    current_password: '',
    new_password: ''
  })

  /** 与全局 AUTH_TYPE 一致：仅密码登录时可改本地密码；SSO 不展示 */
  const showChangePassword = ref(true)

  async function load () {
    loading.value = true
    try {
      const cfg = await fetchAuthConfig()
      showChangePassword.value = cfg.auth_type === 'password'
      const { data } = await api.get<{ user: UserResponse }>('/auth/me')
      profileForm.email = data.user.email
      profileForm.full_name = data.user.full_name ?? ''
    } catch {
    } finally {
      loading.value = false
    }
  }

  async function saveProfile () {
    saving.value = true
    try {
      await api.put('/auth/me', {
        email: profileForm.email,
        full_name: profileForm.full_name
      })
      $q.notify({ type: 'positive', message: t('profileUpdated') })
      await load()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { error?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.error ?? '保存失败' })
    } finally {
      saving.value = false
    }
  }

  async function changePassword () {
    if (!pwdForm.current_password || !pwdForm.new_password || pwdForm.new_password.length < 6) {
      $q.notify({ type: 'warning', message: t('confirm') })
      return
    }
    pwdSaving.value = true
    try {
      await api.post('/auth/change-password', {
        current_password: pwdForm.current_password,
        new_password: pwdForm.new_password
      })
      $q.notify({ type: 'positive', message: t('passwordChanged') })
      pwdForm.current_password = ''
      pwdForm.new_password = ''
    } catch (e: unknown) {
      const err = e as { response?: { data?: { error?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.error ?? '修改失败' })
    } finally {
      pwdSaving.value = false
    }
  }

  onMounted(() => {
    void load()
  })

  return {
    t,
    loading,
    saving,
    pwdSaving,
    showChangePassword,
    profileForm,
    pwdForm,
    load,
    saveProfile,
    changePassword
  }
}
