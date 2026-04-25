<template>
  <q-page padding class="column wf-workflows-page">
    <div class="row items-center q-mb-md wf-page-header">
      <q-btn
        v-if="activeTab === 'editor'"
        flat
        round
        dense
        icon="arrow_back"
        color="grey-8"
        class="wf-header-back q-mr-xs"
        @click="onTabChange('list')"
      >
        <q-tooltip>{{ t('wfBack') }}</q-tooltip>
      </q-btn>
      <div class="text-h6 text-text2">{{ t('workflows') }}</div>
      <q-space />
      <q-btn flat icon="refresh" round dense :loading="loading" @click="loadWorkflows" />
    </div>

    <q-banner v-if="errorMsg" class="bg-negative text-white q-mb-md" dense>{{ errorMsg }}</q-banner>

    <q-tabs v-model="activeTab" dense class="text-grey-7" active-color="primary" indicator-color="primary" align="left" @update:model-value="onTabChange">
      <q-tab name="list" :label="t('wfTabList')" />
      <q-tab name="editor" :label="currentWorkflow ? t('wfTabEditPrefix') + currentWorkflow.name : t('wfTabNew')" />
    </q-tabs>

    <q-separator />

    <q-tab-panels v-model="activeTab" animated class="wf-tab-panels">
      <q-tab-panel name="list">
        <div class="row items-center q-mb-md">
          <q-space />
          <q-btn color="primary" :label="t('wfCreateBtn')" icon="add" unelevated rounded @click="openEditor()" />
        </div>

        <q-table flat bordered :rows="workflowList" :columns="listColumns" row-key="id" :loading="loading" :no-data-label="t('noData')">
          <template #body-cell-kind="props">
            <q-td :props="props">
              <q-badge :color="props.row.kind === 'graph' ? 'purple' : 'blue'" :label="props.row.kind" />
            </q-td>
          </template>
          <template #body-cell-is_active="props">
            <q-td :props="props">
              <q-badge :color="props.row.is_active ? 'positive' : 'grey'" :label="props.row.is_active ? t('active') : t('inactive')" />
            </q-td>
          </template>
          <template #body-cell-actions="props">
            <q-td :props="props">
              <q-btn dense flat color="secondary" icon="history" @click="openWorkflowHistoryFromList(props.row)">
                <q-tooltip>{{ t('wfExecutionHistory') }}</q-tooltip>
              </q-btn>
              <q-btn dense flat color="primary" :label="t('edit')" icon="edit" @click="openEditor(props.row)" />
              <q-btn dense flat color="negative" :label="t('delete')" icon="delete" @click="confirmDelete(props.row)" />
            </q-td>
          </template>
        </q-table>
      </q-tab-panel>

      <q-tab-panel name="editor" class="q-pa-none wf-tab-panel-editor" style="height: calc(100vh - 180px);">
        <div
          class="wf-editor"
          :class="{ 'wf-editor--with-right': configPanelOpen && !rightPanelCollapsed }"
        >
          <!-- 左侧节点面板 -->
          <div class="wf-sidebar wf-left">
            <div class="sidebar-header">
              <q-icon name="widgets" class="q-mr-sm" />
              {{ t('wfNodePalette') }}
            </div>

            <q-input
              v-model="nodeSearch"
              :placeholder="t('wfSearchNodes')"
              dense
              outlined
              class="node-search"
            >
              <template #prepend>
                <q-icon name="search" size="xs" />
              </template>
            </q-input>

            <div class="node-category" v-for="cat in filteredCategories" :key="cat.categoryKey">
              <div class="category-title">
                <q-icon :name="cat.icon" size="16px" />
                {{ cat.name }}
              </div>
              <div class="node-list">
                <component
                  v-for="entry in cat.items"
                  :key="entry.node.value"
                  :is="entry.Palette"
                  @click="addNodeQuick(entry.node.value)"
                />
              </div>
            </div>
          </div>

          <!-- 中间画布：Vue Flow 父级需明确宽高 -->
          <div class="wf-canvas" ref="canvasRef">
            <div class="canvas-toolbar">
              <q-btn-group flat>
                <q-btn flat dense icon="fit_screen" @click="fitView">
                  <q-tooltip>{{ t('wfFitView') }}</q-tooltip>
                </q-btn>
                <q-btn flat dense icon="zoom_in" @click="zoomIn">
                  <q-tooltip>{{ t('wfZoomIn') }}</q-tooltip>
                </q-btn>
                <q-btn flat dense icon="zoom_out" @click="zoomOut">
                  <q-tooltip>{{ t('wfZoomOut') }}</q-tooltip>
                </q-btn>
                <q-btn flat dense icon="center_focus_strong" @click="resetZoom">
                  <q-tooltip>{{ t('wfResetZoom') }}</q-tooltip>
                </q-btn>
              </q-btn-group>
              <q-separator vertical inset class="q-mx-sm" />
              <span class="zoom-level">{{ Math.round(zoom * 100) }}%</span>
              <q-separator vertical inset class="q-mx-sm" />
              <q-btn
                flat
                dense
                icon="history"
                :disable="!currentWorkflow"
                @click="historyDialogOpen = true"
              >
                <q-tooltip>{{ t('wfExecutionHistory') }}</q-tooltip>
              </q-btn>
              <q-btn
                flat
                dense
                icon="play_arrow"
                :disable="!currentWorkflow"
                :loading="executing || runAnimating"
                @click="onRunWorkflow"
              >
                <q-tooltip>{{ t('wfExecute') }}</q-tooltip>
              </q-btn>
              <q-separator vertical inset class="q-mx-sm" />
              <q-btn
                flat
                dense
                :icon="rightPanelCollapsed ? 'chevron_left' : 'chevron_right'"
                :disable="!configPanelOpen"
                @click="toggleRightPropsPanel"
              >
                <q-tooltip>{{ rightPanelCollapsed ? t('wfShowProps') : t('wfHideProps') }}</q-tooltip>
              </q-btn>
            </div>

            <q-btn
              v-if="configPanelOpen && rightPanelCollapsed"
              class="wf-expand-props-fab"
              fab
              mini
              color="primary"
              icon="chevron_left"
              padding="sm"
              @click="rightPanelCollapsed = false"
            >
              <q-tooltip anchor="center left" self="center right">{{ t('wfExpandProps') }}</q-tooltip>
            </q-btn>

            <div class="wf-flow-wrap">
              <VueFlow
                :id="WORKFLOW_FLOW_ID"
                v-model:nodes="nodes"
                v-model:edges="edges"
                :fit-view-on-init="false"
                :default-viewport="{ zoom: 1, x: 0, y: 0 }"
                :default-edge-options="{ markerEnd: MarkerType.ArrowClosed }"
                :connection-line-options="{ markerEnd: MarkerType.ArrowClosed }"
                :delete-key-code="['Backspace', 'Delete']"
                elevate-edges-on-select
                :nodes-connectable="true"
                :edges-updatable="true"
                :min-zoom="0.1"
                :max-zoom="2"
                class="wf-vue-flow-root"
                @node-click="handleNodeClick"
                @pane-click="onPaneClick"
                @edge-click="onEdgeClick"
                @connect="handleConnect"
                @nodes-change="handleNodesChange"
                @drop="handleDrop"
                @dragover="handleDragOver"
                @move-end="onMoveEnd"
                @init="onFlowInit"
              >
                <Background pattern-color="#e0e0e0" :gap="20" />
                <Controls />

                <template #node-default="nodeProps">
                  <WorkflowNode
                    :data="nodeProps.data"
                    :selected="nodeProps.selected"
                    @dblclick="onNodeDoubleClick(nodeProps)"
                    @delete="deleteNodeById(nodeProps.id)"
                  />
                </template>

                <!-- 自定义 edge 槽必须包含 BezierEdge（或 BaseEdge），否则只有标签没有路径，连线不可见且拖拽无法完成 -->
                <template #edge-default="edgeProps">
                  <BezierEdge v-bind="edgeProps" />
                  <EdgeLabelRenderer>
                    <div
                      v-if="edgeProps.data?.label"
                      class="wf-edge-label"
                      :style="getEdgeLabelStyle(edgeProps)"
                    >
                      {{ edgeProps.data.label }}
                    </div>
                  </EdgeLabelRenderer>
                </template>
              </VueFlow>
            </div>
          </div>

          <!-- 右侧配置面板 -->
          <div v-if="configPanelOpen && !rightPanelCollapsed" class="wf-sidebar wf-right">
            <div class="sidebar-header">
              <q-icon name="settings" class="q-mr-sm" />
              {{ t('wfNodeConfig') }}
              <q-space />
              <q-btn flat dense round icon="last_page" size="sm" @click="rightPanelCollapsed = true">
                <q-tooltip>{{ t('wfCollapseSidebar') }}</q-tooltip>
              </q-btn>
              <q-btn flat dense round icon="close" size="sm" @click="onPaneClick" />
            </div>

            <div class="config-section">
              <component
                v-if="selectedNodeId && registryConfigComponent"
                :key="`${selectedNodeId}-${nodeType}`"
                :is="registryConfigComponent"
                :node-label="nodeLabel"
                :config="registryConfigForEditor"
                :nodes="nodes"
                @update:label="onRegistryLabel"
                @update:config="onRegistryConfig"
              />
              <div v-else-if="selectedNodeId" class="text-grey q-pa-md text-caption">
                {{ t('wfUnknownNodeType') }}: {{ nodeType }}
              </div>
            </div>

            <q-separator class="q-my-md" />

            <div class="config-section">
              <div class="config-section-title">
                <q-icon name="link" size="16px" class="q-mr-xs" />
                {{ t('wfInputMapping') }}
                <q-space />
                <VariableInsertDropdown
                  :upstream-nodes="upstreamNodeOptions"
                  :node-output-fields="nodeOutputFields"
                  @insert="onVariableInsert"
                />
              </div>
              <div class="input-mapping-editor">
                <div v-for="(mapping, index) in inputMappings" :key="mapping.id" class="mapping-row">
                  <q-input
                    v-model="mapping.target"
                    outlined dense
                    :placeholder="t('wfMappingTargetPh')"
                    class="mapping-target"
                  />
                  <q-icon name="arrow_forward" size="18px" class="mapping-arrow" />
                  <q-select
                    v-model="mapping.sourceNode"
                    :options="upstreamNodeOptions"
                    outlined dense
                    emit-value
                    map-options
                    :placeholder="t('wfMappingNodePh')"
                    class="mapping-node-select"
                    @update:model-value="onSourceNodeChange(index)"
                  />
                  <q-select
                    v-model="mapping.sourceField"
                    :options="getFieldOptions(mapping.sourceNode)"
                    outlined dense
                    emit-value
                    map-options
                    :placeholder="t('wfMappingFieldPh')"
                    class="mapping-field-select"
                    :disable="!mapping.sourceNode"
                  />
                  <q-btn type="button" flat dense round icon="delete" size="sm" color="negative" @click.stop="removeMapping(index)" />
                </div>
                <q-btn type="button" flat dense color="primary" :label="t('wfAddMapping')" size="sm" class="q-mt-xs" @click.stop="addMapping" />
              </div>
            </div>

            <div class="config-section">
              <div class="config-section-title">
                <q-icon name="input" size="16px" class="q-mr-xs" />
                {{ t('wfInputSchema') }}
                <q-btn flat dense size="sm" color="info" :label="t('wfSchemaExample')" icon="help_outline" @click="showSchemaExample" class="q-ml-sm" />
              </div>
              <div class="schema-editor">
                <q-input
                  v-model="inputSchemaJson"
                  outlined
                  dense
                  type="textarea"
                  rows="4"
                  placeholder="{&quot;type&quot;: &quot;object&quot;, &quot;properties&quot;: {}}"
                  class="config-input"
                  @blur="onInputSchemaBlur"
                />
                <q-btn flat dense size="sm" color="primary" :label="t('wfSchemaExample')" icon="auto_fix_high" @click="fillInputSchemaExample" class="q-mt-xs" />
              </div>
            </div>

            <div class="config-section">
              <div class="config-section-title">
                <q-icon name="output" size="16px" class="q-mr-xs" />
                {{ t('wfOutputSchema') }}
                <q-btn flat dense size="sm" color="info" :label="t('wfSchemaExample')" icon="help_outline" @click="showSchemaExample" class="q-ml-sm" />
              </div>
              <div class="schema-editor">
                <q-input
                  v-model="outputSchemaJson"
                  outlined
                  dense
                  type="textarea"
                  rows="4"
                  placeholder="{&quot;type&quot;: &quot;object&quot;, &quot;properties&quot;: {}}"
                  class="config-input"
                  @blur="onOutputSchemaBlur"
                />
                <q-btn flat dense size="sm" color="primary" :label="t('wfSchemaExample')" icon="auto_fix_high" @click="fillOutputSchemaExample" class="q-mt-xs" />
              </div>
            </div>

            <div class="config-actions">
              <q-btn
                v-if="selectedNodeId"
                color="negative"
                :label="t('wfDeleteNode')"
                icon="delete"
                class="full-width"
                unelevated
                @click="deleteSelectedNode"
              />
            </div>
          </div>

          <!-- 底部工具栏 -->
          <div class="wf-footer">
            <div class="footer-left">
              <q-input
                v-model="workflowName"
                dense
                outlined
                :placeholder="t('wfWorkflowNamePh')"
                class="workflow-name-input"
              />
            </div>
            <div class="footer-right">
              <q-btn
                flat
                color="negative"
                icon="link_off"
                :label="t('wfDeleteEdge')"
                :disable="!hasSelectedEdges"
                class="q-mr-sm"
                @click="deleteSelectedFlowEdges"
              >
                <q-tooltip>{{ t('wfDeleteEdgeHint') }}</q-tooltip>
              </q-btn>
              <q-btn color="primary" :label="t('wfSave')" icon="save" :loading="saving" unelevated @click="saveWorkflow" />
            </div>
          </div>
        </div>
      </q-tab-panel>
    </q-tab-panels>

    <!-- 执行结果对话框（画布运行后立即查看） -->
    <q-dialog v-model="executionDialogOpen" persistent>
      <q-card class="wf-history-dialog-card">
        <q-card-section class="row items-center q-pb-none">
          <div class="text-h6">{{ t('wfExecutionResult') }}</div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>

        <q-card-section>
          <template v-if="executionResult">
            <q-banner class="bg-grey-2 q-mb-md" dense>
              <template #avatar>
                <q-icon :name="executionResult.output ? 'check_circle' : 'error'" :color="executionResult.output ? 'positive' : 'negative'" />
              </template>
              {{ t('wfDuration') }}: {{ executionResult.duration_ms }}ms
            </q-banner>

            <div class="text-subtitle2 q-mb-sm">{{ t('wfOutput') }}:</div>
            <q-card flat bordered class="q-mb-md">
              <q-card-section>
                <pre class="wf-exec-pre">{{ executionResult.output }}</pre>
              </q-card-section>
            </q-card>

            <div class="text-subtitle2 q-mb-sm">{{ t('wfNodeResults') }}:</div>
            <q-card flat bordered>
              <q-card-section>
                <div v-for="[nodeId, result] in nodeResultsEntries" :key="nodeId" class="q-mb-md">
                  <div class="text-weight-bold">{{ (result as Record<string, unknown>).label || nodeId }}</div>
                  <q-chip v-if="(result as Record<string, unknown>).error" color="negative" text-color="white" size="sm">Error</q-chip>
                  <pre class="wf-exec-pre wf-exec-pre--sm">{{ (result as Record<string, unknown>).output }}</pre>
                </div>
              </q-card-section>
            </q-card>
          </template>
        </q-card-section>

        <q-card-actions align="right">
          <q-btn flat :label="t('close')" v-close-popup />
        </q-card-actions>
      </q-card>
    </q-dialog>

    <!-- 执行历史（列表 + 内嵌详情，详情可返回列表） -->
    <!-- full-width：解除 Quasar 在 sm+ 对对话框内层默认 max-width:560px，否则表格会被压得很窄 -->
    <q-dialog v-model="historyDialogOpen" full-width>
      <q-card class="wf-history-dialog-card wf-history-dialog-card--history column">
        <q-card-section class="row items-center q-pb-none">
          <q-btn
            v-if="historyShowDetail"
            flat
            round
            dense
            icon="arrow_back"
            class="q-mr-sm"
            @click="historyDetailBack"
          >
            <q-tooltip>{{ t('wfHistoryBackToList') }}</q-tooltip>
          </q-btn>
          <div class="col">
            <div class="text-h6">{{ historyShowDetail ? t('wfExecutionResult') : t('wfExecutionHistory') }}</div>
            <div v-if="!historyShowDetail && historyDialogTitle" class="text-caption text-grey-7">{{ historyDialogTitle }}</div>
            <div v-if="historyShowDetail && historyDetailMeta" class="text-caption text-grey-7">
              #{{ historyDetailMeta.id }}
              <span v-if="historyDetailMeta.started_at"> · {{ formatWorkflowHistoryStartedAt(historyDetailMeta.started_at) }}</span>
            </div>
          </div>
          <q-space />
          <q-btn v-if="!historyShowDetail" flat round dense icon="refresh" @click="refreshExecutionHistory" />
          <q-btn flat round dense icon="close" v-close-popup />
        </q-card-section>
        <q-card-section class="col scroll wf-history-dialog-body">
          <template v-if="historyShowDetail && executionResult">
            <q-banner class="bg-grey-2 q-mb-md" dense>
              <template #avatar>
                <q-icon :name="executionResult.output ? 'check_circle' : 'error'" :color="executionResult.output ? 'positive' : 'negative'" />
              </template>
              {{ t('wfDuration') }}: {{ executionResult.duration_ms }}ms
              <span v-if="executionResult.node_results && executionResult.node_results.length"> · {{ executionResult.node_results.length }} {{ t('nodes') }}</span>
            </q-banner>

            <div class="text-subtitle2 q-mb-sm">{{ t('wfNodeResults') }}:</div>
            <q-card flat bordered class="q-mb-md">
              <q-card-section class="q-pa-sm">
                <div v-for="(result, idx) in nodeResultsList" :key="String(result.node_id || idx)" class="wf-node-result-card">
                  <div class="wf-node-result-header">
                    <div class="wf-node-result-info">
                      <q-icon
                        :name="result.error ? 'error_outline' : 'check_circle'"
                        :color="result.error ? 'negative' : 'positive'"
                        size="16px"
                        class="q-mr-xs"
                      />
                      <span class="text-weight-bold">{{ result.label || result.node_id }}</span>
                      <q-chip v-if="result.node_type" dense size="sm" color="grey-3" text-color="grey-7" class="q-ml-sm">
                        {{ result.node_type }}
                      </q-chip>
                      <q-chip v-if="result.error" color="negative" text-color="white" size="sm" class="q-ml-xs">
                        Error
                      </q-chip>
                      <q-chip v-if="Number(result.retry_count) > 0" color="orange" text-color="white" size="sm" class="q-ml-xs">
                        重试 {{ result.retry_count }} 次
                      </q-chip>
                    </div>
                    <div class="wf-node-result-meta text-caption text-grey-6">
                      <span v-if="result.start_time && result.end_time">
                        {{ result.start_time }} → {{ result.end_time }}
                      </span>
                      <span v-if="result.duration_ms !== undefined" class="q-ml-sm">
                        {{ result.duration_ms }}ms
                      </span>
                    </div>
                  </div>
                  <div v-if="result.input" class="wf-node-result-section">
                    <div class="text-caption text-grey-7 q-mb-xs">{{ t('input') }}:</div>
                    <pre class="wf-exec-pre wf-exec-pre--sm">{{ result.input }}</pre>
                  </div>
                  <div v-if="result.output" class="wf-node-result-section">
                    <div class="text-caption text-grey-7 q-mb-xs">{{ t('output') }}:</div>
                    <pre class="wf-exec-pre wf-exec-pre--sm">{{ typeof result.output === 'object' ? JSON.stringify(result.output, null, 2) : result.output }}</pre>
                  </div>
                  <div v-if="result.error" class="wf-node-result-section">
                    <div class="text-caption text-negative q-mb-xs">{{ t('error') }}:</div>
                    <pre class="wf-exec-pre wf-exec-pre--sm text-negative">{{ result.error }}</pre>
                  </div>
                </div>
              </q-card-section>
            </q-card>

            <div v-if="executionResult.output" class="q-mt-md">
              <div class="text-subtitle2 q-mb-sm">{{ t('wfOutput') }}:</div>
              <q-card flat bordered>
                <q-card-section>
                  <pre class="wf-exec-pre">{{ executionResult.output }}</pre>
                </q-card-section>
              </q-card>
            </div>
          </template>
          <template v-else>
            <q-banner v-if="!effectiveHistoryWorkflowId" dense class="bg-grey-3">{{ t('wfHistoryNeedWorkflow') }}</q-banner>
            <q-table
              v-else
              class="wf-history-table"
              flat
              bordered
              dense
              wrap-cells
              :rows="executionHistory"
              :columns="historyColumns"
              row-key="id"
              :loading="historyLoading"
              :no-data-label="t('wfHistoryEmpty')"
              v-model:pagination="historyPagination"
              :rows-per-page-options="[10, 20, 50, 100]"
              @row-click="onHistoryRowClick"
            >
              <template #body-cell-status="props">
                <q-td :props="props">
                  <q-badge :color="props.row.status === 'success' ? 'positive' : props.row.status === 'failed' ? 'negative' : 'grey'" :label="props.row.status" />
                </q-td>
              </template>
              <template #body-cell-started_at="props">
                <q-td :props="props" class="wf-history-cell-started">
                  <span class="wf-history-started-text ellipsis">
                    {{ formatWorkflowHistoryStartedAt(props.row.started_at) }}
                    <q-tooltip
                      v-if="props.row.started_at"
                      class="bg-grey-9"
                      anchor="top middle"
                      self="bottom middle"
                      :offset="[0, 6]"
                    >
                      {{ String(props.row.started_at) }}
                    </q-tooltip>
                  </span>
                </q-td>
              </template>
              <template #body-cell-error="props">
                <q-td :props="props" class="wf-history-cell-error">
                  <div class="wf-history-cell-error__clip ellipsis">
                    {{ props.row.error || '—' }}
                    <q-tooltip
                      v-if="props.row.error"
                      class="bg-grey-9"
                      anchor="top middle"
                      self="bottom middle"
                      :offset="[0, 6]"
                    >
                      <div class="wf-history-error-tooltip">{{ props.row.error }}</div>
                    </q-tooltip>
                  </div>
                </q-td>
              </template>
            </q-table>
            <div v-if="effectiveHistoryWorkflowId" class="text-caption text-grey-7 q-mt-sm">{{ t('wfHistoryRowHint') }}</div>
          </template>
        </q-card-section>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useQuasar } from 'quasar'
