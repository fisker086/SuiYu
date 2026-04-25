package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
	jsonschema "github.com/eino-contrib/jsonschema"
	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/mcp"
	"github.com/fisk086/sya/internal/schema"
)

// mcpStorage is the subset of storage needed to resolve MCP tools for an agent.
type mcpStorage interface {
	GetMCPConfig(id int64) (*schema.MCPConfig, error)
	ListMCPTools(configID int64) ([]schema.MCPServer, error)
}

func mcpConfigIDsForAgent(agent *schema.AgentWithRuntime) []int64 {
	if agent == nil {
		return nil
	}
	if agent.RuntimeProfile != nil && len(agent.RuntimeProfile.MCPConfigIDs) > 0 {
		return agent.RuntimeProfile.MCPConfigIDs
	}
	return agent.MCPConfigIDs
}

func registerMCPClientFromConfig(c *mcp.Client, cfg *schema.MCPConfig) {
	if c == nil || cfg == nil {
		return
	}
	transport, target, ok := mcp.ResolveMCPConnection(cfg)
	if !ok || target == "" {
		return
	}
	c.RegisterConfig(cfg.ID, transport, target, mcp.HeadersFromConfig(cfg.Config), cfg.Config)
}

// mcpToolsForAgent builds invokable tools for the chat model from persisted MCP tool rows.
// It registers each MCP config with the client so CallTool can open a session.
func (r *Runtime) mcpToolsForAgent(agent *schema.AgentWithRuntime) ([]tool.BaseTool, error) {
	if r.mcpClient == nil || r.store == nil || agent == nil {
		return nil, nil
	}
	st, ok := r.store.(mcpStorage)
	if !ok {
		return nil, nil
	}
	ids := mcpConfigIDsForAgent(agent)
	if len(ids) == 0 {
		return nil, nil
	}

	var out []tool.BaseTool
	seen := make(map[string]struct{})

	for _, mcpID := range ids {
		cfg, err := st.GetMCPConfig(mcpID)
		if err != nil || cfg == nil {
			logger.Warn("mcp tool bind skipped: config not found", "mcp_config_id", mcpID, "err", err)
			continue
		}
		if _, _, ok := mcp.ResolveMCPConnection(cfg); !ok {
			logger.Warn("mcp tool bind skipped: missing connection target", "mcp_config_id", mcpID)
			continue
		}
		registerMCPClientFromConfig(r.mcpClient, cfg)

		tools, err := st.ListMCPTools(mcpID)
		if err != nil {
			logger.Warn("mcp ListMCPTools failed", "mcp_config_id", mcpID, "err", err)
			continue
		}
		for _, t := range tools {
			if t.ToolName == "" || !t.IsActive {
				continue
			}
			extName := uniqueMCPInvokeName(mcpID, t.ToolName, seen)
			ti, err := mcpserverToToolInfo(extName, t)
			if err != nil {
				logger.Warn("mcp tool schema skipped", "mcp_config_id", mcpID, "tool", t.ToolName, "err", err)
				continue
			}
			configID := mcpID
			toolName := t.ToolName
			out = append(out, toolutils.NewTool(ti, func(c context.Context, in map[string]any) (string, error) {
				return r.mcpClient.CallTool(c, configID, toolName, in)
			}))
		}
	}

	if len(out) > 0 {
		logger.Info("mcp tools bound for agent", "tool_count", len(out))
	}
	return out, nil
}

// mcpUsageHintsFromAgent aggregates usage_hint from each bound MCP config (same field as the MCP UI:
// mcp_configs.config.usage_hint). Appended to the chat instruction when tools are active.
func (r *Runtime) mcpUsageHintsFromAgent(agent *schema.AgentWithRuntime) string {
	if r.store == nil || agent == nil {
		return ""
	}
	st, ok := r.store.(mcpStorage)
	if !ok {
		return ""
	}
	ids := mcpConfigIDsForAgent(agent)
	if len(ids) == 0 {
		return ""
	}
	var parts []string
	for _, id := range ids {
		cfg, err := st.GetMCPConfig(id)
		if err != nil || cfg == nil {
			continue
		}
		hint := usageHintFromMCPConfig(cfg.Config)
		if strings.TrimSpace(hint) == "" {
			continue
		}
		title := strings.TrimSpace(cfg.Name)
		if title == "" {
			title = fmt.Sprintf("MCP #%d", id)
		}
		parts = append(parts, fmt.Sprintf("### %s (config id=%d)\n\n%s", title, id, strings.TrimSpace(hint)))
	}
	if len(parts) == 0 {
		return ""
	}
	return "## MCP usage (from MCP configuration)\n\n" + strings.Join(parts, "\n\n")
}

func usageHintFromMCPConfig(cfg map[string]any) string {
	if cfg == nil {
		return ""
	}
	v, ok := cfg["usage_hint"]
	if !ok || v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		return fmt.Sprint(s)
	}
}

func uniqueMCPInvokeName(configID int64, toolName string, seen map[string]struct{}) string {
	base := fmt.Sprintf("mcp_%d_%s", configID, sanitizeToolNameForModel(toolName))
	candidate := base
	n := 0
	for {
		if _, ok := seen[candidate]; !ok {
			seen[candidate] = struct{}{}
			return candidate
		}
		n++
		candidate = fmt.Sprintf("%s_%d", base, n)
	}
}

func sanitizeToolNameForModel(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return "tool"
	}
	if len(out) > 80 {
		out = out[:80]
	}
	return out
}

func mcpserverToToolInfo(invokeName string, t schema.MCPServer) (*einoschema.ToolInfo, error) {
	params, err := mcpserverInputSchemaToParamsOneOf(t.InputSchema)
	if err != nil {
		return nil, err
	}
	desc := strings.TrimSpace(t.Description)
	if desc == "" {
		desc = t.DisplayName
	}
	if desc == "" {
		desc = "MCP tool " + t.ToolName
	}
	return &einoschema.ToolInfo{
		Name:        invokeName,
		Desc:        desc,
		ParamsOneOf: params,
	}, nil
}

func mcpserverInputSchemaToParamsOneOf(raw map[string]any) (*einoschema.ParamsOneOf, error) {
	if len(raw) == 0 {
		return einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
			"input": {
				Type:     einoschema.Object,
				Desc:     "Tool arguments as a JSON object matching the MCP tool schema.",
				Required: false,
			},
		}), nil
	}
	sanitized := sanitizeJSONSchema(raw)
	data, err := json.Marshal(sanitized)
	if err != nil {
		return nil, err
	}
	var js jsonschema.Schema
	if err := json.Unmarshal(data, &js); err != nil {
		return nil, fmt.Errorf("parse input_schema: %w", err)
	}
	return einoschema.NewParamsOneOfByJSONSchema(&js), nil
}

func sanitizeJSONSchema(v any) any {
	switch val := v.(type) {
	case bool:
		return nil
	case map[string]any:
		result := make(map[string]any)
		for key, value := range val {
			if key == "default" {
				if _, isBool := value.(bool); isBool {
					continue
				}
			}
			sanitized := sanitizeJSONSchema(value)
			if sanitized != nil {
				result[key] = sanitized
			}
		}
		if len(result) == 0 {
			return nil
		}
		if result["type"] == "array" && result["items"] == nil {
			result["items"] = map[string]any{"type": "string"}
		}
		return result
	case []any:
		result := make([]any, 0, len(val))
		for _, item := range val {
			sanitizedItem := sanitizeJSONSchema(item)
			if sanitizedItem != nil {
				result = append(result, sanitizedItem)
			}
		}
		return result
	default:
		return v
	}
}
