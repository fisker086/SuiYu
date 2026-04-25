<template>
  <q-page padding>
    <div class="row items-center q-mb-md">
      <div class="text-h6">{{ t('users') }}</div>
      <q-space />
      <q-btn flat round dense icon="refresh" @click="loadAll" :loading="loading">
        <q-tooltip>刷新</q-tooltip>
      </q-btn>
    </div>

    <q-table
      :rows="users"
      :columns="userColumns"
      row-key="id"
      :loading="loading"
      flat
    >
      <template #body-cell-roles="props">
        <q-td :props="props">
          <q-chip
            v-for="ur in props.row.user_roles"
            :key="ur.id"
            :color="ur.is_active ? 'positive' : 'grey'"
            removable
            @remove="revokeUserRole(props.row.id, ur.role_id, ur.role?.name)"
          >
            {{ ur.role?.name }}
          </q-chip>
          <q-btn flat dense size="sm" icon="add" @click="openUserRoleDialog(props.row)" />
        </q-td>
      </template>
      <template #body-cell-actions="props">
        <q-td :props="props">
          <q-btn flat color="primary" size="sm" :label="t('assignRole')" @click="openUserRoleDialog(props.row)" />
        </q-td>
      </template>
    </q-table>

    <q-dialog v-model="userRoleDialogOpen">
      <q-card style="min-width: 500px">
        <q-card-section class="row items-center bg-primary text-white">
          <div class="text-h6">{{ t('assignRole') }}: {{ selectedUser?.username }}</div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>
        <q-card-section>
          <div class="text-subtitle2 q-mb-sm text-grey-8">当前角色</div>
          <div class="q-mb-md">
            <q-chip
              v-for="ur in selectedUserCurrentRoles"
              :key="ur.id"
              :color="ur.is_active ? 'positive' : 'grey'"
              removable
              @remove="revokeSelectedUserRole(ur.role_id, ur.role?.name)"
              class="q-mr-sm q-mb-sm"
            >
              {{ ur.role?.name }}
            </q-chip>
            <span v-if="selectedUserCurrentRoles.length === 0" class="text-grey">暂无分配角色</span>
          </div>

          <q-separator class="q-my-md" />

          <div class="text-subtitle2 q-mb-sm text-grey-8">分配新角色</div>
          <q-select
            v-model="newUserRoleId"
            :options="availableRoleOptions"
            outlined
            label="选择角色"
            emit-value
            map-options
            clearable
            class="q-mb-md"
          >
            <template #append>
              <q-btn flat dense color="primary" label="分配" @click="handleAssignRole" :disable="!newUserRoleId" />
            </template>
          </q-select>
        </q-card-section>
        <q-card-actions align="right">
          <q-btn flat :label="t('close')" v-close-popup />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { useRBACPage } from 'src/pages/useRBACPage'
import { api } from 'boot/axios'

defineOptions({ name: 'UsersPage' })

const { t } = useI18n()
const $q = useQuasar()

const {
  loading,
  users,
  userRoleDialogOpen,
  selectedUser,
  selectedUserCurrentRoles,
  userColumns,
  availableRoleOptions,
  openUserRoleDialog,
  revokeUserRole,
  revokeSelectedUserRole,
  loadAll
} = useRBACPage()

const newUserRoleId = ref<number | null>(null)

const handleAssignRole = async () => {
  if (!newUserRoleId.value || !selectedUser.value?.id) return
  try {
    await api.post(`/rbac/users/${selectedUser.value.id}/roles`, { role_id: newUserRoleId.value })
    newUserRoleId.value = null
    await loadAll()
    const { data } = await api.get<{ data: any[] }>(`/rbac/users/${selectedUser.value.id}/roles`)
    selectedUserCurrentRoles.value = data.data || []
    $q.notify({ type: 'positive', message: t('saveSuccess') })
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } } }
    $q.notify({ type: 'negative', message: err.response?.data?.message || '分配失败' })
  }
}

onMounted(() => {
  loadAll()
})
</script>
