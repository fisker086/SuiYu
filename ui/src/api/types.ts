export interface UserResponse {
  id: number
  username: string
  email: string
  full_name?: string
  avatar_url?: string
  status: string
  is_admin?: boolean
  user_roles?: { id: number; role_id: number; role?: { name: string } }[]
}

export interface TokenResponse {
  access_token: string
  refresh_token: string
  token_type: string
  expires_in: number
  user: UserResponse
}

export interface APIResponse<T = unknown> {
  code: number
  message: string
  data?: T
}

export interface Skill {
  id: number
  key: string
  name: string
  description?: string
  content?: string
  source_ref: string
  prompt_hint?: string
  category?: string
  risk_level?: string
  execution_mode?: string
  is_active: boolean
  created_at: string
  updated_at?: string
}

export interface MCPTool {
  tool_name: string
  display_name?: string
  description?: string
  input_schema?: Record<string, unknown>
  output_schema?: Record<string, unknown>
  is_active?: boolean
}

export interface NotifyChannel {
  id: number
  name: string
  kind: 'lark' | 'dingtalk' | 'wecom' | string
  webhook_url?: string
  app_id?: string
  has_app_secret: boolean
  extra?: Record<string, string>
  is_active: boolean
  created_at: string
}

export interface MCPConfig {
  id: number
  key: string
  name: string
  description?: string
  transport: string
  endpoint?: string
  /** 后端返回的扩展字段（含 description、usage_hint、headers 等） */
  config?: Record<string, unknown>
  config_json?: string
  is_active: boolean
  health_status: string
  tool_count: number
  validation_status?: string
  tools?: MCPTool[]
  created_at: string
  updated_at?: string
}

export interface Agent {
  id: number
  /** Opaque id for URLs (e.g. /chat/:public_id); not the DB serial. */
  public_id: string
  name: string
  description: string
  category: string
  is_builtin?: boolean
  is_active?: boolean
  created_at?: string
  updated_at?: string
  skill_ids?: string[]
  mcp_config_ids?: number[]
}

export interface RuntimeProfile {
  source_agent: string
  archetype: string
  role?: string
  goal?: string
  backstory?: string
  system_prompt?: string
  llm_model?: string
  temperature: number
  stream_enabled: boolean
  memory_enabled: boolean
  skill_ids?: string[]
  mcp_config_ids?: number[]
  execution_mode?: string
  max_iterations?: number
  plan_prompt?: string
  reflection_depth?: number
  approval_mode?: string
}

export interface CreateAgentRequest {
  name: string
  description: string
  category: string
  is_active?: boolean
  runtime_profile?: RuntimeProfile
}

export interface UpdateAgentRequest {
  name?: string
  description?: string
  category?: string
  is_active?: boolean
  runtime_profile?: RuntimeProfile
}

export interface ChatResponseData {
  message: string
  session_id: string
  agent_id?: number
  workflow_id?: number
  duration_ms?: number
}

export interface ChatSession {
  session_id: string
  agent_id: number
  user_id?: string
  /** 群聊线程时由后端写入，用于 URL ?session= 恢复 */
  group_id?: number
  /** 展示名：首条用户消息摘要或用户重命名 */
  title?: string
  created_at: string
  updated_at: string
}

/** ReAct / ADK 步骤快照（主聊天区内联展示；服务端历史消息通常无此字段） */
export interface ChatReactStep {
  type: string
  data: Record<string, unknown>
  meta?: Record<string, unknown>
  timestamp?: string
}

export interface ChatHistoryMessage {
  id: number
  /** 发言所属智能体（群聊多智能体时用于气泡标题；单聊也有） */
  agent_id?: number
  role: string
  content: string
  /** 图片地址列表（相对路径需经 resolveChatImageUrl / 同源拼接后用于 img src） */
  image_urls: string[]
  file_urls: string[]
  /** 服务端 GET /messages 返回的 ReAct/ADK 步骤（SSE 快照） */
  react_steps?: unknown[]
  created_at: string
}

export interface CreateSkillRequest {
  key: string
  name: string
  description?: string
  content?: string
  source_ref: string
}

