import type { Agent } from 'src/api/types'

/** 从正文解析 @智能体名称，返回 agent id 列表（与群聊 mentions 一致） */
export function extractMentions (text: string, availableAgents: Agent[]): number[] {
  const mentionRegex = /@(\S+)/g
  const mentions: number[] = []
  let match
  while ((match = mentionRegex.exec(text)) !== null) {
    const name = match[1].trim()
    const agent = availableAgents.find(a => a.name === name)
    if (agent) {
      mentions.push(agent.id)
    }
  }
  return mentions
}
