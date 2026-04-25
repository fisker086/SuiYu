package mcp

import (
	"testing"

	"github.com/fisk086/sya/internal/schema"
)

func TestEffectiveConnectionTarget(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		cfg  *schema.MCPConfig
		want string
	}{
		{"nil", nil, ""},
		{"http endpoint", &schema.MCPConfig{Transport: "sse", Endpoint: " https://x/sse "}, "https://x/sse"},
		{"stdio explicit endpoint", &schema.MCPConfig{Transport: "stdio", Endpoint: "npx -y foo"}, "npx -y foo"},
		{
			"stdio command only",
			&schema.MCPConfig{Transport: "stdio", Config: map[string]any{"command": "npx"}},
			"'npx'",
		},
		{
			"stdio command and args",
			&schema.MCPConfig{Transport: "stdio", Config: map[string]any{
				"command": "npx",
				"args":    []any{"-y", "@scope/pkg"},
			}},
			"'npx' '-y' '@scope/pkg'",
		},
		{"stdio empty command", &schema.MCPConfig{Transport: "stdio", Config: map[string]any{"command": "  "}}, ""},
		{
			"sse url in config",
			&schema.MCPConfig{Transport: "sse", Config: map[string]any{"url": "https://mcp.example/sse"}},
			"https://mcp.example/sse",
		},
		{
			"streamable server_url",
			&schema.MCPConfig{Transport: "streamable-http", Config: map[string]any{"server_url": " https://x "}},
			"https://x",
		},
		{
			"mcpServers by key",
			&schema.MCPConfig{
				Key:       "fs",
				Transport: "stdio",
				Config: map[string]any{
					"mcpServers": map[string]any{
						"fs":    map[string]any{"command": "uvx", "args": []any{"tool"}},
						"other": map[string]any{"command": "ignored"},
					},
				},
			},
			"'uvx' 'tool'",
		},
		{
			"infer stdio when transport still sse",
			&schema.MCPConfig{
				Transport: "sse",
				Config:    map[string]any{"command": "npx", "args": []any{"-y", "pkg"}},
			},
			"'npx' '-y' 'pkg'",
		},
		{
			"mcpServers key case insensitive",
			&schema.MCPConfig{
				Key:       "MyTool",
				Transport: "stdio",
				Config: map[string]any{
					"mcpServers": map[string]any{
						"mytool": map[string]any{"command": "echo", "args": []any{"hi"}},
					},
				},
			},
			"'echo' 'hi'",
		},
		{
			"cmd and argv aliases",
			&schema.MCPConfig{
				Transport: "stdio",
				Config:    map[string]any{"cmd": "uvx", "argv": []any{"mcp"}},
			},
			"'uvx' 'mcp'",
		},
		{
			"servers alias for mcpServers",
			&schema.MCPConfig{
				Key:       "a",
				Transport: "stdio",
				Config: map[string]any{
					"servers": map[string]any{
						"a": map[string]any{"command": "true"},
					},
				},
			},
			"'true'",
		},
		{
			"one level nested command",
			&schema.MCPConfig{
				Transport: "sse",
				Config: map[string]any{
					"local": map[string]any{"command": "npx", "args": []any{"-y", "pkg"}},
				},
			},
			"'npx' '-y' 'pkg'",
		},
		{
			"args as single string",
			&schema.MCPConfig{
				Transport: "stdio",
				Config:    map[string]any{"command": "npx", "args": "-y pkg"},
			},
			"'npx' '-y' 'pkg'",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := EffectiveConnectionTarget(tc.cfg)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestResolveMCPConnection_transportInference(t *testing.T) {
	t.Parallel()
	cfg := &schema.MCPConfig{
		Transport: "sse",
		Config:    map[string]any{"command": "npx", "args": []any{"-y", "x"}},
	}
	tr, _, ok := ResolveMCPConnection(cfg)
	if !ok {
		t.Fatal("expected ok")
	}
	if tr != "stdio" {
		t.Fatalf("transport got %q want stdio", tr)
	}
}

func TestNormalizeMCPTransport(t *testing.T) {
	t.Parallel()
	if g := NormalizeMCPTransport("STDIO"); g != "stdio" {
		t.Fatalf("%q", g)
	}
	if g := NormalizeMCPTransport("Streamable-HTTP"); g != "streamable-http" {
		t.Fatalf("%q", g)
	}
	if g := NormalizeMCPTransport(""); g != "sse" {
		t.Fatalf("%q", g)
	}
}
