import { registerNode } from './index'

// Import all palette and config components
import StartPalette from './palettes/StartPalette.vue'
import StartConfig from './configs/StartNodeConfig.vue'
import { startNode, defaultConfig as StartDefCfg } from './start'

import EndPalette from './palettes/EndPalette.vue'
import EndConfig from './configs/EndNodeConfig.vue'
import { endNode, defaultConfig as EndDefCfg } from './end'

import AgentPalette from './palettes/AgentPalette.vue'
import AgentConfig from './configs/AgentNodeConfig.vue'
import { agentNode, defaultConfig as AgentDefCfg } from './agent'

import LLMPalette from './palettes/LLMPalette.vue'
import LLMConfig from './configs/LLMNodeConfig.vue'
import { llmNode, defaultConfig as LLMDefCfg } from './llm'

import KnowledgePalette from './palettes/KnowledgePalette.vue'
import KnowledgeConfig from './configs/KnowledgeNodeConfig.vue'
import { knowledgeNode, defaultConfig as KnowledgeDefCfg } from './knowledge'

import ConditionPalette from './palettes/ConditionPalette.vue'
import ConditionConfig from './configs/ConditionNodeConfig.vue'
import { conditionNode, defaultConfig as ConditionDefCfg } from './condition'

import MergePalette from './palettes/MergePalette.vue'
import MergeConfig from './configs/MergeNodeConfig.vue'
import { mergeNode, defaultConfig as MergeDefCfg } from './merge'

import ToolPalette from './palettes/ToolPalette.vue'
import ToolConfig from './configs/ToolNodeConfig.vue'
import { toolNode, defaultConfig as ToolDefCfg } from './tool'

import HttpPalette from './palettes/HttpPalette.vue'
import HttpConfig from './configs/HttpNodeConfig.vue'
import { httpNode, defaultConfig as HttpDefCfg } from './http'

import CodePalette from './palettes/CodePalette.vue'
import CodeConfig from './configs/CodeNodeConfig.vue'
import { codeNode, defaultConfig as CodeDefCfg } from './code'

import TemplatePalette from './palettes/TemplatePalette.vue'
import TemplateConfig from './configs/TemplateNodeConfig.vue'
import { templateNode, defaultConfig as TemplateDefCfg } from './template'

import VariablePalette from './palettes/VariablePalette.vue'
import VariableConfig from './configs/VariableNodeConfig.vue'
import { variableNode, defaultConfig as VariableDefCfg } from './variable'

import SshPalette from './palettes/SshPalette.vue'
import SshConfig from './configs/SshNodeConfig.vue'
import { sshNode, defaultConfig as SshDefCfg } from './ssh'

import NotifyPalette from './palettes/NotifyPalette.vue'
import NotifyConfig from './configs/NotifyNodeConfig.vue'
import { notifyNode, defaultConfig as NotifyDefCfg } from './notify'

import ApiTestPalette from './palettes/ApiTestPalette.vue'
import ApiTestConfig from './configs/ApiTestNodeConfig.vue'
import { apitestNode, defaultConfig as ApiTestDefCfg } from './apitest'

import DataMaskPalette from './palettes/DataMaskPalette.vue'
import DataMaskConfig from './configs/DataMaskNodeConfig.vue'
import { datamaskNode, defaultConfig as DataMaskDefCfg } from './datamask'

// Register all nodes
registerNode({
  node: startNode,
  config: StartDefCfg,
  Palette: StartPalette,
  Config: StartConfig
})

registerNode({
  node: endNode,
  config: EndDefCfg,
  Palette: EndPalette,
  Config: EndConfig
})

registerNode({
  node: agentNode,
  config: AgentDefCfg,
  Palette: AgentPalette,
  Config: AgentConfig
})

registerNode({
  node: llmNode,
  config: LLMDefCfg,
  Palette: LLMPalette,
  Config: LLMConfig
})

registerNode({
  node: knowledgeNode,
  config: KnowledgeDefCfg,
  Palette: KnowledgePalette,
  Config: KnowledgeConfig
})

registerNode({
  node: conditionNode,
  config: ConditionDefCfg,
  Palette: ConditionPalette,
  Config: ConditionConfig
})

registerNode({
  node: mergeNode,
  config: MergeDefCfg,
  Palette: MergePalette,
  Config: MergeConfig
})

registerNode({
  node: toolNode,
  config: ToolDefCfg,
  Palette: ToolPalette,
  Config: ToolConfig
})

registerNode({
  node: httpNode,
  config: HttpDefCfg,
  Palette: HttpPalette,
  Config: HttpConfig
})

registerNode({
  node: codeNode,
  config: CodeDefCfg,
  Palette: CodePalette,
  Config: CodeConfig
})

registerNode({
  node: templateNode,
  config: TemplateDefCfg,
  Palette: TemplatePalette,
  Config: TemplateConfig
})

registerNode({
  node: variableNode,
  config: VariableDefCfg,
  Palette: VariablePalette,
  Config: VariableConfig
})

registerNode({
  node: sshNode,
  config: SshDefCfg,
  Palette: SshPalette,
  Config: SshConfig
})

registerNode({
  node: notifyNode,
  config: NotifyDefCfg,
  Palette: NotifyPalette,
  Config: NotifyConfig
})

registerNode({
  node: apitestNode,
  config: ApiTestDefCfg,
  Palette: ApiTestPalette,
  Config: ApiTestConfig
})

registerNode({
  node: datamaskNode,
  config: DataMaskDefCfg,
  Palette: DataMaskPalette,
  Config: DataMaskConfig
})

export { getNode, getAllNodes, getNodesByCategory, getCategories } from './index'
