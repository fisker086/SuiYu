package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/fisk086/sya/internal/mcp"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/storage"
)

// ErrMCPDiscoveryNeedsTarget is returned when sync is asked to discover tools remotely
// but the config has no URL (SSE/HTTP) or stdio command.
var ErrMCPDiscoveryNeedsTarget = errors.New("cannot discover tools: set top-level endpoint, or auth JSON with url/server_url, or command (or cmd) with optional args/argv / mcpServers|servers.<config key>")

type MCPService struct {
	store storage.Storage
	mcp   *mcp.Client
}

func NewMCPService(store storage.Storage, mcpClient *mcp.Client) *MCPService {
	return &MCPService{store: store, mcp: mcpClient}
}

func (s *MCPService) ListConfigs() ([]*schema.MCPConfig, error) {
	return s.store.ListMCPConfigs()
}

func (s *MCPService) GetConfig(id int64) (*schema.MCPConfig, error) {
	return s.store.GetMCPConfig(id)
}

// ListTools returns tools persisted for this MCP config (after sync).
func (s *MCPService) ListTools(configID int64) ([]schema.MCPServer, error) {
	if _, err := s.store.GetMCPConfig(configID); err != nil {
		return nil, err
	}
	return s.store.ListMCPTools(configID)
}

func (s *MCPService) CreateConfig(req *schema.CreateMCPConfigRequest) (*schema.MCPConfig, error) {
	cfg, err := s.store.CreateMCPConfig(req)
	if err != nil {
		return nil, err
	}
	s.registerMCPClient(cfg)
	return cfg, nil
}

func (s *MCPService) UpdateConfig(id int64, req *schema.CreateMCPConfigRequest) (*schema.MCPConfig, error) {
	cfg, err := s.store.UpdateMCPConfig(id, req)
	if err != nil {
		return nil, err
	}
	s.registerMCPClient(cfg)
	return cfg, nil
}

func (s *MCPService) DeleteConfig(id int64) error {
	if s.mcp != nil {
		s.mcp.UnregisterConfig(id)
	}
	return s.store.DeleteMCPConfig(id)
}

func (s *MCPService) registerMCPClient(cfg *schema.MCPConfig) {
	if s.mcp == nil || cfg == nil {
		return
	}
	transport, target, ok := mcp.ResolveMCPConnection(cfg)
	if !ok || target == "" {
		return
	}
	s.mcp.RegisterConfig(cfg.ID, transport, target, mcp.HeadersFromConfig(cfg.Config), cfg.Config)
}

func (s *MCPService) SyncServer(ctx context.Context, id int64, req *schema.SyncMCPServerRequest) error {
	tools := req.Tools
	if len(tools) == 0 {
		if s.mcp == nil {
			return fmt.Errorf("mcp client not configured: provide tools in request body or enable server MCP client")
		}
		cfg, err := s.store.GetMCPConfig(id)
		if err != nil {
			return err
		}
		transport, target, ok := mcp.ResolveMCPConnection(cfg)
		if !ok || target == "" {
			return ErrMCPDiscoveryNeedsTarget
		}
		s.mcp.RegisterConfig(id, transport, target, mcp.HeadersFromConfig(cfg.Config), cfg.Config)
		discovered, err := s.mcp.DiscoverTools(ctx, id)
		if err != nil {
			return fmt.Errorf("discover tools: %w", err)
		}
		tools = discovered
	}
	syncReq := &schema.SyncMCPServerRequest{
		Tools:            tools,
		CreateCapability: req.CreateCapability,
	}
	return s.store.SyncMCPServer(id, syncReq)
}
