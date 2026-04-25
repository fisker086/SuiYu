package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/fisk086/sya/internal/schema"
	"github.com/jackc/pgx/v5"
)

func (s *PostgresStorage) ListMCPConfigs() ([]*schema.MCPConfig, error) {
	rows, err := s.pool.Query(context.Background(), `SELECT id, key, name, transport, endpoint, config, is_active, health_status, tool_count, created_at FROM mcp_configs ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*schema.MCPConfig
	for rows.Next() {
		var cfg schema.MCPConfig
		var configJSON []byte
		if err := rows.Scan(&cfg.ID, &cfg.Key, &cfg.Name, &cfg.Transport, &cfg.Endpoint, &configJSON, &cfg.IsActive, &cfg.HealthStatus, &cfg.ToolCount, &cfg.CreatedAt); err != nil {
			return nil, err
		}
		if configJSON != nil {
			if err := json.Unmarshal(configJSON, &cfg.Config); err != nil {
				cfg.Config = nil
			}
		}
		configs = append(configs, &cfg)
	}
	return configs, nil
}

func (s *PostgresStorage) GetMCPConfig(id int64) (*schema.MCPConfig, error) {
	var cfg schema.MCPConfig
	var configJSON []byte
	err := s.pool.QueryRow(context.Background(),
		`SELECT id, key, name, transport, endpoint, config, is_active, health_status, tool_count, created_at FROM mcp_configs WHERE id = $1`, id).
		Scan(&cfg.ID, &cfg.Key, &cfg.Name, &cfg.Transport, &cfg.Endpoint, &configJSON, &cfg.IsActive, &cfg.HealthStatus, &cfg.ToolCount, &cfg.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMCPConfigNotFound
		}
		return nil, err
	}
	if configJSON != nil {
		if err := json.Unmarshal(configJSON, &cfg.Config); err != nil {
			cfg.Config = nil
		}
	}
	return &cfg, nil
}

func (s *PostgresStorage) ListMCPTools(configID int64) ([]schema.MCPServer, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT id, config_id, tool_name, COALESCE(display_name, ''), COALESCE(description, ''), input_schema, is_active FROM mcp_servers WHERE config_id = $1 ORDER BY tool_name`,
		configID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []schema.MCPServer
	for rows.Next() {
		var t schema.MCPServer
		var inputJSON []byte
		if err := rows.Scan(&t.ID, &t.ConfigID, &t.ToolName, &t.DisplayName, &t.Description, &inputJSON, &t.IsActive); err != nil {
			return nil, err
		}
		if len(inputJSON) > 0 {
			if err := json.Unmarshal(inputJSON, &t.InputSchema); err != nil {
				t.InputSchema = nil
			}
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []schema.MCPServer{}
	}
	return out, nil
}

func (s *PostgresStorage) CreateMCPConfig(req *schema.CreateMCPConfigRequest) (*schema.MCPConfig, error) {
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	var id int64
	var now time.Time
	err = s.pool.QueryRow(context.Background(),
		`INSERT INTO mcp_configs (key, name, transport, endpoint, config) VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`,
		req.Key, req.Name, req.Transport, req.Endpoint, configJSON).Scan(&id, &now)
	if err != nil {
		return nil, err
	}
	return &schema.MCPConfig{ID: id, Key: req.Key, Name: req.Name, Transport: req.Transport, Endpoint: req.Endpoint, Config: req.Config, IsActive: true, HealthStatus: "unknown", CreatedAt: now}, nil
}

func (s *PostgresStorage) UpdateMCPConfig(id int64, req *schema.CreateMCPConfigRequest) (*schema.MCPConfig, error) {
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	_, err = s.pool.Exec(context.Background(),
		`UPDATE mcp_configs SET key = COALESCE(NULLIF($1, ''), key), name = COALESCE(NULLIF($2, ''), name), transport = COALESCE(NULLIF($3, ''), transport), endpoint = COALESCE(NULLIF($4, ''), endpoint), config = COALESCE($5, config) WHERE id = $6`,
		req.Key, req.Name, req.Transport, req.Endpoint, configJSON, id)
	if err != nil {
		return nil, err
	}
	return s.GetMCPConfig(id)
}

func (s *PostgresStorage) DeleteMCPConfig(id int64) error {
	_, err := s.pool.Exec(context.Background(), `DELETE FROM mcp_configs WHERE id = $1`, id)
	return err
}

func (s *PostgresStorage) SyncMCPServer(id int64, req *schema.SyncMCPServerRequest) error {
	tx, err := s.pool.Begin(context.Background())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())

	_, _ = tx.Exec(context.Background(), `DELETE FROM mcp_servers WHERE config_id = $1`, id)

	for _, tool := range req.Tools {
		inputSchema, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return fmt.Errorf("failed to marshal input schema for tool %s: %w", tool.ToolName, err)
		}
		_, err = tx.Exec(context.Background(),
			`INSERT INTO mcp_servers (config_id, tool_name, display_name, description, input_schema, is_active) VALUES ($1, $2, $3, $4, $5, $6)`,
			id, tool.ToolName, tool.DisplayName, tool.Description, inputSchema, tool.IsActive)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(context.Background(), `UPDATE mcp_configs SET tool_count = $1, health_status = 'ready' WHERE id = $2`, len(req.Tools), id)
	if err != nil {
		return err
	}

	return tx.Commit(context.Background())
}
