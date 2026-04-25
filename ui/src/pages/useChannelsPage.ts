import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { api } from 'boot/axios'
import type { APIResponse, NotifyChannel } from 'src/api/types'

const defaultForm = {
  name: '',
  kind: 'lark',
  webhook_url: '',
  app_id: '',
  app_secret: '',
  extra_json: '{}',
  is_active: true,
  test_message: 'channel test'
}

export function useChannelsPage () {
  const { t } = useI18n()
  const $q = useQuasar()
  const columns = computed(() => [
    { name: 'id', label: 'ID', field: 'id', align: 'left' as const },
    { name: 'name', label: t('name'), field: 'name', align: 'left' as const },
    { name: 'kind', label: t('channelKind'), field: 'kind', align: 'left' as const },
    { name: 'webhook_url', label: 'Webhook', field: 'webhook_url', align: 'left' as const },
    { name: 'has_app_secret', label: t('channelColSecret'), field: 'has_app_secret', align: 'center' as const },
    { name: 'is_active', label: t('isActive'), field: 'is_active', align: 'center' as const },
    { name: 'actions', label: t('actions'), field: 'actions', align: 'right' as const }
  ])
  const loading = ref(false)
  const saving = ref(false)
  const testing = ref(false)
  const rows = ref<NotifyChannel[]>([])
  const errorMsg = ref('')
  const dialogOpen = ref(false)
  const editingId = ref<number | null>(null)
  const form = reactive({ ...defaultForm })

  async function load () {
    loading.value = true
    errorMsg.value = ''
    try {
      const { data } = await api.get<APIResponse<NotifyChannel[]>>('/channels')
      rows.value = (data.data ?? []) as NotifyChannel[]
    } catch (e: unknown) {
      rows.value = []
      const err = e as { response?: { data?: { message?: string } } }
      errorMsg.value = err.response?.data?.message ?? t('loadFailed')
    } finally {
      loading.value = false
    }
  }

  function openDialog (row?: NotifyChannel) {
    if (row) {
      editingId.value = row.id
      form.name = row.name
      form.kind = row.kind
      form.webhook_url = row.webhook_url || ''
      form.app_id = row.app_id || ''
      form.app_secret = ''
      form.extra_json = JSON.stringify(row.extra ?? {}, null, 2)
      form.is_active = row.is_active
      form.test_message = 'channel test'
    } else {
      editingId.value = null
      Object.assign(form, { ...defaultForm })
    }
    dialogOpen.value = true
  }

  async function saveChannel () {
    if (!form.name?.trim()) {
      $q.notify({ type: 'negative', message: t('channelNameRequired') })
      return
    }
    let extra: Record<string, string> = {}
    try {
      const parsed = JSON.parse(form.extra_json || '{}') as Record<string, unknown>
      extra = {}
      for (const [k, v] of Object.entries(parsed)) {
        extra[k] = typeof v === 'string' ? v : String(v)
      }
    } catch {
      $q.notify({ type: 'negative', message: t('channelExtraJsonInvalid') })
      return
    }

    saving.value = true
    try {
      if (editingId.value) {
        const body: Record<string, unknown> = {
          name: form.name.trim(),
          webhook_url: form.webhook_url.trim(),
          app_id: form.app_id.trim(),
          extra,
          is_active: form.is_active
        }
        if (form.app_secret.trim()) {
          body.app_secret = form.app_secret.trim()
        }
        await api.put(`/channels/${editingId.value}`, body)
        $q.notify({ type: 'positive', message: t('saveOk') })
      } else {
        await api.post('/channels', {
          name: form.name.trim(),
          kind: form.kind,
          webhook_url: form.webhook_url.trim(),
          app_id: form.app_id.trim(),
          app_secret: form.app_secret.trim(),
          extra,
          is_active: form.is_active
        })
        $q.notify({ type: 'positive', message: t('createOk') })
      }
      dialogOpen.value = false
      await load()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('saveFailed') })
    } finally {
      saving.value = false
    }
  }

  async function testSend () {
    if (!editingId.value) return
    testing.value = true
    try {
      await api.post(`/channels/${editingId.value}/test`, {
        message: form.test_message || 'channel test'
      })
      $q.notify({ type: 'positive', message: t('channelTestSent') })
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('channelTestFailed') })
    } finally {
      testing.value = false
    }
  }

  function confirmDelete (row: NotifyChannel) {
    $q.dialog({
      title: t('confirmDelete'),
      message: `${row.name} (ID: ${row.id})`,
      cancel: { label: t('cancel'), flat: true },
      ok: { label: t('delete'), color: 'negative' }
    }).onOk(async () => {
      try {
        await api.delete(`/channels/${row.id}`)
        $q.notify({ type: 'positive', message: t('deleteOk') })
        await load()
      } catch (e: unknown) {
        const err = e as { response?: { data?: { message?: string } } }
        $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('deleteFailed') })
      }
    })
  }

  onMounted(() => { void load() })

  return {
    t,
    loading,
    saving,
    testing,
    rows,
    columns,
    errorMsg,
    dialogOpen,
    form,
    editingId,
    openDialog,
    saveChannel,
    testSend,
    confirmDelete,
    load
  }
}
