<template>
  <q-page padding>
    <div class="row items-center q-mb-md">
      <div class="text-h6 text-text2">{{ t('mcp') }}</div>
      <q-space />
      <q-btn color="primary" :label="t('createMCP')" icon="add" @click="openDialog()" class="q-mr-sm" unelevated rounded />
      <q-btn flat icon="refresh" round dense @click="load" :loading="loading" />
    </div>

    <q-banner v-if="errorMsg" class="bg-negative text-white q-mb-md" dense>{{ errorMsg }}</q-banner>

    <q-table flat bordered class="radius-sm" :rows="rows" :columns="columns" row-key="id" :loading="loading" :no-data-label="t('noData')">
      <template #body-cell-health_status="props">
        <q-td :props="props">
          <q-badge :color="healthBadgeColor(props.row.health_status)" :label="props.row.health_status || '—'" />
        </q-td>
      </template>
      <template #body-cell-actions="props">
        <q-td :props="props">
          <q-btn dense flat color="primary" :label="t('edit')" @click="openDialog(props.row)" />
          <q-btn dense flat color="secondary" :label="t('sync')" @click="syncMCP(props.row)" :loading="syncingId === props.row.id" />
          <q-btn dense flat color="negative" :label="t('delete')" @click="confirmDelete(props.row)" />
        </q-td>
      </template>
    </q-table>

    <q-dialog v-model="dialogOpen">
      <q-card style="min-width: 600px; max-width: 90vw;">
        <q-card-section class="row items-center">
          <div class="text-h6">{{ editingId ? t('editMCP') : t('createMCP') }}</div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>

        <q-card-section class="q-pt-none">
          <div class="row q-col-gutter-md">
            <div class="col-6">
              <q-input v-model="form.key" :label="t('mcpKey')" outlined dense :rules="[v => !!v]" :readonly="!!editingId" />
            </div>
            <div class="col-6">
              <q-input v-model="form.name" :label="t('mcpName')" outlined dense :rules="[v => !!v]" />
            </div>
          </div>

          <div class="row q-col-gutter-md q-mt-md">
            <div class="col-6">
              <q-select v-model="form.transport" :label="t('transport')" :options="transportOptions" outlined dense />
            </div>
            <div class="col-6">
              <q-input v-model="form.endpoint" :label="t('endpoint')" outlined dense :placeholder="endpointPlaceholder" />
              <div v-if="form.transport === 'stdio'" class="text-caption text-grey-7 q-mt-xs">{{ t('mcpStdioEndpointHint') }}</div>
            </div>
          </div>

          <q-input v-model="form.description" :label="t('description')" outlined dense type="textarea" autogrow class="q-mt-md" />

          <q-input
            v-model="form.usage_hint"
            :label="t('mcpUsageHint')"
            outlined
            dense
            type="textarea"
            :rows="10"
            class="q-mt-md mcp-usage-hint-input"
          />

          <q-input v-model="form.config_json" :label="t('authConfig')" outlined dense type="textarea" rows="6" class="q-mt-md" />

          <div class="q-mt-md">
            <div class="row items-center q-mb-sm">
              <div class="text-subtitle2">{{ t('toolsSnapshot') }}</div>
              <q-space />
              <q-btn v-if="editingId" color="secondary" :label="t('discoverTools')" size="sm" @click="discoverTools" :loading="discoveringTools" />
            </div>
            <q-input v-model="form.tools_json" outlined dense type="textarea" rows="8" placeholder="[{&quot;tool_name&quot;:&quot;list_hosts&quot;,&quot;display_name&quot;:&quot;列出主机&quot;,&quot;description&quot;:&quot;...&quot;,&quot;input_schema&quot;:{}}]" />
          </div>

          <q-checkbox v-model="form.is_active" :label="t('active')" class="q-mt-md" />
        </q-card-section>

        <q-card-actions align="right">
          <q-btn flat :label="t('cancel')" v-close-popup />
          <q-btn color="primary" :label="t('save')" @click="saveMCP" :loading="saving" unelevated />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { useMCPPage, transportOptions, healthBadgeColor } from 'pages/useMcpPage'

defineOptions({ name: 'MCPPage' })

const {
  t,
  loading,
  syncingId,
  saving,
  rows,
  columns,
  errorMsg,
  load,
  dialogOpen,
  form,
  editingId,
  openDialog,
  saveMCP,
  syncMCP,
  confirmDelete,
  discoveringTools,
  discoverTools,
  endpointPlaceholder
} = useMCPPage()
</script>
