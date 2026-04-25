<template>
  <div class="node-palette">
    <q-input
      v-model="searchText"
      :placeholder="searchPlaceholder"
      dense
      outlined
      class="node-search"
    >
      <template #prepend>
        <q-icon name="search" size="xs" />
      </template>
    </q-input>

    <div class="node-category" v-for="cat in filteredCategories" :key="cat.name">
      <div class="category-title">
        <q-icon :name="cat.icon" size="16px" />
        {{ cat.name }}
      </div>
      <div class="node-list">
        <div
          v-for="nt in cat.nodes"
          :key="nt.value"
          class="node-item"
          draggable="true"
          @dragstart="onDragStart($event, nt)"
          @click="onNodeClick(nt)"
        >
          <div class="node-icon" :style="{ backgroundColor: nt.color + '20', color: nt.color }">
            <q-icon :name="nt.icon" size="18px" />
          </div>
          <div class="node-info">
            <div class="node-name">{{ nt.label }}</div>
            <div class="node-desc">{{ nt.desc }}</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { WorkflowNodeType } from './types'
import { getCategories } from './index'

const emit = defineEmits<{
  'node-select': [node: WorkflowNodeType]
  'node-drag': [node: WorkflowNodeType, event: DragEvent]
}>()

const { t } = useI18n()
const searchText = ref('')
const searchPlaceholder = computed(() => t('wfSearchNodes'))

const categories = computed(() => getCategories())

const filteredCategories = computed(() => {
  const cats = categories.value
  if (!searchText.value) return cats
  const search = searchText.value.toLowerCase()
  return cats
    .map(cat => ({
      ...cat,
      nodes: cat.nodes.filter(n =>
        n.label.toLowerCase().includes(search) ||
        n.desc.toLowerCase().includes(search)
      )
    }))
    .filter(cat => cat.nodes.length > 0)
})

function onDragStart (event: DragEvent, node: WorkflowNodeType) {
  event.dataTransfer?.setData('application/vueflow', JSON.stringify(node))
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
  emit('node-drag', node, event)
}

function onNodeClick (node: WorkflowNodeType) {
  emit('node-select', node)
}
</script>

<style scoped>
.node-palette {
  background: white;
  border-right: 1px solid #e8e8e8;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  height: 100%;
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
</style>