import type { Component } from 'vue'
import { VueFlow, useVueFlow, EdgeLabelRenderer, BezierEdge, MarkerType } from '@vue-flow/core'
import type { Edge } from '@vue-flow/core'
import type { ExecuteWorkflowResponse, WorkflowExecution, WorkflowDefinition } from 'src/api/types'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import { useWorkflowEditor } from 'pages/useWorkflowEditor'
import 'pages/workflow-nodes/nodes/loader'
import { getNode as getWorkflowNodeRegistry, getAllNodes } from 'pages/workflow-nodes/nodes/index'
import WorkflowNode from 'components/WorkflowNode.vue'
import VariableInsertDropdown from 'pages/workflow-nodes/VariableInsertDropdown.vue'

defineOptions({ name: 'WorkflowsPage' })

const $q = useQuasar()

/** 与 <VueFlow :id> 一致，父组件内 useVueFlow 必须带同一 id，否则 project/连线/视口与画布脱节 */
const WORKFLOW_FLOW_ID = 'workflow-editor-flow'

const {
  project,
  fitView: flowFitView,
  zoomIn: flowZoomIn,
  zoomOut: flowZoomOut,
  setViewport,
  viewport: flowViewport,
  getSelectedEdges,
  removeEdges
} = useVueFlow(WORKFLOW_FLOW_ID)

