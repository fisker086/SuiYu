import { reactive, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { useRouter } from 'vue-router'
import { api } from 'boot/axios'

export function useRegisterPage () {
  const { t } = useI18n()
  const $q = useQuasar()
  const router = useRouter()
  const loading = ref(false)
  const captchaLoading = ref(false)
  const captchaToken = ref('')
  const captchaImage = ref('')
  const form = reactive({
    username: '',
    email: '',
    full_name: '',
    password: '',
    captcha_code: ''
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

  onMounted(() => {
    void loadCaptcha()
  })

  async function onSubmit () {
    loading.value = true
    try {
      await api.post('/auth/register', {
        username: form.username,
        email: form.email,
        password: form.password,
        full_name: form.full_name,
        captcha_token: captchaToken.value,
        captcha_code: form.captcha_code.trim()
      })
      $q.notify({ type: 'positive', message: t('registerSuccess') })
      await router.replace('/login')
    } catch (e: unknown) {
      const err = e as { response?: { data?: { error?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.error ?? t('registerFailed') })
      void loadCaptcha()
    } finally {
      loading.value = false
    }
  }

  return {
    t,
    form,
    loading,
    captchaLoading,
    captchaImage,
    onSubmit,
    loadCaptcha
  }
}