export interface CreateMCPConfigRequest {
  key: string
  name: string
  description?: string
  transport?: string
  endpoint?: string
  config_json?: string
  is_active?: boolean
  tools_json?: string
}

export interface AgentWorkflow {
  id: number
  key: string
  name: string
  description: string
  kind: string
  step_agent_ids: number[]
  config?: Record<string, unknown>
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface CreateWorkflowRequest {
  key: string
  name: string
  description?: string
  kind: string
  step_agent_ids: number[]
  config?: Record<string, unknown>
  is_active?: boolean
}

export interface UpdateWorkflowRequest {
  name?: string
  description?: string
  kind?: string
  step_agent_ids?: number[]
  config?: Record<string, unknown>
  is_active?: boolean
}

export interface UpdateProfileRequest {
  email?: string
  full_name?: string
}

export interface ChangePasswordRequest {
  current_password: string
  new_password: string
}

export type Action = 1 | 2 | 4 | 8

export interface Permission {
  id: number
  resource_type: string
  resource_name: string
  actions: Action
  description?: string
  is_system: boolean
  created_at: string
}

export interface Role {
  id: number
  name: string
  description?: string
  is_system: boolean
  is_active: boolean
  permissions?: Permission[]
  user_count?: number
  agent_count?: number
  created_at: string
  updated_at: string
}

export interface UserRole {
  id: number
  user_id: number
  role_id: number
  is_active: boolean
  expires_at?: string
  created_at: string
  role?: Role
}

export interface CreatePermissionRequest {
  resource_type: string
  resource_name: string
  actions: Action
  description?: string
}

export interface CreateRoleRequest {
  name: string
  description?: string
  is_active?: boolean
}

export interface UpdateRoleRequest {
  name?: string
  description?: string
  is_active?: boolean
}

export interface AssignRoleRequest {
  role_id: number
}

export interface SetRolePermissionsRequest {
  permission_ids: number[]
}

export interface MessageChannel {
  id: number
  name: string
  agent_id: number
  agent_name?: string
  kind: 'direct' | 'broadcast' | 'topic' | string
  description: string
  is_public: boolean
  metadata?: Record<string, string>
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface CreateMessageChannelRequest {
  name: string
  agent_id: number
  kind: string
  description?: string
  is_public?: boolean
  metadata?: Record<string, string>
  is_active?: boolean
}

export interface UpdateMessageChannelRequest {
  name?: string
  description?: string
  is_public?: boolean
  metadata?: Record<string, string>
  is_active?: boolean
}

export interface AgentMessage {
  id: number
  from_agent_id: number
  from_agent_name?: string
  to_agent_id: number
  to_agent_name?: string
  channel_id: number
  session_id: string
  kind: 'text' | 'command' | 'event' | 'result' | string
  content: string
  metadata?: Record<string, unknown>
  status: string
  priority: number
  created_at: string
  delivered_at?: string
}

export interface SendMessageRequest {
  from_agent_id: number
  to_agent_id: number
  channel_id?: number
  session_id?: string
  kind: string
  content: string
  metadata?: Record<string, unknown>
  priority?: number
}

export interface MessageSendResponse {
  message_id: number
  status: string
  delivered_at?: string
}

export interface MessageSpanRequest {
  from_agent_id: number
  to_agent_id: number
  channel_id?: number
  session_id?: string
  content: string
  metadata?: Record<string, unknown>
}

export interface MessageSpanResponse {
  span_id: string
  message_id: number
  status: string
  trace_id?: string
}

export interface A2ACard {
  id: number
  agent_id: number
  name: string
  description: string
  url: string
  version: string
  capabilities?: string[]
  is_active: boolean
  created_at: string
}

export interface CreateA2ACardRequest {
  agent_id: number
  name: string
  description?: string
  url?: string
  version?: string
  capabilities?: string[]
  is_active?: boolean
}

export interface WorkflowNode {
  id: string
  type: string
  label: string
  agent_id?: number
  config?: Record<string, unknown>
  position?: { x: number; y: number }
  input_schema?: Record<string, unknown>
  output_schema?: Record<string, unknown>
}

export interface WorkflowEdge {
  id: string
  source_node_id: string
  source_port?: string
  target_node_id: string
  target_port?: string
  condition?: string
  label?: string
}

export interface WorkflowDefinition {
  id: number
  key: string
  name: string
  description?: string
  kind: string
  nodes: WorkflowNode[]
  edges: WorkflowEdge[]
  variables?: Record<string, unknown>
  input_schema?: Record<string, unknown>
  output_schema?: Record<string, unknown>
  version: number
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface CreateWorkflowDefinitionRequest {
  key: string
  name: string
  description?: string
  kind: string
  nodes: WorkflowNode[]
  edges: WorkflowEdge[]
  variables?: Record<string, unknown>
  input_schema?: Record<string, unknown>
  output_schema?: Record<string, unknown>
  is_active?: boolean
}

export interface UpdateWorkflowDefinitionRequest {
  name?: string
  description?: string
  kind?: string
  nodes?: WorkflowNode[]
  edges?: WorkflowEdge[]
  variables?: Record<string, unknown>
  input_schema?: Record<string, unknown>
  output_schema?: Record<string, unknown>
  is_active?: boolean
}

export interface ExecuteWorkflowRequest {
  workflow_id: number
  message: string
  variables?: Record<string, unknown>
}

export interface ExecuteWorkflowResponse {
  output: unknown
  /** Same as backend JSON key `node_results` */
  node_results?: Record<string, unknown>
  /** Nodes finished in order (for canvas edge animation) */
  node_result_order?: string[]
  duration_ms: number
  execution_id?: number
}

export interface WorkflowExecution {
  id: number
  workflow_id: number
  workflow_key: string
  status: string
  input: string
  output: string
  error: string
  node_results: Record<string, unknown>[]
  variables: Record<string, unknown>
  duration_ms: number
  started_at: string
  finished_at: string
  created_by: string
}

export interface Schedule {
  id: number
  name: string
  description?: string
  agent_id?: number
  workflow_id?: number
  workflow_name?: string
  agent_name?: string
  /** python | shell | javascript — code task when set */
  code_language?: string
  channel_id?: number
  channel_name?: string
  schedule_kind: 'at' | 'every' | 'cron'
  cron_expr?: string
  at?: string
  every_ms?: number
  timezone?: string
  wake_mode: string
  session_target: string
  /** Present after first run; same id as chat session list (open in UI). */
  chat_session_id?: string
  prompt: string
  stagger_ms: number
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface CreateScheduleRequest {
  name: string
  description?: string
  agent_id?: number
  workflow_id?: number
  channel_id?: number
  /** python | shell | javascript — mutually exclusive with agent_id / workflow_id */
  code_language?: string
  schedule_kind: 'at' | 'every' | 'cron'
  cron_expr?: string
  at?: string
  every_ms?: number
  timezone?: string
  wake_mode?: string
  session_target?: string
  /** User message (agent/workflow) or source code (code_language) */
  prompt: string
  stagger_ms?: number
  enabled?: boolean
}

export interface UpdateScheduleRequest {
  name?: string
  description?: string
  agent_id?: number
  workflow_id?: number
  channel_id?: number
  code_language?: string
  schedule_kind?: string
  cron_expr?: string
  at?: string
  every_ms?: number
  timezone?: string
  wake_mode?: string
  session_target?: string
  prompt?: string
  stagger_ms?: number
  enabled?: boolean
}

export interface ScheduleExecution {
  id: number
  schedule_id: number
  status: string
  result?: string
  error?: string
  duration_ms: number
  started_at: string
  finished_at?: string
}

export interface AgentMember {
  agent_id: number
  agent_name?: string
}

export interface ChatGroup {
  id: number
  name: string
  members: AgentMember[]
  created_by?: string
  created_at: string
  /** 最近活跃时间：后端取该群关联会话的 max(updated_at)，无会话时同 created_at */
  updated_at?: string
}

export interface CreateGroupRequest {
  name: string
  agent_ids: number[]
}

export interface UpdateGroupRequest {
  name?: string
  agent_ids?: number[]
}