const hasSelectedEdges = computed(() => getSelectedEdges.value.length > 0)

function deleteSelectedFlowEdges () {
  const sel = getSelectedEdges.value
  if (!sel.length) return
  removeEdges(sel.map(e => e.id))
}

/** 适应画布时不超过 100% 缩放，避免节点被放得过大（例如显示成 139%） */
function fitCanvasView () {
  flowFitView({ padding: 0.15, maxZoom: 1 })
}

const canvasRef = ref<HTMLElement>()
const nodeSearch = ref('')
const zoom = ref(1)
/** 收起右侧属性栏，中间画布占满（与 configPanelOpen 独立：收起后仍保留选中节点） */
const rightPanelCollapsed = ref(false)

const {
  t,
  loading, saving, errorMsg, activeTab, workflowList,
  currentWorkflow, workflowName,
  nodes, edges, selectedNodeId, configPanelOpen, selectedNodeData,
  updateSelectedNodeData,
  openEditor, addNode, onNodeClick, onPaneClick, onEdgeClick, onConnect, onNodesChange,
  deleteSelectedNode,
  deleteNodeById,
  saveWorkflow, confirmDelete, onTabChange, loadWorkflows,
  getUpstreamNodes, getNodeOutputFields,
  executing, executionResult, executionDialogOpen,
  executionHistory,
  executeWorkflowDirect,
  loadExecutionHistory,
  getExecutionDetail
} = useWorkflowEditor()

