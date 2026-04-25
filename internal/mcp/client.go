package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"sync"

	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/schema"
	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// headerRoundTripper adds fixed headers to every request (Authorization, etc.).
type headerRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	for k, v := range t.headers {
		if strings.TrimSpace(k) == "" || v == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	return base.RoundTrip(req)
}

// newStreamableTransport configures StreamableClientTransport. By default DisableStandaloneSSE is true:
// many Cloudflare/proxy setups drop idle GET SSE streams and the SDK fails with
// "exceeded N retries without progress". Opt in with config mcp_enable_standalone_sse: true.
func newStreamableTransport(endpoint string, hc *http.Client, cfg map[string]any) mcp.Transport {
	t := &mcp.StreamableClientTransport{
		Endpoint:             endpoint,
		HTTPClient:           hc,
		DisableStandaloneSSE: true,
		MaxRetries:           20,
	}
	if cfg == nil {
		return t
	}
	if v, ok := cfg["mcp_enable_standalone_sse"]; ok {
		if b, ok := v.(bool); ok {
			t.DisableStandaloneSSE = !b
		}
	}
	if v, ok := cfg["mcp_streamable_max_retries"]; ok {
		switch n := v.(type) {
		case float64:
			t.MaxRetries = int(n)
		case int:
			t.MaxRetries = n
		case int64:
			t.MaxRetries = int(n)
		}
	}
	return t
}

func httpClientWithHeaders(headers map[string]string) *http.Client {
	if len(headers) == 0 {
		return nil
	}
	return &http.Client{
		Transport: &headerRoundTripper{
			base:    http.DefaultTransport,
			headers: headers,
		},
	}
}

type Client struct {
	mu      sync.RWMutex
	config  map[int64]*mcpConfig
	session map[int64]*mcp.ClientSession
}

type mcpConfig struct {
	transport string
	endpoint  string
	headers   map[string]string
	// config is the raw MCP config JSON (same as DB); used for streamable transport tuning.
	config map[string]any
}

func NewClient() *Client {
	return &Client{
		config:  make(map[int64]*mcpConfig),
		session: make(map[int64]*mcp.ClientSession),
	}
}

func (c *Client) RegisterConfig(id int64, transport, endpoint string, headers map[string]string, config map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.config[id] = &mcpConfig{
		transport: transport,
		endpoint:  endpoint,
		headers:   headers,
		config:    config,
	}
	// Drop cached session so the next DiscoverTools / CallTool reconnects with new endpoint/headers.
	if session, ok := c.session[id]; ok {
		session.Close()
		delete(c.session, id)
	}
}

func (c *Client) UnregisterConfig(id int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.config, id)
	if session, ok := c.session[id]; ok {
		session.Close()
		delete(c.session, id)
	}
}

func (c *Client) DiscoverTools(ctx context.Context, configID int64) ([]schema.MCPServer, error) {
	c.mu.RLock()
	cfg, ok := c.config[configID]
	c.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("mcp config not found: %d", configID)
	}

	session, err := c.getOrCreateSession(ctx, configID, cfg)
	if err != nil {
		return nil, err
	}

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	tools := make([]schema.MCPServer, 0, len(result.Tools))
	for _, tool := range result.Tools {
		displayName := tool.Name
		if tool.Title != "" {
			displayName = tool.Title
		}
		inputSchema := toMapStringAny(tool.InputSchema)
		tools = append(tools, schema.MCPServer{
			ConfigID:    configID,
			ToolName:    tool.Name,
			DisplayName: displayName,
			Description: tool.Description,
			InputSchema: inputSchema,
			IsActive:    true,
		})
	}

	return tools, nil
}

func (c *Client) CallTool(ctx context.Context, configID int64, toolName string, args map[string]any) (string, error) {
	logger.Info("mcp CallTool invoked",
		"mcp_config_id", configID,
		"tool_name", toolName,
		"arg_keys", mapKeysForLog(args),
	)
	c.mu.RLock()
	cfg, ok := c.config[configID]
	c.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("mcp config not found: %d", configID)
	}

	session, err := c.getOrCreateSession(ctx, configID, cfg)
	if err != nil {
		logger.Warn("mcp CallTool session failed", "mcp_config_id", configID, "tool_name", toolName, "err", err)
		return "", err
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		logger.Warn("mcp CallTool remote failed", "mcp_config_id", configID, "tool_name", toolName, "err", err)
		return "", fmt.Errorf("failed to call tool %s: %w", toolName, err)
	}
	out := formatCallToolResult(result)
	logger.Info("mcp CallTool finished", "mcp_config_id", configID, "tool_name", toolName, "result_len", len(out))
	return out, nil
}

func mapKeysForLog(m map[string]any) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (c *Client) getOrCreateSession(ctx context.Context, configID int64, cfg *mcpConfig) (*mcp.ClientSession, error) {
	c.mu.RLock()
	if session, ok := c.session[configID]; ok {
		c.mu.RUnlock()
		return session, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if session, ok := c.session[configID]; ok {
		return session, nil
	}

	hc := httpClientWithHeaders(cfg.headers)

	var transport mcp.Transport
	switch NormalizeMCPTransport(cfg.transport) {
	case "sse":
		transport = &mcp.SSEClientTransport{Endpoint: cfg.endpoint, HTTPClient: hc}
	case "streamable-http":
		transport = newStreamableTransport(cfg.endpoint, hc, cfg.config)
	case "stdio":
		cmd := exec.Command("sh", "-c", cfg.endpoint)
		transport = &mcp.CommandTransport{Command: cmd}
	default:
		transport = &mcp.SSEClientTransport{Endpoint: cfg.endpoint, HTTPClient: hc}
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "eino-agent", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	c.session[configID] = session
	return session, nil
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for id, session := range c.session {
		session.Close()
		delete(c.session, id)
	}
}

func formatCallToolResult(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}
	var parts []string
	for _, content := range result.Content {
		switch x := content.(type) {
		case *mcp.TextContent:
			parts = append(parts, x.Text)
		default:
			if x != nil {
				parts = append(parts, fmt.Sprintf("%v", x))
			}
		}
	}
	if result.StructuredContent != nil {
		parts = append(parts, fmt.Sprintf("%v", result.StructuredContent))
	}
	out := strings.Join(parts, "\n")
	if result.IsError {
		return "tool error: " + out
	}
	return out
}

func toMapStringAny(v any) map[string]any {
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}
