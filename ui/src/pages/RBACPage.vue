<template>
  <q-page padding>
    <div class="row items-center q-mb-md">
      <div class="text-h6">{{ t('roles') }}</div>
      <q-space />
      <q-btn flat round dense icon="refresh" @click="loadRoles" :loading="loading" class="q-mr-sm">
        <q-tooltip>刷新</q-tooltip>
      </q-btn>
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
          <q-badge v-if="props.row.is_system" color="orange" label="system" class="q-ml-sm" />
        </q-td>
      </template>
      <template #body-cell-user_count="props">
        <q-td :props="props">
          <q-badge :color="props.row.user_count > 0 ? 'primary' : 'grey'" :label="String(props.row.user_count ?? 0)" />
        </q-td>
      </template>
      <template #body-cell-agent_count="props">
        <q-td :props="props">
          <q-badge :color="(props.row.agent_count ?? 0) > 0 ? 'teal' : 'grey'" :label="String(props.row.agent_count ?? 0)" />
        </q-td>
      </template>
      <template #body-cell-is_active="props">
        <q-td :props="props">
          <q-badge :color="props.row.is_active ? 'positive' : 'grey'" :label="props.row.is_active ? '启用' : '停用'" />
        </q-td>
      </template>
      <template #body-cell-actions="props">
        <q-td :props="props">
          <q-btn flat color="primary" size="sm" :label="t('edit')" @click="openRoleDialog(props.row)" />
          <q-btn flat color="negative" size="sm" :label="t('delete')" @click="deleteRole(props.row)" :disable="props.row.is_system" />
          <q-btn flat color="info" size="sm" label="智能体权限" @click="openPermissionDialog(props.row)" />
        </q-td>
      </template>
    </q-table>

    <q-dialog v-model="roleDialogOpen">
      <q-card style="min-width: 400px">
        <q-card-section class="row items-center bg-primary text-white">
          <div class="text-h6">{{ isEditRole ? t('edit') : t('create') }}{{ t('roles') }}</div>
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
      <q-card style="min-width: 600px; max-width: 80vw">
        <q-card-section class="row items-center bg-primary text-white">
          <div class="text-h6">智能体权限: {{ selectedRole?.name }}</div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>
        <q-card-section>
          <div class="text-caption text-grey-7 q-mb-sm">
            勾选此角色可访问的智能体。用户被分配此角色后，将只能访问被勾选的智能体。
          </div>
          <div class="row items-center q-mb-md q-gutter-sm">
            <q-input
              v-model="agentSearch"
              outlined
              dense
              placeholder="搜索智能体..."
              clearable
              class="col"
            >
              <template #prepend>
                <q-icon name="search" />
              </template>
            </q-input>
            <q-btn flat size="sm" color="primary" label="全选" @click="selectAllAgents" />
            <q-btn flat size="sm" color="grey" label="清空" @click="deselectAllAgents" />
            <q-badge color="primary" :label="`已选 ${selectedAgentCount} / ${allAgents.length}`" />
          </div>
          <div class="agent-perm-grid">
            <div v-for="agent in filteredAllAgents" :key="agent.id" class="agent-perm-item">
              <q-checkbox
                :model-value="isAgentPermitted(agent.id)"
                :label="agent.name"
                @update:model-value="(val: boolean) => setAgentAccess(agent.id, val)"
              />
            </div>
          </div>
          <div v-if="filteredAllAgents.length === 0 && allAgents.length > 0" class="text-grey q-pa-md text-center">
            没有找到匹配的智能体
          </div>
          <div v-if="allAgents.length === 0" class="text-grey q-pa-md text-center">
            暂无可分配的智能体，请先创建智能体
          </div>
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
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRBACPage } from 'src/pages/useRBACPage'

const { t } = useI18n()

const {
  loading,
  roles,
  allAgents,
  roleDialogOpen,
  rolePermDialogOpen,
  isEditRole,
  selectedRole,
  agentSearch,
  roleForm,
  roleColumns,
  filteredAllAgents,
  selectedAgentCount,
  openRoleDialog,
  openPermissionDialog,
  saveRole,
  saveRolePermissions,
  deleteRole,
  loadRoles,
  isAgentPermitted,
  setAgentAccess,
  selectAllAgents,
  deselectAllAgents
} = useRBACPage()

onMounted(() => {
  loadRoles()
})
</script>

<style scoped>
.agent-perm-grid {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 400px;
  overflow-y: auto;
}

.agent-perm-item {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 8px 12px;
  background: #fafafa;
  border-radius: 6px;
}
</style>