const runAnimating = ref(false)
const historyDialogOpen = ref(false)
const historyLoading = ref(false)
/** 历史弹窗内：从列表进入某次运行的详情 */
const historyShowDetail = ref(false)
const historyDetailMeta = ref<{ id: number; started_at?: string } | null>(null)
/** 从列表点「历史」时携带，避免未 openEditor 时 currentWorkflow 为空导致无法拉取记录 */
const historyWorkflowIdForDialog = ref<number | null>(null)

/** Quasar q-table 默认每页 5 条，改为默认 10 */
const historyPagination = ref({
  sortBy: 'started_at' as string,
  descending: true,
  page: 1,
  rowsPerPage: 10
})

/** 节点结果列表（带类型安全） */
const nodeResultsList = computed((): Record<string, unknown>[] => {
  const nr = executionResult.value?.node_results as unknown
  if (!nr) return []
  if (Array.isArray(nr)) {
    return nr.map((r) => r as Record<string, unknown>)
  }
  return []
})

function historyDetailBack () {
  historyShowDetail.value = false
  historyDetailMeta.value = null
}

const effectiveHistoryWorkflowId = computed(() =>
  historyWorkflowIdForDialog.value ?? currentWorkflow.value?.id ?? null
)

const historyDialogTitle = computed(() => {
  const id = effectiveHistoryWorkflowId.value
  if (!id) return ''
  const fromList = workflowList.value.find(w => w.id === id)
  const name = fromList?.name ?? currentWorkflow.value?.name
  return name ? `${name} (ID ${id})` : `ID ${id}`
})

function openWorkflowHistoryFromList (wf: WorkflowDefinition) {
  historyWorkflowIdForDialog.value = wf.id
  historyDialogOpen.value = true
}

const nodeResultsEntries = computed(() => {
  const r = executionResult.value?.node_results
  if (!r || typeof r !== 'object') return [] as Array<[string, Record<string, unknown>]>
  return Object.entries(r) as Array<[string, Record<string, unknown>]>
})

/** 错误列固定像素；其余列按比例分「表格宽度 − 错误列」 */
const WF_HISTORY_ERROR_COL_PX = 500

/** 与旧版百分比方案一致：前四列份额之和用于分配剩余宽度（不必为 100） */
const WF_HISTORY_REMAIN_SHARE = {
  id: 6,
  status: 11,
  duration: 9,
  started_at: 50
} as const

const WF_HISTORY_REMAIN_SUM =
  WF_HISTORY_REMAIN_SHARE.id +
  WF_HISTORY_REMAIN_SHARE.status +
  WF_HISTORY_REMAIN_SHARE.duration +
  WF_HISTORY_REMAIN_SHARE.started_at

