import type { Component } from 'vue'

import './loader' // auto register all nodes

interface NodeRegistryItem {
  node: {
    value: string
    label: string
    icon: string
    color: string
    desc: string
    category: string
  }
  config: Record<string, unknown>
  Palette: Component
  Config: Component
}

const nodeRegistry = new Map<string, NodeRegistryItem>()

function registerNode (item: NodeRegistryItem) {
  nodeRegistry.set(item.node.value, item)
}

export function getNode (value: string): NodeRegistryItem | undefined {
  return nodeRegistry.get(value)
}

export function getAllNodes (): NodeRegistryItem[] {
  return Array.from(nodeRegistry.values())
}

export function getNodesByCategory (category: string): NodeRegistryItem[] {
  return getAllNodes().filter(n => n.node.category === category)
}

export function getCategories (): string[] {
  const categories = new Set<string>()
  getAllNodes().forEach(n => categories.add(n.node.category))
  return Array.from(categories)
}

export { registerNode }
export type { NodeRegistryItem }
