package storage

import (
	"context"
	"fmt"

	"github.com/fisk086/sya/internal/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStorage struct {
	pool      *pgxpool.Pool
	dimension int
}

func NewPostgresStorage(ctx context.Context, dsn string, dimension int) (*PostgresStorage, error) {
	logger.Info("postgres", "step", "pool_open")
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}

	logger.Info("postgres", "step", "ping")
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	return &PostgresStorage{pool: pool, dimension: dimension}, nil
}

func (s *PostgresStorage) Close() {
	s.pool.Close()
}

func (s *PostgresStorage) Migrate(ctx context.Context) error {
	vecDim := s.dimension
	const hnswM = 16
	const hnswEfConstruction = 64
	migrations := []string{
		`CREATE EXTENSION IF NOT EXISTS vector`,
		`CREATE TABLE IF NOT EXISTS users (
			id BIGSERIAL PRIMARY KEY,
			username VARCHAR(50) NOT NULL,
			email VARCHAR(255) NOT NULL,
			hashed_password VARCHAR(255) NOT NULL,
			full_name VARCHAR(100),
			avatar_url VARCHAR(512),
			status VARCHAR(20) DEFAULT 'active',
			is_superuser BOOLEAN DEFAULT false,
			is_admin BOOLEAN DEFAULT false,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ,
			last_login TIMESTAMPTZ,
			CONSTRAINT uni_users_username UNIQUE (username),
			CONSTRAINT uni_users_email UNIQUE (email)
		)`,
		`DO $$
BEGIN
	IF EXISTS (
		SELECT 1 FROM pg_constraint c
		JOIN pg_class t ON c.conrelid = t.oid
		JOIN pg_namespace n ON t.relnamespace = n.oid
		WHERE n.nspname = 'public' AND t.relname = 'users' AND c.conname = 'users_username_key'
	) THEN
		ALTER TABLE users RENAME CONSTRAINT users_username_key TO uni_users_username;
	END IF;
	IF EXISTS (
		SELECT 1 FROM pg_constraint c
		JOIN pg_class t ON c.conrelid = t.oid
		JOIN pg_namespace n ON t.relnamespace = n.oid
		WHERE n.nspname = 'public' AND t.relname = 'users' AND c.conname = 'users_email_key'
	) THEN
		ALTER TABLE users RENAME CONSTRAINT users_email_key TO uni_users_email;
	END IF;
END $$;`,
		`CREATE TABLE IF NOT EXISTS agents (
			id BIGSERIAL PRIMARY KEY,
			public_id UUID NOT NULL DEFAULT gen_random_uuid(),
			name VARCHAR(255) NOT NULL,
			description TEXT,
			category VARCHAR(100),
			is_builtin BOOLEAN DEFAULT false,
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_public_id ON agents(public_id)`,
		`CREATE TABLE IF NOT EXISTS agent_runtime (
			id BIGSERIAL PRIMARY KEY,
			agent_id BIGINT REFERENCES agents(id) ON DELETE CASCADE,
			source_agent VARCHAR(255),
			archetype VARCHAR(100),
			role TEXT,
			goal TEXT,
			backstory TEXT,
			system_prompt TEXT,
			llm_model VARCHAR(100),
			temperature FLOAT DEFAULT 0.7,
			stream_enabled BOOLEAN DEFAULT true,
			memory_enabled BOOLEAN DEFAULT false,
			skill_ids TEXT[] DEFAULT '{}',
			mcp_config_ids BIGINT[] DEFAULT '{}',
			execution_mode VARCHAR(64) DEFAULT '',
			max_iterations INT DEFAULT 16,
			plan_prompt TEXT DEFAULT '',
			reflection_depth INT DEFAULT 0,
			approval_mode VARCHAR(30) DEFAULT 'auto',
			approvers TEXT[] DEFAULT '{}',
			im_enabled VARCHAR(30) DEFAULT 'disabled',
			im_config JSONB DEFAULT '{}',
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS capabilities (
			id BIGSERIAL PRIMARY KEY,
			key VARCHAR(255) NOT NULL UNIQUE,
			display_name VARCHAR(255),
			description TEXT,
			source_type VARCHAR(100),
			source_ref VARCHAR(255),
			tool_name VARCHAR(255),
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS capability_tree_nodes (
			id BIGSERIAL PRIMARY KEY,
			agent_id BIGINT REFERENCES agents(id) ON DELETE CASCADE,
			parent_id BIGINT REFERENCES capability_tree_nodes(id),
			node_type VARCHAR(100),
			label VARCHAR(255),
			capability_id BIGINT REFERENCES capabilities(id),
			rule_json JSONB,
			sort_order INT DEFAULT 0,
			is_active BOOLEAN DEFAULT true,
			version INT DEFAULT 1,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS skills (
			id BIGSERIAL PRIMARY KEY,
			key VARCHAR(255) NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			content TEXT,
			source_ref VARCHAR(255),
			is_active BOOLEAN DEFAULT true,
			risk_level VARCHAR(32) DEFAULT 'low',
			execution_mode VARCHAR(64) DEFAULT '',
			prompt_hint TEXT DEFAULT '',
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS mcp_configs (
			id BIGSERIAL PRIMARY KEY,
			key VARCHAR(255) NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			transport VARCHAR(100),
			endpoint TEXT,
			config JSONB,
			is_active BOOLEAN DEFAULT true,
			health_status VARCHAR(50) DEFAULT 'unknown',
			tool_count INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS learnings (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT,
			error_type VARCHAR(255) NOT NULL,
			context TEXT,
			root_cause TEXT,
			fix TEXT,
			lesson TEXT,
			times INT DEFAULT 1,
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			created_at TIMESTAMP DEFAULT NOW(),
			UNIQUE(user_id, error_type)
		)`,
		`CREATE TABLE IF NOT EXISTS mcp_servers (
			id BIGSERIAL PRIMARY KEY,
			config_id BIGINT REFERENCES mcp_configs(id) ON DELETE CASCADE,
			tool_name VARCHAR(255) NOT NULL,
			display_name VARCHAR(255),
			description TEXT,
			input_schema JSONB,
			is_active BOOLEAN DEFAULT true
		)`,
	}

	migrations = append(migrations,
		`CREATE TABLE IF NOT EXISTS chat_sessions (
			id VARCHAR(36) PRIMARY KEY,
			agent_id BIGINT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
			user_id VARCHAR(255),
			title VARCHAR(512) NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_sessions_agent ON chat_sessions(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_sessions_agent_user ON chat_sessions(agent_id, user_id)`,
		`CREATE TABLE IF NOT EXISTS chat_groups (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			created_by VARCHAR(255),
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS chat_group_members (
			id BIGSERIAL PRIMARY KEY,
			group_id BIGINT NOT NULL REFERENCES chat_groups(id) ON DELETE CASCADE,
			agent_id BIGINT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE(group_id, agent_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_group_members_group ON chat_group_members(group_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_group_members_agent ON chat_group_members(agent_id)`,
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS agent_memory (
			id BIGSERIAL PRIMARY KEY,
			agent_id BIGINT REFERENCES agents(id) ON DELETE CASCADE,
			user_id VARCHAR(255),
			session_id VARCHAR(255),
			role VARCHAR(50),
			content TEXT,
			extra JSONB DEFAULT '{}'::jsonb,
			embedding vector(%d),
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`, vecDim),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS semantic_memory (
			id BIGSERIAL PRIMARY KEY,
			agent_id BIGINT REFERENCES agents(id) ON DELETE CASCADE,
			user_id VARCHAR(255),
			content TEXT,
			metadata JSONB,
			embedding vector(%d),
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`, vecDim),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_agent_memory_embedding ON agent_memory USING hnsw (embedding vector_cosine_ops) WITH (m = %d, ef_construction = %d)`, hnswM, hnswEfConstruction),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_semantic_memory_embedding ON semantic_memory USING hnsw (embedding vector_cosine_ops) WITH (m = %d, ef_construction = %d)`, hnswM, hnswEfConstruction),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS user_profile (
			id BIGSERIAL PRIMARY KEY,
			user_id VARCHAR(255) NOT NULL,
			agent_id BIGINT REFERENCES agents(id) ON DELETE CASCADE,
			profile JSONB DEFAULT '{}',
			embedding vector(%d),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE(user_id, agent_id)
		)`, vecDim),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_user_profile_user_agent ON user_profile(user_id, agent_id)`),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_user_profile_embedding ON user_profile USING hnsw (embedding vector_cosine_ops) WITH (m = %d, ef_construction = %d)`, hnswM, hnswEfConstruction),
		`CREATE INDEX IF NOT EXISTS idx_capability_tree_agent ON capability_tree_nodes(agent_id, version)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_memory_session ON agent_memory(session_id)`,
		`CREATE TABLE IF NOT EXISTS agent_workflows (
			id BIGSERIAL PRIMARY KEY,
			key VARCHAR(100) NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			kind VARCHAR(32) NOT NULL DEFAULT 'sequential',
			step_agent_ids BIGINT[] NOT NULL DEFAULT '{}',
			config JSONB DEFAULT '{}',
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_workflows_active ON agent_workflows(is_active)`,
		`CREATE TABLE IF NOT EXISTS workflow_definitions (
			id BIGSERIAL PRIMARY KEY,
			key VARCHAR(100) NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			kind VARCHAR(32) NOT NULL DEFAULT 'graph',
			nodes JSONB NOT NULL DEFAULT '[]',
			edges JSONB NOT NULL DEFAULT '[]',
			variables JSONB DEFAULT '{}',
			input_schema JSONB,
			output_schema JSONB,
			version INT NOT NULL DEFAULT 1,
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_workflow_definitions_active ON workflow_definitions(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_workflow_definitions_key ON workflow_definitions(key)`,
		`CREATE TABLE IF NOT EXISTS workflow_executions (
			id BIGSERIAL PRIMARY KEY,
			workflow_id BIGINT NOT NULL REFERENCES workflow_definitions(id) ON DELETE CASCADE,
			workflow_key VARCHAR(100),
			status VARCHAR(20) NOT NULL DEFAULT 'running',
			input TEXT,
			output TEXT,
			error TEXT,
			node_results JSONB DEFAULT '[]',
			variables JSONB DEFAULT '{}',
			duration_ms BIGINT DEFAULT 0,
			started_at TIMESTAMPTZ DEFAULT NOW(),
			finished_at TIMESTAMPTZ,
			created_by VARCHAR(100)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_workflow_executions_workflow ON workflow_executions(workflow_id)`,
		`CREATE INDEX IF NOT EXISTS idx_workflow_executions_status ON workflow_executions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_workflow_executions_started ON workflow_executions(started_at DESC)`,
		`CREATE TABLE IF NOT EXISTS channels (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			kind VARCHAR(32) NOT NULL,
			webhook_url TEXT,
			app_id VARCHAR(512),
			app_secret TEXT,
			extra JSONB DEFAULT '{}',
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_channels_kind ON channels(kind)`,
		`CREATE TABLE IF NOT EXISTS rbac_roles (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL UNIQUE,
			description VARCHAR(500),
			is_system BOOLEAN DEFAULT FALSE,
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rbac_roles_name ON rbac_roles(name)`,
		`CREATE TABLE IF NOT EXISTS rbac_role_agent_permissions (
			role_id BIGINT REFERENCES rbac_roles(id) ON DELETE CASCADE,
			agent_id BIGINT NOT NULL,
			PRIMARY KEY (role_id, agent_id)
		)`,
		`CREATE TABLE IF NOT EXISTS user_roles (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
			role_id BIGINT REFERENCES rbac_roles(id) ON DELETE CASCADE,
			is_active BOOLEAN DEFAULT TRUE,
			expires_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_roles_user ON user_roles(user_id)`,
		`CREATE TABLE IF NOT EXISTS message_channels (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			agent_id BIGINT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
			kind VARCHAR(32) NOT NULL DEFAULT 'direct',
			description TEXT,
			is_public BOOLEAN DEFAULT false,
			metadata JSONB DEFAULT '{}',
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_message_channels_agent ON message_channels(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_message_channels_kind ON message_channels(kind)`,
		`CREATE TABLE IF NOT EXISTS agent_messages (
			id BIGSERIAL PRIMARY KEY,
			from_agent_id BIGINT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
			to_agent_id BIGINT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
			channel_id BIGINT NOT NULL REFERENCES message_channels(id) ON DELETE CASCADE,
			session_id VARCHAR(255),
			kind VARCHAR(32) NOT NULL DEFAULT 'text',
			content TEXT NOT NULL,
			metadata JSONB DEFAULT '{}',
			status VARCHAR(32) NOT NULL DEFAULT 'pending',
			priority INT DEFAULT 0,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			delivered_at TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_messages_channel ON agent_messages(channel_id)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_messages_from ON agent_messages(from_agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_messages_to ON agent_messages(to_agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_messages_status ON agent_messages(status)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_messages_session ON agent_messages(session_id)`,
		`CREATE TABLE IF NOT EXISTS a2a_cards (
			id BIGSERIAL PRIMARY KEY,
			agent_id BIGINT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			url TEXT,
			version VARCHAR(50) DEFAULT '1.0.0',
			capabilities TEXT[] DEFAULT '{}',
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_a2a_cards_agent ON a2a_cards(agent_id)`,
		`CREATE TABLE IF NOT EXISTS schedules (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			agent_id BIGINT REFERENCES agents(id) ON DELETE CASCADE,
			workflow_id BIGINT REFERENCES workflow_definitions(id) ON DELETE CASCADE,
			channel_id BIGINT REFERENCES channels(id) ON DELETE SET NULL,
			schedule_kind VARCHAR(50) NOT NULL,
			cron_expr VARCHAR(255),
			at_time VARCHAR(50),
			every_ms BIGINT,
			timezone VARCHAR(100),
			wake_mode VARCHAR(50) DEFAULT 'now',
			session_target VARCHAR(50) NOT NULL DEFAULT 'new',
			prompt TEXT,
			stagger_ms BIGINT DEFAULT 0,
			enabled BOOLEAN DEFAULT true,
			owner_user_id VARCHAR(64),
			chat_session_id VARCHAR(36),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_schedules_agent ON schedules(agent_id)`,
		`CREATE TABLE IF NOT EXISTS schedule_executions (
			id BIGSERIAL PRIMARY KEY,
			schedule_id BIGINT NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
			status VARCHAR(50) NOT NULL,
			result TEXT,
			error TEXT,
			duration_ms BIGINT,
			started_at TIMESTAMPTZ NOT NULL,
			finished_at TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_schedule_executions_schedule ON schedule_executions(schedule_id)`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id BIGSERIAL PRIMARY KEY,
			user_id VARCHAR(100),
			agent_id BIGINT NOT NULL,
			session_id VARCHAR(100),
			tool_name VARCHAR(100) NOT NULL,
			action VARCHAR(50) NOT NULL,
			risk_level VARCHAR(20) NOT NULL,
			input TEXT,
			output TEXT,
			error TEXT,
			status VARCHAR(20) NOT NULL,
			duration_ms BIGINT,
			ip_address VARCHAR(50),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_agent ON audit_logs(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_user ON audit_logs(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_session ON audit_logs(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_tool ON audit_logs(tool_name)`,
		`CREATE TABLE IF NOT EXISTS approval_requests (
			id BIGSERIAL PRIMARY KEY,
			agent_id BIGINT NOT NULL,
			session_id VARCHAR(100),
			user_id VARCHAR(100),
			tool_name VARCHAR(100) NOT NULL,
			risk_level VARCHAR(20) NOT NULL,
			input TEXT,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			approver_id VARCHAR(100),
			comment TEXT,
			approved_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			approval_type VARCHAR(20) DEFAULT 'internal',
			external_id VARCHAR(100),
			expires_at TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_approval_requests_agent ON approval_requests(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_approval_requests_status ON approval_requests(status)`,
		`CREATE INDEX IF NOT EXISTS idx_approval_requests_created ON approval_requests(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_approval_requests_user ON approval_requests(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_approval_requests_external ON approval_requests(external_id)`,
		`CREATE TABLE IF NOT EXISTS token_usage (
			id BIGSERIAL PRIMARY KEY,
			user_id VARCHAR(100),
			user_name VARCHAR(100),
			agent_id BIGINT,
			agent_name VARCHAR(200),
			model VARCHAR(100),
			prompt_tokens BIGINT DEFAULT 0,
			completion BIGINT DEFAULT 0,
			total_tokens BIGINT DEFAULT 0,
			cost DOUBLE PRECISION DEFAULT 0,
			date VARCHAR(10),
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_token_usage_user_date ON token_usage(user_id, date)`,
		`CREATE INDEX IF NOT EXISTS idx_token_usage_agent_date ON token_usage(agent_id, date)`,
		`CREATE INDEX IF NOT EXISTS idx_token_usage_model ON token_usage(model)`,
		`CREATE INDEX IF NOT EXISTS idx_token_usage_date ON token_usage(date)`,
		// Legacy bug: runtime created approvals without CreatedAt → Go zero time stored as year 0001.
		`UPDATE approval_requests SET created_at = NOW() WHERE created_at < '1970-01-01'::timestamptz`,
		`DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'agent_memory' AND column_name = 'created_at'
      AND data_type = 'timestamp without time zone'
  ) THEN
    ALTER TABLE agent_memory ALTER COLUMN created_at TYPE TIMESTAMPTZ
      USING created_at AT TIME ZONE 'Asia/Shanghai';
  END IF;
END $$;`,
		`DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'semantic_memory' AND column_name = 'created_at'
      AND data_type = 'timestamp without time zone'
  ) THEN
    ALTER TABLE semantic_memory ALTER COLUMN created_at TYPE TIMESTAMPTZ
      USING created_at AT TIME ZONE 'Asia/Shanghai';
  END IF;
END $$;`,
		`ALTER TABLE chat_sessions ADD COLUMN IF NOT EXISTS group_id BIGINT REFERENCES chat_groups(id) ON DELETE SET NULL`,
		`CREATE INDEX IF NOT EXISTS idx_chat_sessions_group ON chat_sessions(group_id)`,
		`ALTER TABLE schedules ADD COLUMN IF NOT EXISTS code_language VARCHAR(20)`,
		// Legacy DBs may have skills without content/risk columns (CREATE TABLE IF NOT EXISTS does not alter).
		`ALTER TABLE skills ADD COLUMN IF NOT EXISTS content TEXT`,
		`ALTER TABLE skills ADD COLUMN IF NOT EXISTS source_ref VARCHAR(255)`,
		`ALTER TABLE skills ADD COLUMN IF NOT EXISTS risk_level VARCHAR(32) DEFAULT 'low'`,
		`ALTER TABLE skills ADD COLUMN IF NOT EXISTS execution_mode VARCHAR(64) DEFAULT ''`,
		`ALTER TABLE skills ADD COLUMN IF NOT EXISTS prompt_hint TEXT DEFAULT ''`,
		`ALTER TABLE skills ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT true`,
		`ALTER TABLE skills ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW()`,
	)

	for _, query := range migrations {
		if _, err := s.pool.Exec(ctx, query); err != nil {
			return err
		}
	}

	return nil
}