const historyColumns = computed(() => {
  const s = WF_HISTORY_REMAIN_SHARE
  const sum = WF_HISTORY_REMAIN_SUM
  const err = WF_HISTORY_ERROR_COL_PX
  const wRem = (share: number) =>
    `width: calc((100% - ${err}px) * ${share} / ${sum}); min-width: 0; max-width: calc((100% - ${err}px) * ${share} / ${sum})`
  const wErr = `width: ${err}px; min-width: ${err}px; max-width: ${err}px`
  return [
    { name: 'id', label: 'ID', field: 'id', align: 'left' as const, classes: 'wf-history-col-id', style: wRem(s.id) },
    { name: 'status', label: t('wfHistoryColStatus'), field: 'status', align: 'center' as const, classes: 'wf-history-col-status', style: wRem(s.status) },
    { name: 'duration_ms', label: t('wfDuration'), field: 'duration_ms', align: 'right' as const, classes: 'wf-history-col-duration', style: wRem(s.duration) },
    { name: 'started_at', label: t('wfHistoryColStarted'), field: 'started_at', align: 'left' as const, classes: 'wf-history-col-started', style: wRem(s.started_at) },
    {
      name: 'error',
      label: t('wfHistoryColError'),
      field: 'error',
      align: 'left' as const,
      classes: 'wf-history-col-error',
      style: wErr
    }
  ]
})

/** 执行历史列表：开始时间展示为本地 YYYY-MM-DD HH:mm:ss，避免整段 RFC3339 占满列宽 */
function formatWorkflowHistoryStartedAt (raw: unknown): string {
  if (raw == null || raw === '') return '—'
  const s = String(raw).trim()
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return s.length > 22 ? `${s.slice(0, 22)}…` : s
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function delay (ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}

function snapshotEdgeVisuals (): Map<string, { style?: Record<string, unknown>; animated?: boolean }> {
  const m = new Map<string, { style?: Record<string, unknown>; animated?: boolean }>()
  for (const e of edges.value) {
    m.set(e.id, {
      style: e.style && typeof e.style === 'object' ? { ...(e.style as Record<string, unknown>) } : undefined,
      animated: e.animated
    })
  }
  return m
}

function restoreEdgeVisuals (snap: Map<string, { style?: Record<string, unknown>; animated?: boolean }>) {
  for (const e of edges.value) {
    const s = snap.get(e.id)
    if (!s) continue
    e.style = s.style as typeof e.style
    e.animated = s.animated
  }
}

/** 当接口未返回顺序时，按图上依赖做拓扑排序 */
function inferNodeOrderFromGraph (
  nodeResults: Record<string, unknown> | undefined,
  edgeList: Edge[]
): string[] {
  const keys = nodeResults ? Object.keys(nodeResults) : []
  if (keys.length === 0) return []
  const idSet = new Set(keys)
  const inDeg = new Map<string, number>()
  const adj = new Map<string, string[]>()
  for (const k of keys) {
    inDeg.set(k, 0)
    adj.set(k, [])
  }
  for (const e of edgeList) {
    if (!idSet.has(e.source) || !idSet.has(e.target)) continue
    inDeg.set(e.target, (inDeg.get(e.target) ?? 0) + 1)
    const arr = adj.get(e.source)
    if (arr) arr.push(e.target)
  }
  const q: string[] = []
  for (const [id, d] of inDeg) {
    if (d === 0) q.push(id)
  }
  const out: string[] = []
  while (q.length) {
    const u = q.shift()!
    out.push(u)
    for (const v of adj.get(u) || []) {
      const nd = (inDeg.get(v) ?? 0) - 1
      inDeg.set(v, nd)
      if (nd === 0) q.push(v)
    }
  }
  return out.length ? out : keys
}

async function playRunEdgeAnimation (resp: ExecuteWorkflowResponse): Promise<void> {
  const order = (resp.node_result_order?.length ? resp.node_result_order : inferNodeOrderFromGraph(resp.node_results, edges.value))
  if (order.length < 2) return
  const snap = snapshotEdgeVisuals()
  try {
    for (let i = 1; i < order.length; i++) {
      const prev = order[i - 1]
      const curr = order[i]
      const edge = edges.value.find(e => e.source === prev && e.target === curr)
      if (!edge) continue
      const el = edges.value.find(x => x.id === edge.id)
      if (!el) continue
      if (!el.style || typeof el.style !== 'object') el.style = {}
      const st = el.style as Record<string, string | number>
      st.stroke = '#fb8c00'
      st.strokeWidth = 3
      el.animated = true
      await delay(280)
      st.stroke = '#2e7d32'
      st.strokeWidth = 2.5
      el.animated = false
      await delay(100)
    }
  } finally {
    restoreEdgeVisuals(snap)
  }
}

async function onRunWorkflow () {
  const payload = await executeWorkflowDirect()
  if (!payload) return
  runAnimating.value = true
  try {
    await playRunEdgeAnimation(payload)
  } finally {
    runAnimating.value = false
  }
  const wid = currentWorkflow.value?.id
  if (wid != null) {
    await loadExecutionHistory(wid)
  }
  executionDialogOpen.value = true
}

function workflowExecutionToDisplay (exec: WorkflowExecution): ExecuteWorkflowResponse {
  const rawNr = exec.node_results
  const map: Record<string, unknown> = {}
  if (Array.isArray(rawNr)) {
    for (const row of rawNr) {
      if (row && typeof row === 'object' && 'node_id' in row) {
        const nid = String((row as { node_id: string }).node_id)
        map[nid] = row
      }
    }
  } else if (rawNr && typeof rawNr === 'object' && !Array.isArray(rawNr)) {
    Object.assign(map, rawNr as Record<string, unknown>)
  }
  return {
    output: exec.output,
    node_results: map,
    duration_ms: exec.duration_ms
  }
}

async function refreshExecutionHistory () {
  const wid = effectiveHistoryWorkflowId.value
  if (wid == null) return
  historyLoading.value = true
  try {
    await loadExecutionHistory(wid)
  } finally {
    historyLoading.value = false
  }
}

watch(historyDialogOpen, (open) => {
  if (open) {
    historyPagination.value.page = 1
    void refreshExecutionHistory()
  } else {
    historyWorkflowIdForDialog.value = null
    historyShowDetail.value = false
    historyDetailMeta.value = null
  }
})

async function onHistoryRowClick (_evt: unknown, row: WorkflowExecution) {
  historyLoading.value = true
  try {
    const detail = await getExecutionDetail(row.id)
    executionResult.value = workflowExecutionToDisplay(detail ?? row)
    historyDetailMeta.value = { id: row.id, started_at: row.started_at }
    historyShowDetail.value = true
  } finally {
    historyLoading.value = false
  }
}

watch(activeTab, (tab) => {
  if (tab === 'editor') {
    nextTick(() => fitCanvasView())
  }
})

function toggleRightPropsPanel () {
  if (!configPanelOpen.value) return
  rightPanelCollapsed.value = !rightPanelCollapsed.value
}

watch(rightPanelCollapsed, () => {
  nextTick(() => fitCanvasView())
})

const PALETTE_CATEGORY_ORDER = ['flow', 'ai', 'action', 'data', 'ops', 'notify', 'test']

const nodeCategories = computed(() => {
  const byCat = new Map<string, ReturnType<typeof getAllNodes>>()
  for (const item of getAllNodes()) {
    const k = item.node.category
    if (!byCat.has(k)) byCat.set(k, [])
    const bucket = byCat.get(k)
    if (bucket) bucket.push(item)
  }
  const categoryNames: Record<string, string> = {
    flow: t('wfPaletteCatFlow'),
    ai: t('wfPaletteCatAI'),
    action: t('wfPaletteCatAction'),
    data: t('wfPaletteCatData'),
    ops: t('wfPaletteCatOps'),
    notify: t('wfPaletteCatNotify'),
    test: t('wfPaletteCatTest')
  }
  const categoryIcons: Record<string, string> = {
    flow: 'account_tree',
    ai: 'smart_toy',
    action: 'build',
    data: 'data_usage',
    ops: 'monitor',
    notify: 'notifications',
    test: 'science'
  }
  const keys = [...new Set([...PALETTE_CATEGORY_ORDER, ...byCat.keys()])].filter(k => byCat.has(k))
  return keys.map(catKey => ({
    categoryKey: catKey,
    name: categoryNames[catKey] || catKey,
    icon: categoryIcons[catKey] || 'widgets',
    items: byCat.get(catKey) || []
  }))
})

const filteredCategories = computed(() => {
  const cats = nodeCategories.value
  if (!nodeSearch.value) return cats
  const search = nodeSearch.value.toLowerCase()
  return cats
    .map(cat => ({
      ...cat,
      items: cat.items.filter(entry =>
        entry.node.label.toLowerCase().includes(search) ||
        entry.node.desc.toLowerCase().includes(search)
      )
    }))
    .filter(cat => cat.items.length > 0)
})

const listColumns = computed(() => [
  { name: 'id', label: 'ID', field: 'id', align: 'left' as const },
  { name: 'key', label: t('key'), field: 'key', align: 'left' as const },
  { name: 'name', label: t('name'), field: 'name', align: 'left' as const },
  { name: 'kind', label: t('workflowKind'), field: 'kind', align: 'center' as const },
  { name: 'version', label: t('wfVersion'), field: 'version', align: 'center' as const },
  { name: 'is_active', label: t('isActive'), field: 'is_active', align: 'center' as const },
  { name: 'actions', label: t('actions'), field: 'actions', align: 'right' as const }
])

function getConfig (): Record<string, unknown> {
  return (selectedNodeData.value.config as Record<string, unknown>) || {}
}

const nodeLabel = computed({
  get: () => (selectedNodeData.value.label as string) || '',
  set: (v: string) => { updateSelectedNodeData(d => ({ ...d, label: v })) }
})

const nodeType = computed(() => selectedNodeData.value.nodeType as string)

const registryConfigComponent = computed((): Component | undefined => {
  const nt = nodeType.value
  if (!nt) return undefined
  return getWorkflowNodeRegistry(nt)?.Config
})

const registryConfigForEditor = computed(() => {
  const c = { ...getConfig() } as Record<string, unknown>
  const nt = nodeType.value
  if (nt === 'agent' || nt === 'llm') {
    const aid = selectedNodeData.value.agentId
    if (aid != null && aid !== '') {
      const n = typeof aid === 'number' ? aid : Number(aid)
      if (!Number.isNaN(n)) c.agent_id = n
    }
  }
  return c
})

function onRegistryLabel (v: string) {
  updateSelectedNodeData(d => ({ ...d, label: v }))
}

function onRegistryConfig (cfg: Record<string, unknown>) {
  updateSelectedNodeData(d => {
    const prevCfg = (d.config as Record<string, unknown>) || {}
    /** 与节点表单合并，避免子组件某次 emit 未带上 input_mapping 等「旁路」字段时把整份 config 覆盖掉 */
    const mergedCfg = { ...prevCfg, ...cfg }
    const next: Record<string, unknown> = { ...d, config: mergedCfg }
    if ('agent_id' in cfg && cfg.agent_id !== undefined && cfg.agent_id !== null) {
      const aid = cfg.agent_id
      const n = typeof aid === 'number' ? aid : Number(aid)
      next.agentId = Number.isNaN(n) ? null : n
    }
    return next
  })
}

function newMappingRowId (): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `im_${Date.now()}_${Math.random().toString(36).slice(2, 9)}`
}

