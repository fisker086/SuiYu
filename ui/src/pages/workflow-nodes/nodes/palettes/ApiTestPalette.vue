<template>
  <div class="node-item" draggable="true" @dragstart="onDragStart" @click="onClick">
    <div class="node-icon" :style="{ backgroundColor: node.color + '20', color: node.color }">
      <q-icon :name="node.icon" size="18px" />
    </div>
    <div class="node-info">
      <div class="node-name">API Test</div>
      <div class="node-desc">Execute API test</div>
    </div>
  </div>
</template>

<script setup lang="ts">
const node = {
  value: 'apitest',
  label: 'API Test',
  icon: 'science',
  color: '#2196f3',
  desc: 'Execute API test',
  category: 'test'
}

const emit = defineEmits(['drag', 'click'])

function onDragStart (event: DragEvent) {
  event.dataTransfer?.setData('application/vueflow', JSON.stringify(node))
  emit('drag', event)
}

function onClick () {
  emit('click')
}
</script>

<style scoped>
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
.node-name {
  font-size: 13px;
  font-weight: 500;
  color: #333;
}
.node-desc {
  font-size: 11px;
  color: #8c8c8c;
}
</style>
