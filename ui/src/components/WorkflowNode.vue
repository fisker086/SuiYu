<template>
  <div class="workflow-node" :class="{ selected: selected, [`type-${data.nodeType}`]: true }">
    <div class="node-header">
      <div class="node-icon" :style="{ backgroundColor: headerColor }">
        <q-icon :name="nodeIcon" size="14px" color="white" />
      </div>
      <div class="node-title">{{ data.label || '未命名节点' }}</div>
      <q-btn v-if="data.nodeType !== 'input' && data.nodeType !== 'output'" flat dense round size="sm" icon="more_vert" class="node-menu">
        <q-menu>
          <q-list dense style="min-width: 100px">
            <q-item clickable v-close-popup @click="$emit('dblclick', $event)">
              <q-item-section>编辑</q-item-section>
            </q-item>
            <q-item clickable v-close-popup @click="$emit('delete', $event)">
              <q-item-section>删除</q-item-section>
            </q-item>
          </q-list>
        </q-menu>
      </q-btn>
    </div>

    <div class="node-body">
      <div v-if="data.nodeType === 'agent' && data.agentId" class="node-badge agent-badge">
        <q-icon name="smart_toy" size="12px" />
        <span>智能体 #{{ data.agentId }}</span>
      </div>

      <div v-if="data.nodeType === 'llm'" class="node-badge llm-badge">
        <q-icon name="psychology" size="12px" />
        <span>LLM</span>
      </div>

      <template v-if="hasPrompt && promptPreview">
        <div class="node-prompt">
          <q-icon name="text_snippet" size="12px" class="q-mr-xs" />
          {{ promptPreview }}
        </div>
      </template>

      <template v-if="hasCondition">
        <div class="node-condition">
          <q-icon name="call_split" size="12px" class="q-mr-xs" />
          {{ conditionText }}
        </div>
      </template>

      <template v-if="hasToolName">
        <div class="node-tool">
          <q-icon name="build" size="12px" class="q-mr-xs" />
          {{ config.tool_name }}
        </div>
      </template>

      <div v-if="hasInputSchema || hasOutputSchema" class="node-ports">
        <div v-if="hasInputSchema" class="port-info">
          <q-icon name="arrow_downward" size="10px" color="green-6" />
          <span>{{ inputSchemaFieldCount }} 输入</span>
        </div>
        <div v-if="hasOutputSchema" class="port-info">
          <q-icon name="arrow_upward" size="10px" color="blue-6" />
          <span>{{ outputSchemaFieldCount }} 输出</span>
        </div>
      </div>
    </div>

    <!-- 连接点：显式 id 便于 Vue Flow 校验 source/target handle -->
    <Handle id="target" type="target" :position="Position.Left" class="handle handle-input" connectable />
    <Handle
      v-if="data.nodeType !== 'output'"
      id="source"
      type="source"
      :position="Position.Right"
      class="handle handle-output"
      connectable
    />
  </div>
</template>

<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'
import { useWorkflowNode, type WorkflowNodeData } from './WorkflowNode'

const props = defineProps<{
  data: WorkflowNodeData
  selected: boolean
}>()

defineEmits(['dblclick', 'delete'])

const {
  headerColor,
  nodeIcon,
  config,
  hasPrompt,
  promptPreview,
  hasCondition,
  conditionText,
  hasToolName,
  hasInputSchema,
  inputSchemaFieldCount,
  hasOutputSchema,
  outputSchemaFieldCount
} = useWorkflowNode(props.data, props.selected)
</script>

<style scoped>
.workflow-node {
  background: white;
  border-radius: 10px;
  min-width: 140px;
  width: auto;
  box-shadow: 0 2px 8px rgba(0,0,0,0.08);
  border: 1px solid #e8e8e8;
  transition: all 0.2s ease;
  overflow: hidden;
}

.workflow-node:hover {
  box-shadow: 0 4px 16px rgba(0,0,0,0.12);
}

.workflow-node.selected {
  box-shadow: 0 0 0 2px #1890ff, 0 4px 16px rgba(24,144,255,0.25);
  border-color: #1890ff;
}

.node-header {
  padding: 6px 10px;
  display: flex;
  align-items: center;
  background: linear-gradient(to right, rgba(0,0,0,0.02), transparent);
  border-bottom: 1px solid #f0f0f0;
}

.node-icon {
  width: 24px;
  height: 24px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-right: 8px;
  flex-shrink: 0;
}

.node-title {
  flex: 1;
  font-size: 13px;
  font-weight: 600;
  color: #333;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.node-menu {
  opacity: 0;
  transition: opacity 0.2s;
}

.workflow-node:hover .node-menu {
  opacity: 1;
}

.node-body {
  padding: 6px 10px;
  font-size: 11px;
}

.node-badge {
  display: inline-flex;
  align-items: center;
  padding: 3px 8px;
  border-radius: 4px;
  font-size: 11px;
  margin-bottom: 6px;
}

.agent-badge {
  background: #f3e8ff;
  color: #722ED1;
}

.llm-badge {
  background: #fce4ec;
  color: #EB2F96;
}

.node-prompt, .node-tool {
  color: #666;
  font-size: 11px;
  padding: 4px 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.node-condition {
  color: #d46b08;
  background: #fff7e6;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 11px;
  margin-top: 4px;
}

.node-ports {
  display: flex;
  gap: 12px;
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px dashed #f0f0f0;
}

.port-info {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 10px;
  color: #8c8c8c;
}

.handle {
  width: 10px;
  height: 10px;
  background: white;
  border: 2px solid #1890ff;
}

.handle:hover {
  background: #1890ff;
}
</style>