interface InputMapping {
  id: string
  target: string
  sourceNode: string | null
  sourceField: string | null
}

const inputMappings = ref<InputMapping[]>([])

/** Reverse `{{.<nodeLabel>.<field>}}` saved in node config → editor rows */
function parseInputMappingFromConfig (
  mapping: Record<string, string> | undefined,
  allNodes: typeof nodes.value
): InputMapping[] {
  if (!mapping || typeof mapping !== 'object') return []
  const labelToId = new Map<string, string>()
  for (const n of allNodes) {
    const data = n.data as Record<string, unknown>
    const label = String(data?.label || n.id)
    labelToId.set(label, n.id)
  }
  const rows: InputMapping[] = []
  for (const [target, expr] of Object.entries(mapping)) {
    if (typeof expr !== 'string') continue
    const m = expr.match(/^\{\{\.([^}]+)\}\}$/)
    if (!m) {
      rows.push({ id: newMappingRowId(), target, sourceNode: null, sourceField: null })
      continue
    }
    const inner = m[1]
    const dotIdx = inner.indexOf('.')
    if (dotIdx < 0) {
      rows.push({ id: newMappingRowId(), target, sourceNode: null, sourceField: null })
      continue
    }
    const sourceLabel = inner.slice(0, dotIdx)
    const sourceField = inner.slice(dotIdx + 1)
    const sourceNode = labelToId.get(sourceLabel) ?? null
    rows.push({ id: newMappingRowId(), target, sourceNode, sourceField: sourceField || null })
  }
  return rows
}

watch(selectedNodeId, (id) => {
  if (!id) {
    inputMappings.value = []
    return
  }
  void nextTick(() => {
    if (selectedNodeId.value !== id) return
    const node = nodes.value.find(n => n.id === id)
    const cfg = (node?.data as Record<string, unknown> | undefined)?.config as Record<string, unknown> | undefined
    const im = cfg?.input_mapping as Record<string, string> | undefined
    inputMappings.value = parseInputMappingFromConfig(im, nodes.value)
  })
}, { immediate: true })

watch(inputMappings, (val) => {
  const mapping: Record<string, string> = {}
  val.forEach(m => {
    if (m.target && m.sourceNode && m.sourceField) {
      const sourceLabel = getNodeLabelById(m.sourceNode)
      mapping[m.target] = `{{.${sourceLabel}.${m.sourceField}}}`
    }
  })
  updateSelectedNodeData(d => {
    const base = (d.config as Record<string, unknown>) || {}
    return { ...d, config: { ...base, input_mapping: mapping } }
  })
}, { deep: true })

function getNodeLabelById (nodeId: string): string {
  const node = nodes.value.find(n => n.id === nodeId)
  if (node) {
    return (node.data as Record<string, unknown>).label as string || nodeId
  }
  return nodeId
}

