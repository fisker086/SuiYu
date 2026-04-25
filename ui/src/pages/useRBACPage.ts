import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { api } from 'boot/axios'
import type { Role, UserResponse, Agent } from 'src/api/types'
import { fetchAgentList } from 'src/utils/fetchAgentList'

export const roleTableColumns = [
  { name: 'name', label: '角色名称', field: 'name', align: 'left' as const },
  { name: 'description', label: '描述', field: 'description', align: 'left' as const },
  { name: 'user_count', label: '用户数', field: 'user_count', align: 'center' as const },
  { name: 'agent_count', label: '智能体数', field: 'agent_count', align: 'center' as const },
  { name: 'is_active', label: '状态', field: 'is_active', align: 'center' as const },
  { name: 'actions', label: '操作', field: 'actions', align: 'center' as const }
]

export const userTableColumns = [
  { name: 'username', label: '用户名', field: 'username', align: 'left' as const },
  { name: 'email', label: '邮箱', field: 'email', align: 'left' as const },
  { name: 'roles', label: '角色', field: 'user_roles', align: 'left' as const },
  { name: 'actions', label: '操作', field: 'actions', align: 'center' as const }
]

export function useRBACPage () {
  const { t } = useI18n()
  const $q = useQuasar()

  const loading = ref(false)
  const roles = ref<Role[]>([])
  const users = ref<(UserResponse & { user_roles?: any[] })[]>([])
  const allAgents = ref<Agent[]>([])

  const roleDialogOpen = ref(false)
  const rolePermDialogOpen = ref(false)
  const userRoleDialogOpen = ref(false)
  const isEditRole = ref(false)
  const selectedRole = ref<Role | null>(null)
  const selectedUser = ref<UserResponse | null>(null)
  const selectedUserCurrentRoles = ref<any[]>([])
  const agentSearch = ref('')
  const roleForm = ref<{ name: string; description: string; is_active: boolean }>({ name: '', description: '', is_active: true })

  const roleColumns = roleTableColumns
  const userColumns = userTableColumns
  const roleOptions = computed(() => roles.value.map(r => ({ label: r.name, value: r.id })))

  const agentPerms = ref<Record<number, Record<number, boolean>>>({})

  const loadAllAgents = async () => {
    allAgents.value = await fetchAgentList()
  }

  const filteredAllAgents = computed(() => {
    const search = agentSearch.value.toLowerCase().trim()
    if (!search) return allAgents.value
    return allAgents.value.filter(a =>
      a.name.toLowerCase().includes(search) ||
      (a.description && a.description.toLowerCase().includes(search)) ||
      String(a.id).includes(search)
    )
  })

  const selectedAgentCount = computed(() => {
    if (!selectedRole.value) return 0
    const rolePerms = agentPerms.value[selectedRole.value.id] || {}
    return Object.keys(rolePerms).filter(id => rolePerms[Number(id)]).length
  })

  const loadRoleAgentPerms = async (roleId: number) => {
    try {
      const { data } = await api.get<{ data: Record<string, boolean> }>(`/rbac/roles/${roleId}/agent-permissions`)
      const perms: Record<number, boolean> = {}
      for (const [agentId, hasAccess] of Object.entries(data.data || {})) {
        if (hasAccess) {
          perms[Number(agentId)] = true
        }
      }
      agentPerms.value = { ...agentPerms.value, [roleId]: perms }
    } catch {
      agentPerms.value[roleId] = {}
    }
  }

  const isAgentPermitted = (agentId: number): boolean => {
    if (!selectedRole.value) return false
    const rolePerms = agentPerms.value[selectedRole.value.id] || {}
    return !!rolePerms[agentId]
  }

  const setAgentAccess = (agentId: number, permitted: boolean) => {
    if (!selectedRole.value) return
    const roleId = selectedRole.value.id
    if (!agentPerms.value[roleId]) {
      agentPerms.value[roleId] = {}
    }
    if (permitted) {
      agentPerms.value[roleId][agentId] = true
    } else {
      delete agentPerms.value[roleId][agentId]
    }
    agentPerms.value = { ...agentPerms.value }
  }

  const selectAllAgents = () => {
    if (!selectedRole.value) return
    const roleId = selectedRole.value.id
    if (!agentPerms.value[roleId]) {
      agentPerms.value[roleId] = {}
    }
    for (const agent of allAgents.value) {
      agentPerms.value[roleId][agent.id] = true
    }
    agentPerms.value = { ...agentPerms.value }
  }

  const deselectAllAgents = () => {
    if (!selectedRole.value) return
    const roleId = selectedRole.value.id
    agentPerms.value[roleId] = {}
    agentPerms.value = { ...agentPerms.value }
  }

  const saveAgentPermissions = async (): Promise<boolean> => {
    if (!selectedRole.value) return false
    const roleId = selectedRole.value.id
    const agentIDs = Object.entries(agentPerms.value[roleId] || {})
      .filter(([, hasAccess]) => hasAccess)
      .map(([id]) => Number(id))
    try {
      await api.post(`/rbac/roles/${roleId}/agent-permissions`, { agent_ids: agentIDs })
      $q.notify({ type: 'positive', message: '保存成功' })
      return true
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message || '保存失败' })
      return false
    }
  }

  const openRoleDialog = (role?: Role) => {
    if (role) {
      isEditRole.value = true
      selectedRole.value = role
      roleForm.value = { name: role.name, description: role.description || '', is_active: role.is_active }
    } else {
      isEditRole.value = false
      selectedRole.value = null
      roleForm.value = { name: '', description: '', is_active: true }
    }
    roleDialogOpen.value = true
  }

  const openPermissionDialog = async (role: Role) => {
    selectedRole.value = role
    agentSearch.value = ''
    await loadAllAgents()
    await loadRoleAgentPerms(role.id)
    rolePermDialogOpen.value = true
  }

  const openUserRoleDialog = async (user: UserResponse & { user_roles?: any[] }) => {
    selectedUser.value = user
    try {
      const { data } = await api.get<{ data: any[] }>(`/rbac/users/${user.id}/roles`)
      selectedUserCurrentRoles.value = data.data || []
    } catch {
      selectedUserCurrentRoles.value = user.user_roles || []
    }
    userRoleDialogOpen.value = true
  }

  const saveRole = async () => {
    try {
      if (isEditRole.value && selectedRole.value) {
        await api.put(`/rbac/roles/${selectedRole.value.id}`, roleForm.value)
      } else {
        await api.post('/rbac/roles', roleForm.value)
      }
      roleDialogOpen.value = false
      loadRoles()
      $q.notify({ type: 'positive', message: t('saveSuccess') || '保存成功' })
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message || '保存失败' })
    }
  }

  const saveRolePermissions = async () => {
    const ok = await saveAgentPermissions()
    if (!ok) return
    rolePermDialogOpen.value = false
    void loadRoles()
  }

  const availableRoleOptions = computed(() => {
    const currentIds = selectedUserCurrentRoles.value.map((r: any) => r.role_id)
    return roles.value
      .filter(r => !currentIds.includes(r.id))
      .map(r => ({ label: r.name, value: r.id }))
  })

  const revokeUserRole = async (userId: number, roleId: number, roleName: string) => {
    const label = roleName?.trim() || String(roleId)
    $q.dialog({
      title: t('confirm'),
      message: `确定撤销用户角色「${label}」？`,
      cancel: true,
      persistent: true
    }).onOk(async () => {
      try {
        await api.delete(`/rbac/users/${userId}/roles/${roleId}`)
        void loadUsers()
        if (selectedUser.value?.id === userId) {
          try {
            const { data } = await api.get<{ data: any[] }>(`/rbac/users/${userId}/roles`)
            selectedUserCurrentRoles.value = data.data || []
          } catch {
            /* ignore */
          }
        }
        $q.notify({ type: 'positive', message: '撤销成功' })
      } catch (e: unknown) {
        const err = e as { response?: { data?: { message?: string } } }
        $q.notify({ type: 'negative', message: err.response?.data?.message || '撤销失败' })
      }
    })
  }

  /** 分配角色弹窗内撤销：避免模板里 `selectedUser?.id` 与 number 不兼容 */
  const revokeSelectedUserRole = (roleId: number, roleName?: string) => {
    const uid = selectedUser.value?.id
    if (uid == null) return
    void revokeUserRole(uid, roleId, roleName ?? '')
  }

  const deleteRole = async (role: Role) => {
    $q.dialog({
      title: t('confirm'),
      message: `确定删除角色 ${role.name}?`,
      cancel: true,
      persistent: true
    }).onOk(async () => {
      try {
        await api.delete(`/rbac/roles/${role.id}`)
        loadRoles()
        $q.notify({ type: 'positive', message: '删除成功' })
      } catch (e: unknown) {
        const err = e as { response?: { data?: { message?: string } } }
        $q.notify({ type: 'negative', message: err.response?.data?.message || '删除失败' })
      }
    })
  }

  const loadRoles = async () => {
    loading.value = true
    try {
      const { data } = await api.get<{ data: Role[] }>('/rbac/roles')
      roles.value = data.data || []
    } finally {
      loading.value = false
    }
  }

  const loadUsers = async () => {
    loading.value = true
    try {
      const { data } = await api.get<{ data: { list: UserResponse[] } }>('/auth/users')
      const usersData = data.data?.list || []
      for (const user of usersData) {
        try {
          const { data: roleData } = await api.get<{ data: any[] }>(`/rbac/users/${user.id}/roles`)
          user.user_roles = roleData.data || []
        } catch {
          user.user_roles = []
        }
      }
      users.value = usersData
    } finally {
      loading.value = false
    }
  }

  const loadAll = () => {
    loadRoles()
    loadUsers()
  }

  return {
    loading,
    roles,
    users,
    roleDialogOpen,
    rolePermDialogOpen,
    userRoleDialogOpen,
    isEditRole,
    selectedRole,
    selectedUser,
    selectedUserCurrentRoles,
    agentSearch,
    roleForm,
    roleColumns,
    userColumns,
    roleOptions,
    availableRoleOptions,
    filteredAllAgents,
    selectedAgentCount,
    openRoleDialog,
    openPermissionDialog,
    openUserRoleDialog,
    saveRole,
    saveRolePermissions,
    revokeUserRole,
    revokeSelectedUserRole,
    deleteRole,
    loadRoles,
    loadUsers,
    loadAll,
    allAgents,
    isAgentPermitted,
    setAgentAccess,
    selectAllAgents,
    deselectAllAgents
  }
}
