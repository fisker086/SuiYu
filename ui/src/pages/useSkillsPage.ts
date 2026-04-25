import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { api } from 'boot/axios'
import type { Skill, APIResponse, CreateSkillRequest } from 'src/api/types'
import { renderChatMarkdown } from 'src/utils/chatMarkdown'

const defaultForm = {
  key: '',
  name: '',
  description: '',
  content: '',
  source_ref: '',
  is_active: true
}

export function useSkillsPage () {
  const { t } = useI18n()
  const $q = useQuasar()
  const loading = ref(false)
  const saving = ref(false)
  const rows = ref<Skill[]>([])
  const skillRiskOptions = computed(() => [
    { label: t('riskLow'), value: 'low' },
    { label: t('riskMedium'), value: 'medium' },
    { label: t('riskHigh'), value: 'high' },
    { label: t('riskCritical'), value: 'critical' }
  ])

  const skillExecutionModeOptions = computed(() => [
    { label: t('execServer'), value: 'server' },
    { label: t('execClient'), value: 'client' }
  ])

  function riskLevelLabel (raw: string | undefined): string {
    const r = (raw || 'medium').toLowerCase()
    const map: Record<string, string> = {
      low: t('riskLow'),
      medium: t('riskMedium'),
      high: t('riskHigh'),
      critical: t('riskCritical')
    }
    return map[r] || r
  }

  const columns = computed(() => [
    { name: 'id', label: 'ID', field: 'id', align: 'left' as const },
    {
      name: 'key',
      label: t('key'),
      field: 'key',
      align: 'left' as const,
      style: 'width: 220px; min-width: 220px; max-width: 220px',
      classes: 'skill-table-col-key'
    },
    {
      name: 'kind',
      label: t('skillKindCol'),
      field: (row: Skill) => (row.key?.startsWith('builtin_skill.') ? 'builtin' : 'custom'),
      align: 'center' as const
    },
    {
      name: 'name',
      label: t('name'),
      field: 'name',
      align: 'left' as const,
      style: 'max-width: 140px',
      classes: 'skill-table-col-name'
    },
    {
      name: 'risk_level',
      label: t('riskLevel'),
      field: 'risk_level',
      align: 'center' as const,
      style: 'min-width: 120px; width: 120px; max-width: 120px',
      classes: 'skill-table-col-risk'
    },
    { name: 'execution_mode', label: t('executionLocation'), field: 'execution_mode', align: 'center' as const },
    { name: 'description', label: t('description'), field: 'description', align: 'left' as const },
    {
      name: 'is_active',
      label: t('status'),
      field: (row: Skill) => (row.is_active ? t('roleEnabled') : t('roleDisabled')),
      align: 'center' as const,
      style: 'width: 80px; min-width: 80px; max-width: 80px',
      classes: 'skill-table-col-status'
    },
    {
      name: 'actions',
      label: t('actions'),
      field: 'actions',
      align: 'center' as const,
      style: 'min-width: 200px; width: 200px; max-width: 200px',
      classes: 'skill-table-col-actions'
    }
  ])
  const dialogOpen = ref(false)
  const editorOpen = ref(false)
  const editingId = ref<number | null>(null)
  const form = reactive({ ...defaultForm })
  const editingSkill = ref<Skill | null>(null)
  const syncLoading = ref(false)
  const editorSplit = ref(52)
  const previewHtml = computed(() => renderChatMarkdown(editingSkill.value?.content ?? ''))
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const search = ref('')

  /** 客户端分页；Quasar 默认 rowsPerPage 为 5，显式设为 10 */
  const pagination = ref({
    sortBy: 'id' as string,
    descending: false,
    page: 1,
    rowsPerPage: 10
  })

  async function load () {
    loading.value = true
    try {
      const { data } = await api.get<APIResponse<Skill[]>>('/skills')
      rows.value = (data.data ?? []) as Skill[]
    } catch {
      rows.value = []
    } finally {
      loading.value = false
    }
  }

  function openCreateDialog () {
    editingId.value = null
    Object.assign(form, defaultForm)
    dialogOpen.value = true
  }

  async function saveSkill () {
    const key = (form.key || '').trim()
    const name = (form.name || '').trim()
    if (!key || !name) return
    const sourceRef = (form.source_ref || '').trim() || key.replace(/^builtin_skill\./, '')
    if (!sourceRef) return
    saving.value = true
    try {
      const body: CreateSkillRequest = {
        key,
        name,
        description: form.description,
        content: form.content,
        source_ref: sourceRef
      }

      if (editingId.value) {
        await api.put(`/skills/${editingId.value}`, body)
        $q.notify({ type: 'positive', message: t('updateOk') })
      } else {
        await api.post('/skills', body)
        $q.notify({ type: 'positive', message: t('createOk') })
      }

      dialogOpen.value = false
      await load()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('operationFailed') })
    } finally {
      saving.value = false
    }
  }

  async function openEditor (skill: Skill) {
    try {
      const { data } = await api.get<APIResponse<Skill>>(`/skills/${skill.id}`)
      const full = data.data
      if (full) {
        editingSkill.value = {
          ...full,
          content: full.content ?? '',
          risk_level: full.risk_level || 'medium',
          execution_mode: full.execution_mode || 'server'
        }
        editorOpen.value = true
        return
      }
    } catch {
      /* use list row */
    }
    editingSkill.value = {
      ...skill,
      content: skill.content ?? '',
      risk_level: skill.risk_level || 'medium',
      execution_mode: skill.execution_mode || 'server'
    }
    if (!(editingSkill.value.content ?? '').trim() && skill.source_ref) {
      $q.notify({ type: 'warning', message: t('skillEditorContentEmptyHint') })
    }
    editorOpen.value = true
  }

  async function saveSkillContent () {
    if (!editingSkill.value) return
    saving.value = true
    try {
      await api.put(`/skills/${editingSkill.value.id}`, {
        content: editingSkill.value.content ?? '',
        risk_level: editingSkill.value.risk_level,
        execution_mode: editingSkill.value.execution_mode
      })
      $q.notify({ type: 'positive', message: t('saveSuccess') })
      editorOpen.value = false
      await load()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('saveFailed') })
    } finally {
      saving.value = false
    }
  }

  async function toggleActive (skill: Skill, val: boolean) {
    try {
      await api.put(`/skills/${skill.id}`, { is_active: val })
      $q.notify({ type: 'positive', message: val ? t('toggleEnabledOn') : t('toggleDisabledOff') })
      await load()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('updateFailed') })
      await load()
    }
  }

  function confirmDelete (skill: Skill) {
    $q.dialog({
      title: t('confirmDelete'),
      message: `${skill.name} (ID: ${skill.id})`,
      cancel: { label: t('cancel'), flat: true },
      ok: { label: t('delete'), color: 'negative' }
    }).onOk(async () => {
      try {
        await api.delete(`/skills/${skill.id}`)
        $q.notify({ type: 'positive', message: t('deleteOk') })
        await load()
      } catch (e: unknown) {
        const err = e as { response?: { data?: { message?: string } } }
        $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('deleteFailed') })
      }
    })
  }

  async function syncBuiltinSkills () {
    syncLoading.value = true
    try {
      const { data } = await api.post<APIResponse<{ created?: number }>>('/skills/sync-builtins')
      const n = data.data?.created ?? 0
      $q.notify({
        type: 'positive',
        message: n > 0 ? t('skillSyncBuiltinsOk', { n }) : t('skillSyncBuiltinsNone')
      })
      await load()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('skillSyncBuiltinsFail') })
    } finally {
      syncLoading.value = false
    }
  }

  onMounted(() => { void load() })

  return {
    t,
    loading,
    saving,
    rows,
    pagination,
    columns,
    search,
    load,
    syncBuiltinSkills,
    syncLoading,
    editorSplit,
    previewHtml,
    dialogOpen,
    form,
    editingId,
    openCreateDialog,
    saveSkill,
    editorOpen,
    editingSkill,
    openEditor,
    saveSkillContent,
    toggleActive,
    confirmDelete,
    riskLevelLabel,
    skillRiskOptions,
    skillExecutionModeOptions
  }
}