const upstreamNodeOptions = computed(() => {
  return getUpstreamNodes().map(n => ({
    label: n.label,
    value: n.value,
    nodeType: n.nodeType
  }))
})

const nodeOutputFields = computed(() => {
  const fields: Record<string, string[]> = {}
  for (const n of getUpstreamNodes()) {
    fields[n.value] = getNodeOutputFields(n.value)
  }
  return fields
})

function onVariableInsert (varPath: string) {
  $q.notify({
    type: 'info',
    message: `已复制: ${varPath}`,
    timeout: 1500
  })
  navigator.clipboard?.writeText(varPath)
}

function getFieldOptions (nodeId: string | null): Array<{ label: string; value: string }> {
  if (!nodeId) return []
  return getNodeOutputFields(nodeId).map(f => ({
    label: f,
    value: f
  }))
}

function onSourceNodeChange (index: number) {
  inputMappings.value[index].sourceField = null
}

function addMapping () {
  inputMappings.value.push({
    id: newMappingRowId(),
    target: '',
    sourceNode: null,
    sourceField: null
  })
}

function removeMapping (index: number) {
  inputMappings.value.splice(index, 1)
}

const inputSchemaJson = ref('')
const outputSchemaJson = ref('')

watch(() => selectedNodeData.value.inputSchema, (val) => {
  inputSchemaJson.value = val ? JSON.stringify(val, null, 2) : ''
}, { immediate: true })

watch(() => selectedNodeData.value.outputSchema, (val) => {
  outputSchemaJson.value = val ? JSON.stringify(val, null, 2) : ''
}, { immediate: true })

function onInputSchemaBlur () {
  try {
    updateSelectedNodeData(d => ({ ...d, inputSchema: inputSchemaJson.value ? JSON.parse(inputSchemaJson.value) : undefined }))
  } catch { /* ignore */ }
}

function onOutputSchemaBlur () {
  try {
    updateSelectedNodeData(d => ({ ...d, outputSchema: outputSchemaJson.value ? JSON.parse(outputSchemaJson.value) : undefined }))
  } catch { /* ignore */ }
}

function showSchemaExample () {
  $q.dialog({
    title: t('wfSchemaExampleTitle'),
    message: `<pre style="font-size:12px;white-space:pre-wrap;max-height:400px;overflow:auto;">${t('wfSchemaExampleBody')}</pre>`,
    html: true,
    class: 'schema-example-dialog',
    ok: { label: t('close'), flat: true, color: 'primary' }
  })
}

function fillInputSchemaExample () {
  inputSchemaJson.value = JSON.stringify({
    type: 'object',
    properties: {
      city: { type: 'string', description: '城市名称' },
      date: { type: 'string', description: '日期，格式：YYYY-MM-DD' }
    },
    required: ['city']
  }, null, 2)
  onInputSchemaBlur()
}

function fillOutputSchemaExample () {
  outputSchemaJson.value = JSON.stringify({
    type: 'object',
    properties: {
      temperature: { type: 'number', description: '温度，单位摄氏度' },
      condition: { type: 'string', description: '天气状况' }
    }
  }, null, 2)
  onOutputSchemaBlur()
}

function addNodeQuick (type: string) {
  const center = {
    x: 400 + Math.random() * 200,
    y: 300 + Math.random() * 200
  }
  addNode(type, center)
}

function handleDragOver (event: unknown) {
  const e = event as DragEvent
  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
}

function handleDrop (event: unknown) {
  const e = event as DragEvent
  const bounds = (e.currentTarget as HTMLElement).getBoundingClientRect()
  const data = e.dataTransfer?.getData('application/vueflow')
  if (!data) return

  const nodeTypeInfo = JSON.parse(data) as { value: string }
  const position = project({
    x: e.clientX - bounds.left,
    y: e.clientY - bounds.top
  })

  addNode(nodeTypeInfo.value, position)
}

function handleNodeClick (evt: unknown) {
  const e = evt as { event?: MouseEvent; node?: { id: string } }
  if (e.node?.id) {
    onNodeClick(e.event || new MouseEvent('click'), { id: e.node.id } as any)
  }
}

function onNodeDoubleClick (nodeProps: any) {
  const node = nodeProps.data
  if (node.nodeType === 'input' || node.nodeType === 'output') {
    configPanelOpen.value = true
  }
}

function handleConnect (connection: unknown) {
  onConnect(connection as any)
}

function handleNodesChange (changes: unknown) {
  onNodesChange(changes as any[])
}

function getEdgeLabelStyle (edgeProps: any) {
  const midX = (edgeProps.sourceX + edgeProps.targetX) / 2
  const midY = (edgeProps.sourceY + edgeProps.targetY) / 2
  return {
    transform: `translate(-50%, -50%) translate(${midX}px,${midY}px)`
  }
}

function fitView () {
  fitCanvasView()
}

function zoomIn () {
  flowZoomIn()
}

function zoomOut () {
  flowZoomOut()
}

function resetZoom () {
  setViewport({ x: 0, y: 0, zoom: 1 })
}

function onFlowInit () {
  nextTick(() => fitCanvasView())
}

function onMoveEnd (e: any) {
  zoom.value = e.transform?.zoom ?? flowViewport.value?.zoom ?? 1
}
</script>

<style scoped>
/* 整页纵向铺满，tab 内容区才能算出高度，Vue Flow 父级才有非 0 宽高 */
.wf-workflows-page {
  display: flex;
  flex-direction: column;
  min-height: 0;
  flex: 1 1 auto;
  width: 100%;
}

.wf-tab-panels {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.wf-tab-panels :deep(.q-tab-panel) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* 让 q-tab-panel 成为可计算高度的父级，子链 height:100% 才生效 */
.wf-tab-panel-editor {
  display: flex;
  flex-direction: column;
  min-height: 0;
  box-sizing: border-box;
}

.wf-tab-panel-editor .wf-editor {
  flex: 1 1 auto;
  min-height: 0;
}

.wf-editor {
  display: grid;
  /* 默认两列：无右侧属性时画布占满剩余宽度 */
  grid-template-columns: 240px minmax(0, 1fr);
  grid-template-rows: 1fr 56px;
  height: 100%;
  min-height: 0;
  background: #f5f5f5;
}

.wf-editor.wf-editor--with-right {
  grid-template-columns: 240px minmax(0, 1fr) 400px;
}

.wf-sidebar {
  background: white;
  border-right: 1px solid #e8e8e8;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}

.wf-right {
  border-right: none;
  border-left: 1px solid #e8e8e8;
}

.sidebar-header {
  padding: 12px 16px;
  font-weight: 600;
  font-size: 14px;
  border-bottom: 1px solid #e8e8e8;
  display: flex;
  align-items: center;
  color: #333;
  position: sticky;
  top: 0;
  background: white;
  z-index: 10;
}

.node-search {
  margin: 12px;
}

.node-category {
  padding: 0 12px 12px;
}

.category-title {
  font-size: 12px;
  color: #8c8c8c;
  padding: 8px 4px;
  display: flex;
  align-items: center;
  font-weight: 500;
}

.node-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.node-item {
  display: flex;
  align-items: center;
  padding: 8px 10px;
  border-radius: 6px;
  cursor: grab;
  transition: all 0.2s;
  border: 1px solid transparent;
}

.node-item:hover {
  background: #f5f5f5;
  border-color: #e8e8e8;
  box-shadow: 0 2px 6px rgba(0,0,0,0.08);
}

.node-item:active {
  cursor: grabbing;
}

.node-icon {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-right: 10px;
  flex-shrink: 0;
}

.node-info {
  flex: 1;
  min-width: 0;
}

.node-name {
  font-size: 13px;
  font-weight: 500;
  color: #333;
}

.node-desc {
  font-size: 11px;
  color: #8c8c8c;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.wf-canvas {
  position: relative;
  background: #fafafa;
  min-height: 0;
  height: 100%;
  width: 100%;
  display: flex;
  flex-direction: column;
}

.wf-expand-props-fab {
  position: absolute;
  right: 8px;
  top: 50%;
  transform: translateY(-50%);
  z-index: 11;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.12);
}

.wf-flow-wrap {
  position: absolute;
  inset: 0;
  min-height: 320px;
  width: auto;
  height: auto;
}

.canvas-toolbar {
  position: absolute;
  top: 12px;
  left: 12px;
  z-index: 10;
  background: white;
  border-radius: 6px;
  padding: 4px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.1);
  display: flex;
  align-items: center;
}

