package schema

import "time"

type Agent struct {
	ID           int64     `json:"id"`
	PublicID     string    `json:"public_id"`
	Name         string    `json:"name"`
	Desc         string    `json:"description"`
	Category     string    `json:"category"`
	IsBuiltin    bool      `json:"is_builtin"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	SkillIDs     []string  `json:"skill_ids,omitempty"`
	MCPConfigIDs []int64   `json:"mcp_config_ids,omitempty"`
}

type AgentWithRuntime struct {
	Agent
	RuntimeProfile *RuntimeProfile `json:"runtime_profile,omitempty"`
	Capabilities   []Capability    `json:"capabilities,omitempty"`
	CapabilityTree *CapabilityTree `json:"capability_tree,omitempty"`
	// SkillExecutionOverrides maps builtin_skill.* and builtin_* tool names → client|server from DB (skills.execution_mode).
	// Takes precedence over SKILL.md / tool Extra when set; filled per request by ChatService/runtime; not exposed in JSON APIs.
	SkillExecutionOverrides map[string]string `json:"-"`
}

type RuntimeProfile struct {
	SourceAgent     string   `json:"source_agent"`
	Archetype       string   `json:"archetype"`
	Role            string   `json:"role,omitempty"`
	Goal            string   `json:"goal,omitempty"`
	Backstory       string   `json:"backstory,omitempty"`
	SystemPrompt    string   `json:"system_prompt,omitempty"`
	LlmModel        string   `json:"llm_model,omitempty"`
	Temperature     float64  `json:"temperature"`
	StreamEnabled   bool     `json:"stream_enabled"`
	MemoryEnabled   bool     `json:"memory_enabled"`
	SkillIDs        []string `json:"skill_ids,omitempty"`
	MCPConfigIDs    []int64  `json:"mcp_config_ids,omitempty"`
	ExecutionMode   string   `json:"execution_mode,omitempty"`
	MaxIterations   int      `json:"max_iterations,omitempty"`
	PlanPrompt      string   `json:"plan_prompt,omitempty"`
	ReflectionDepth int      `json:"reflection_depth,omitempty"`
	ApprovalMode    string   `json:"approval_mode,omitempty"`
	Approvers       []string `json:"approvers,omitempty"`
	ApprovalType    string   `json:"approval_type,omitempty"`
	ApprovalTimeout int      `json:"approval_timeout,omitempty"`
	IMEnabled       string   `json:"im_enabled,omitempty"`
	IMConfig        IMConfig `json:"im_config,omitempty"`
}

type IMConfig struct {
	WebhookURL       string `json:"webhook_url,omitempty"`
	Secret           string `json:"secret,omitempty"`
	BotName          string `json:"bot_name,omitempty"`
	AppID            string `json:"app_id,omitempty"`
	AppSecret        string `json:"app_secret,omitempty"`
	TelegramToken    string `json:"telegram_token,omitempty"`
	TelegramChatID   string `json:"telegram_chat_id,omitempty"`
	AutoReply        bool   `json:"auto_reply,omitempty"`
	NotifyOnApproval bool   `json:"notify_on_approval,omitempty"`
	// Lark event subscription: URL challenge (LARK_BOT_VERIFICATION_TOKEN) and payload decrypt (LARK_BOT_ENCRYPT_KEY).
	VerificationToken string `json:"verification_token,omitempty"`
	EncryptKey        string `json:"encrypt_key,omitempty"`
	// LarkRegion legacy: "cn" | "intl" when lark_open_domain empty (prefer lark_open_domain in UI).
	LarkRegion string `json:"lark_region,omitempty"`
	// LarkOpenDomain is the Open Platform base URL, e.g. https://open.feishu.cn or https://open.larksuite.com
	LarkOpenDomain string `json:"lark_open_domain,omitempty"`
	// WsEnabled: when true (or nil legacy), server global Start() opens WebSocket on boot; false = register credentials only, manual Start per bot.
	WsEnabled *bool `json:"ws_enabled,omitempty"`
}

// LarkRegisterLongConnection returns whether the bot should auto-start WebSocket on server global Start().
// The bot is still registered in the Lark in-memory pool when IM credentials are present; manual per-agent WS start is allowed regardless.
func (c IMConfig) LarkRegisterLongConnection() bool {
	if c.WsEnabled == nil {
		return true
	}
	return *c.WsEnabled
}

type ApprovalType string

const (
	ApprovalTypeInternal ApprovalType = "internal"
	ApprovalTypeLark     ApprovalType = "lark"
	ApprovalTypeDingTalk ApprovalType = "dingtalk"
)

type Capability struct {
	ID            int64  `json:"id"`
	Key           string `json:"key"`
	DisplayName   string `json:"display_name"`
	Description   string `json:"description"`
	SourceType    string `json:"source_type"`
	SourceRef     string `json:"source_ref"`
	ToolName      string `json:"tool_name,omitempty"`
	ExecutionMode string `json:"execution_mode"`
	IsActive      bool   `json:"is_active"`
}

type CapabilityTree struct {
	AgentID int64                `json:"agent_id"`
	Version int                  `json:"version"`
	Nodes   []CapabilityTreeNode `json:"nodes"`
}

type CapabilityTreeNode struct {
	ID           int64                `json:"id"`
	ParentID     *int64               `json:"parent_id,omitempty"`
	NodeType     string               `json:"node_type"`
	Label        string               `json:"label"`
	CapabilityID *int64               `json:"capability_id,omitempty"`
	RuleJSON     map[string]any       `json:"rule_json,omitempty"`
	SortOrder    int                  `json:"sort_order"`
	IsActive     bool                 `json:"is_active"`
	Children     []CapabilityTreeNode `json:"children,omitempty"`
}

type Skill struct {
	ID            int64     `json:"id"`
	Key           string    `json:"key"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Content       string    `json:"content"`
	SourceRef     string    `json:"source_ref"`
	Category      string    `json:"category"`
	RiskLevel     string    `json:"risk_level"`
	ExecutionMode string    `json:"execution_mode"`
	PromptHint    string    `json:"prompt_hint,omitempty"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

const (
	SkillCategorySafe       = "safe"
	SkillCategoryReadLocal  = "read_local"
	SkillCategoryReadRemote = "read_remote"
	SkillCategoryWrite      = "write"

	RiskLevelLow      = "low"
	RiskLevelMedium   = "medium"
	RiskLevelHigh     = "high"
	RiskLevelCritical = "critical"
)

type MCPConfig struct {
	ID           int64          `json:"id"`
	Key          string         `json:"key"`
	Name         string         `json:"name"`
	Transport    string         `json:"transport"`
	Endpoint     string         `json:"endpoint"`
	Config       map[string]any `json:"config,omitempty"`
	IsActive     bool           `json:"is_active"`
	HealthStatus string         `json:"health_status"`
	ToolCount    int            `json:"tool_count"`
	CreatedAt    time.Time      `json:"created_at"`
}

type Learning struct {
	ID        int64     `json:"id"`
	UserID    *int64    `json:"user_id,omitempty"` // nil = global learning
	ErrorType string    `json:"error_type"`
	Context   string    `json:"context"`
	RootCause string    `json:"root_cause"`
	Fix       string    `json:"fix"`
	Lesson    string    `json:"lesson"`
	Times     int       `json:"times"` // 重复次数
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

var DefaultSkillCategory = map[string]string{
	"builtin_skill.datetime":         SkillCategorySafe,
	"builtin_skill.regex":            SkillCategorySafe,
	"builtin_skill.json_parser":      SkillCategorySafe,
	"builtin_skill.csv_analyzer":     SkillCategorySafe,
	"builtin_skill.file_parser":      SkillCategorySafe,
	"builtin_skill.image_analyzer":   SkillCategorySafe,
	"builtin_skill.terraform_plan":   SkillCategorySafe,
	"builtin_skill.browser_client":   SkillCategoryReadRemote,
	"builtin_skill.visible_browser":  SkillCategoryWrite,
	"builtin_skill.docker_operator":  SkillCategoryReadLocal,
	"builtin_skill.git_operator":     SkillCategoryReadLocal,
	"builtin_skill.system_monitor":   SkillCategoryReadLocal,
	"builtin_skill.cron_manager":     SkillCategoryWrite,
	"builtin_skill.network_tools":    SkillCategoryReadLocal,
	"builtin_skill.cert_checker":     SkillCategoryReadLocal,
	"builtin_skill.nginx_diagnose":   SkillCategoryReadLocal,
	"builtin_skill.dns_lookup":       SkillCategoryReadLocal,
	"builtin_skill.ssh_executor":     SkillCategoryReadRemote,
	"builtin_skill.k8s_operator":     SkillCategoryReadRemote,
	"builtin_skill.db_query":         SkillCategoryReadRemote,
	"builtin_skill.http_client":      SkillCategoryReadRemote,
	"builtin_skill.prometheus_query": SkillCategoryReadRemote,
	"builtin_skill.grafana_reader":   SkillCategoryReadRemote,
	"builtin_skill.aws_readonly":     SkillCategoryReadRemote,
	"builtin_skill.gcp_readonly":     SkillCategoryReadRemote,
	"builtin_skill.azure_readonly":   SkillCategoryReadRemote,
	"builtin_skill.loki_query":       SkillCategoryReadRemote,
	"builtin_skill.argocd_readonly":  SkillCategoryReadRemote,
	"builtin_skill.alert_sender":     SkillCategoryWrite,
	"builtin_skill.slack_notify":     SkillCategoryWrite,
	"builtin_skill.jira_connector":   SkillCategoryWrite,
	"builtin_skill.github_issue":     SkillCategoryWrite,
	"builtin_skill.search":           SkillCategoryReadRemote,
	"builtin_skill.calculator":       SkillCategorySafe,
	"builtin_skill.code_interpreter": SkillCategorySafe,
	"builtin_skill.log_analyzer":     SkillCategorySafe,
}

var DefaultSkillRiskLevel = map[string]string{
	"builtin_skill.datetime":         RiskLevelLow,
	"builtin_skill.regex":            RiskLevelLow,
	"builtin_skill.json_parser":      RiskLevelLow,
	"builtin_skill.csv_analyzer":     RiskLevelLow,
	"builtin_skill.file_parser":      RiskLevelLow,
	"builtin_skill.image_analyzer":   RiskLevelLow,
	"builtin_skill.terraform_plan":   RiskLevelLow,
	"builtin_skill.browser_client":   RiskLevelLow,
	"builtin_skill.visible_browser":  RiskLevelLow,
	"builtin_skill.docker_operator":  RiskLevelLow,
	"builtin_skill.git_operator":     RiskLevelLow,
	"builtin_skill.system_monitor":   RiskLevelLow,
	"builtin_skill.cron_manager":     RiskLevelMedium,
	"builtin_skill.network_tools":    RiskLevelLow,
	"builtin_skill.cert_checker":     RiskLevelLow,
	"builtin_skill.nginx_diagnose":   RiskLevelLow,
	"builtin_skill.dns_lookup":       RiskLevelLow,
	"builtin_skill.ssh_executor":     RiskLevelLow,
	"builtin_skill.k8s_operator":     RiskLevelLow,
	"builtin_skill.db_query":         RiskLevelLow,
	"builtin_skill.http_client":      RiskLevelLow,
	"builtin_skill.prometheus_query": RiskLevelLow,
	"builtin_skill.grafana_reader":   RiskLevelLow,
	"builtin_skill.aws_readonly":     RiskLevelLow,
	"builtin_skill.gcp_readonly":     RiskLevelLow,
	"builtin_skill.azure_readonly":   RiskLevelLow,
	"builtin_skill.loki_query":       RiskLevelLow,
	"builtin_skill.argocd_readonly":  RiskLevelLow,
	"builtin_skill.alert_sender":     RiskLevelLow,
	"builtin_skill.slack_notify":     RiskLevelLow,
	"builtin_skill.jira_connector":   RiskLevelLow,
	"builtin_skill.github_issue":     RiskLevelLow,
	"builtin_skill.search":           RiskLevelLow,
	"builtin_skill.calculator":       RiskLevelLow,
	"builtin_skill.code_interpreter": RiskLevelLow,
	"builtin_skill.log_analyzer":     RiskLevelLow,
}

type MCPServer struct {
	ID          int64          `json:"id"`
	ConfigID    int64          `json:"config_id"`
	ToolName    string         `json:"tool_name"`
	DisplayName string         `json:"display_name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
	IsActive    bool           `json:"is_active"`
}
