import { onMounted, ref } from 'vue'
import { api } from 'boot/axios'
import type { APIResponse, MCPConfig, MCPTool, Skill } from 'src/api/types'

export type ToolNameOption = { label: string; value: string }

function dedupeByValue (ordered: ToolNameOption[]): ToolNameOption[] {
  const seen = new Set<string>()
  return ordered.filter((o) => {
    if (seen.has(o.value)) return false
    seen.add(o.value)
    return true
  })
}

/** GET /skills：已启用且仅服务端执行（排除 execution_mode=client，与 workflow 在服务端跑 tool 一致） */
async function loadSkillToolOptions (): Promise<ToolNameOption[]> {
  try {
    const { data } = await api.get<APIResponse<Skill[]>>('/skills')
    const out: ToolNameOption[] = []
    for (const s of data.data ?? []) {
      if (!s.is_active) continue
      const mode = (s.execution_mode || '').trim().toLowerCase()
      if (mode !== 'server') continue
      const key = (s.key || '').trim()
      if (!key) continue
      const name = (s.name || '').trim()
      const label = name ? `${name} (${key})` : key
      out.push({ label, value: key })
    }
    return out
  } catch {
    return []
  }
}

/** 聚合各已启用 MCP 的 tools 列表 */
async function loadMcpToolOptions (): Promise<ToolNameOption[]> {
  try {
    const { data } = await api.get<APIResponse<MCPConfig[]>>('/mcp/configs')
    const configs = (data.data ?? []).filter((c) => c.is_active)
    const merged: ToolNameOption[] = []
    await Promise.all(
      configs.map(async (cfg) => {
        try {
          const { data: toolsRes } = await api.get<APIResponse<MCPTool[]>>(
            `/mcp/configs/${cfg.id}/tools`
          )
          for (const t of toolsRes.data ?? []) {
            if (!t.tool_name) continue
            const base = t.display_name?.trim()
              ? `${t.display_name} (${t.tool_name})`
              : t.tool_name
            merged.push({ label: `${base} · ${cfg.name}`, value: t.tool_name })
          }
        } catch {
          // 单个 MCP 失败时跳过
        }
      })
    )
    return merged
  } catch {
    return []
  }
}

/** Tool 节点下拉：Skills（主） + MCP 工具；同 value 保留先出现的（Skills 优先） */
export function useToolNodeNameOptions () {
  const loading = ref(false)
  const toolOptions = ref<ToolNameOption[]>([])

  async function load (): Promise<void> {
    loading.value = true
    try {
      const [skillOpts, mcpOpts] = await Promise.all([
        loadSkillToolOptions(),
        loadMcpToolOptions()
      ])
      toolOptions.value = dedupeByValue([...skillOpts, ...mcpOpts])
    } finally {
      loading.value = false
    }
  }

  onMounted(() => {
    void load()
  })

  return { loading, toolOptions, load }
}