.zoom-level {
  font-size: 12px;
  color: #666;
  padding: 0 8px;
}

.config-section {
  padding: 12px 16px;
  border-bottom: 1px solid #f0f0f0;
}

.config-section-title {
  font-size: 13px;
  font-weight: 500;
  color: #333;
  margin-bottom: 12px;
  display: flex;
  align-items: center;
}

.config-group {
  margin-bottom: 16px;
}

.config-label {
  font-size: 13px;
  color: #333;
  margin-bottom: 6px;
  font-weight: 500;
}

.config-hint {
  font-size: 11px;
  color: #8c8c8c;
  font-weight: normal;
  margin-left: 8px;
}

.config-input {
  font-size: 13px;
}

.schema-editor {
  margin-top: 8px;
}

.input-mapping-editor {
  margin-top: 8px;
}

.mapping-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.mapping-target {
  width: 100px;
}

.mapping-node-select {
  width: 120px;
}

.mapping-field-select {
  width: 100px;
}

.mapping-arrow {
  color: #8c8c8c;
}

.config-actions {
  padding: 12px 16px;
  margin-top: auto;
}

.wf-footer {
  grid-column: 1 / -1;
  background: white;
  border-top: 1px solid #e8e8e8;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
}

.footer-left {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
  min-width: 0;
}

.workflow-name-input {
  width: 200px;
}

.footer-right {
  display: flex;
  align-items: center;
}

.wf-edge-label {
  background: #fff;
  border: 1px solid #d9d9d9;
  border-radius: 4px;
  padding: 2px 8px;
  font-size: 12px;
  color: #666;
}
</style>

<style>
@import '@vue-flow/core/dist/style.css';
@import '@vue-flow/controls/dist/style.css';
.vue-flow__node {
  width: auto !important;
  height: auto !important;
}

.vue-flow__node-default,
.vue-flow__node-input,
.vue-flow__node-output {
  width: auto !important;
  height: auto !important;
  padding: 0;
  border: none;
  background: transparent;
}

/* Vue Flow 根节点需填满父级，否则 pane 尺寸为 0、连线无法命中 */
.wf-vue-flow-root.vue-flow,
.wf-flow-wrap .vue-flow {
  width: 100%;
  height: 100%;
  min-height: 280px;
}

/*
 * 连线方向感：沿路径流动的虚线（stroke-dashoffset），配合默认 marker-end 箭头表示从 source → target。
 * 仅作用于可见 path，不影响透明命中层 vue-flow__edge-interaction。
 */
.wf-vue-flow-root .vue-flow__edge-path {
  stroke-width: 1.5;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-dasharray: 10 14;
  animation: wf-edge-flow 1.2s linear infinite;
}

.wf-vue-flow-root .vue-flow__connection-path {
  stroke-linecap: round;
  stroke-dasharray: 10 14;
  animation: wf-edge-flow 1.2s linear infinite;
}

@keyframes wf-edge-flow {
  to {
    stroke-dashoffset: -24;
  }
}

@media (prefers-reduced-motion: reduce) {
  .wf-vue-flow-root .vue-flow__edge-path,
  .wf-vue-flow-root .vue-flow__connection-path {
    animation: none;
    stroke-dasharray: none;
  }
}

.wf-history-dialog-card {
  width: min(1120px, 96vw);
  max-width: 96vw;
  min-width: min(320px, 96vw);
  max-height: 90vh;
}

/* 执行历史：在 full-width 对话框内居中；卡片自身限制宽度，避免贴边或 100vw 滚动条 */
.wf-history-dialog-card--history {
  width: 100%;
  max-width: min(1320px, 100%);
  margin-left: auto;
  margin-right: auto;
  min-width: 0;
}

.wf-history-dialog-body {
  min-width: 0;
}

/* Quasar 把 table-class 加在 .q-table__middle 上；table-layout 必须作用在 table 上 */
.wf-history-table :deep(.q-table__middle.scroll) {
  overflow-x: hidden;
  overflow-y: auto;
}

.wf-history-table :deep(table.q-table) {
  table-layout: fixed;
  width: 100%;
}

.wf-history-table :deep(.q-table th),
.wf-history-table :deep(.q-table td) {
  box-sizing: border-box;
  word-break: break-word;
  overflow-wrap: anywhere;
  vertical-align: top;
}

.wf-history-table :deep(.wf-history-col-id),
.wf-history-table :deep(.wf-history-col-status),
.wf-history-table :deep(.wf-history-col-duration),
.wf-history-table :deep(.wf-history-col-started),
.wf-history-table :deep(.wf-history-col-error) {
  word-break: normal;
  overflow-wrap: normal;
}

/* 开始时间：悬停可看完整 ISO；列宽随表格分配 */
.wf-history-cell-started {
  min-width: 0;
  word-break: normal;
  overflow-wrap: normal;
}

.wf-history-started-text {
  display: block;
  width: 100%;
  min-width: 0;
}

/* 错误列：单行省略；列宽由 WF_HISTORY_ERROR_COL_PX 决定；悬停 tooltip 看全文 */
.wf-history-cell-error {
  min-width: 0;
  max-width: 100%;
  word-break: normal;
  overflow-wrap: normal;
}

.wf-history-cell-error__clip {
  display: block;
  width: 100%;
  min-width: 0;
}

.wf-history-error-tooltip {
  max-width: min(420px, 85vw);
  white-space: pre-wrap;
  word-break: break-word;
}

.wf-exec-pre {
  white-space: pre-wrap;
  word-break: break-word;
  margin: 0;
  font-size: 13px;
}

.wf-exec-pre--sm {
  font-size: 12px;
}

.wf-node-result-card {
  padding: 12px;
  border-bottom: 1px solid #eee;
}
.wf-node-result-card:last-child {
  border-bottom: none;
}

.wf-node-result-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.wf-node-result-info {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
}

.wf-node-result-meta {
  white-space: nowrap;
}

.wf-node-result-section {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px dashed #eee;
}
</style>
