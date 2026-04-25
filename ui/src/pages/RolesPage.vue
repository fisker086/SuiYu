<template>
  <q-page padding>
    <div class="row items-center q-mb-md">
      <div class="text-h6">{{ t('roles') }}</div>
      <q-space />
      <q-btn color="primary" :label="t('create')" icon="add" @click="openRoleDialog()" unelevated rounded />
    </div>

    <q-table
      :rows="roles"
      :columns="roleColumns"
      row-key="id"
      :loading="loading"
      flat
    >
      <template #body-cell-name="props">
        <q-td :props="props">
          <span class="text-weight-bold">{{ props.row.name }}</span>
          <q-badge v-if="props.row.is_system" color="orange" :label="t('roleSystem')" class="q-ml-sm" />
        </q-td>
      </template>
      <template #body-cell-is_active="props">
        <q-td :props="props">
          <q-badge
            :color="props.row.is_active ? 'positive' : 'grey'"
            :label="props.row.is_active ? t('roleEnabled') : t('roleDisabled')"
          />
        </q-td>
      </template>
      <template #body-cell-actions="props">
        <q-td :props="props">
          <q-btn flat color="primary" size="sm" :label="t('edit')" @click="openRoleDialog(props.row)" />
          <q-btn flat color="negative" size="sm" :label="t('delete')" @click="deleteRole(props.row)" :disable="props.row.is_system" />
          <q-btn flat color="info" size="sm" :label="t('permissions')" @click="openPermissionDialog(props.row)" />
        </q-td>
      </template>
    </q-table>

    <q-dialog v-model="roleDialogOpen">
      <q-card style="min-width: 400px">
        <q-card-section class="row items-center bg-primary text-white">
          <div class="text-h6">{{ (isEditRole ? t('edit') : t('create')) + ' ' + t('roles') }}</div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>
        <q-card-section>
          <q-input v-model="roleForm.name" :label="t('roleName')" outlined class="q-mb-md" />
          <q-input v-model="roleForm.description" :label="t('roleDescription')" outlined type="textarea" />
          <q-toggle v-model="roleForm.is_active" :label="t('isActive')" />
        </q-card-section>
        <q-card-actions align="right">
          <q-btn flat :label="t('cancel')" v-close-popup />
          <q-btn color="primary" :label="t('save')" @click="saveRole" unelevated />
        </q-card-actions>
      </q-card>
    </q-dialog>

    <q-dialog v-model="rolePermDialogOpen">
      <q-card style="min-width: 500px">
        <q-card-section class="row items-center bg-primary text-white">
          <div class="text-h6">{{ t('roles') }} - {{ t('permissions') }}</div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>
        <q-card-section>
          <div class="q-mb-md">{{ t('rolePermissionRoleLine', { name: selectedRole?.name ?? '—' }) }}</div>
          <q-select
            v-model="selectedPermIDs"
            :options="permissionOptions"
            multiple
            outlined
            :label="t('permissions')"
            emit-value
            map-options
          />
        </q-card-section>
        <q-card-actions align="right">
          <q-btn flat :label="t('cancel')" v-close-popup />
          <q-btn color="primary" :label="t('save')" @click="saveRolePermissions" unelevated />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { onMounted, computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { api } from 'boot/axios'
import type { Role, Permission } from 'src/api/types'

defineOptions({ name: 'RolesPage' })

const { t, locale } = useI18n()
const $q = useQuasar()

const loading = ref(false)
const roles = ref<Role[]>([])
const permissions = ref<Permission[]>([])

const roleDialogOpen = ref(false)
const rolePermDialogOpen = ref(false)
const isEditRole = ref(false)
const selectedRole = ref<Role | null>(null)
const selectedPermIDs = ref<number[]>([])

const roleForm = ref<{ name: string; description: string; is_active: boolean }>({ name: '', description: '', is_active: true })

const roleColumns = computed(() => {
  void locale.value
  return [
    { name: 'name', label: t('roleName'), field: 'name', align: 'left' as const },
    { name: 'description', label: t('description'), field: 'description', align: 'left' as const },
    { name: 'is_active', label: t('status'), field: 'is_active', align: 'center' as const },
    { name: 'actions', label: t('actions'), field: 'actions', align: 'center' as const }
  ]
})

const permissionOptions = computed(() => permissions.value.map(p => ({ label: `${p.resource_type}:${p.resource_name}`, value: p.id })))

const loadRoles = async () => {
  loading.value = true
  try {
    const { data } = await api.get<{ data: Role[] }>('/rbac/roles')
    roles.value = data.data || []
  } finally {
    loading.value = false
  }
}

const loadPermissions = async () => {
  try {
    const { data } = await api.get<{ data: Permission[] }>('/rbac/permissions')
    permissions.value = data.data || []
  } catch {}
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
  selectedPermIDs.value = []
  try {
    const { data } = await api.get<{ data: Role }>(`/rbac/roles/${role.id}`)
    const full = data.data
    if (full) {
      selectedRole.value = full
      selectedPermIDs.value = full.permissions?.map(p => p.id) ?? []
    }
  } catch {
    $q.notify({ type: 'warning', message: t('loadRolePermissionsFailed') })
  }
  rolePermDialogOpen.value = true
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
    $q.notify({ type: 'positive', message: t('saveSuccess') })
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } } }
    $q.notify({ type: 'negative', message: err.response?.data?.message || t('saveFailed') })
  }
}

const saveRolePermissions = async () => {
  try {
    await api.post(`/rbac/roles/${selectedRole.value?.id}/permissions`, { permission_ids: selectedPermIDs.value })
    rolePermDialogOpen.value = false
    loadRoles()
    $q.notify({ type: 'positive', message: t('saveSuccess') })
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } } }
    $q.notify({ type: 'negative', message: err.response?.data?.message || t('saveFailed') })
  }
}

const deleteRole = async (role: Role) => {
  $q.dialog({
    title: t('confirm'),
    message: t('deleteRoleConfirm', { name: role.name }),
    cancel: true,
    persistent: true
  }).onOk(async () => {
    try {
      await api.delete(`/rbac/roles/${role.id}`)
      loadRoles()
      $q.notify({ type: 'positive', message: t('deleteSuccess') })
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message || t('deleteFailed') })
    }
  })
}

onMounted(() => {
  loadRoles()
  loadPermissions()
})
</script>
