import type { Agent } from 'src/api/types'

/**
 * Build select labels for agent permission UI: append #id only when multiple agents share the same name.
 */
export function labelsForAgentPermissionSelect (
  agents: Agent[],
  labelWhenDuplicate: (a: Agent) => string
): { label: string; value: number }[] {
  const nameCount = new Map<string, number>()
  for (const x of agents) {
    nameCount.set(x.name, (nameCount.get(x.name) || 0) + 1)
  }
  return agents.map((a) => ({
    label: (nameCount.get(a.name) || 0) > 1 ? labelWhenDuplicate(a) : a.name,
    value: a.id
  }))
}
