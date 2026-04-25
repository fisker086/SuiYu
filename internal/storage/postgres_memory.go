package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fisk086/sya/internal/model"
	"github.com/pgvector/pgvector-go"
)

func extraJSONB(extra map[string]any) []byte {
	if extra == nil || len(extra) == 0 {
		return []byte("{}")
	}
	b, err := json.Marshal(extra)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func (s *PostgresStorage) StoreMemory(ctx context.Context, agentID int64, userID, sessionID, role, content string, embedding []float32, extra map[string]any) error {
	vec := pgvector.NewVector(embedding)
	ex := extraJSONB(extra)
	_, err := s.pool.Exec(ctx,
		`INSERT INTO agent_memory (agent_id, user_id, session_id, role, content, embedding, extra) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		agentID, userID, sessionID, role, content, vec, ex)
	if err != nil {
		return err
	}
	if sessionID != "" {
		if role == "user" {
			autoTitle := SessionTitleFromFirstMessage(content)
			if autoTitle != "" {
				_, _ = s.pool.Exec(ctx, `
					UPDATE chat_sessions
					SET updated_at = NOW(),
					    title = CASE WHEN COALESCE(TRIM(title), '') = '' THEN $2 ELSE title END
					WHERE id = $1`,
					sessionID, autoTitle,
				)
			} else {
				_, _ = s.pool.Exec(ctx, `UPDATE chat_sessions SET updated_at = NOW() WHERE id = $1`, sessionID)
			}
		} else {
			_, _ = s.pool.Exec(ctx, `UPDATE chat_sessions SET updated_at = NOW() WHERE id = $1`, sessionID)
		}
	}
	return nil
}

func (s *PostgresStorage) SearchMemory(ctx context.Context, agentID int64, embedding []float32, limit int) ([]model.AgentMemory, error) {
	vec := pgvector.NewVector(embedding)
	rows, err := s.pool.Query(ctx,
		`SELECT id, agent_id, user_id, session_id, role, content, created_at FROM agent_memory WHERE agent_id = $1 ORDER BY embedding <-> $2 LIMIT $3`,
		agentID, vec, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []model.AgentMemory
	for rows.Next() {
		var m model.AgentMemory
		if err := rows.Scan(&m.ID, &m.AgentID, &m.UserID, &m.SessionID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		memories = append(memories, m)
	}
	return memories, nil
}

func (s *PostgresStorage) StoreSemanticMemory(ctx context.Context, agentID int64, userID, content string, metadata map[string]any, embedding []float32) error {
	vec := pgvector.NewVector(embedding)
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO semantic_memory (agent_id, user_id, content, metadata, embedding) VALUES ($1, $2, $3, $4, $5)`,
		agentID, userID, content, metadataJSON, vec)
	return err
}

func (s *PostgresStorage) SearchSemanticMemory(ctx context.Context, agentID int64, embedding []float32, limit int) ([]model.SemanticMemory, error) {
	vec := pgvector.NewVector(embedding)
	rows, err := s.pool.Query(ctx,
		`SELECT id, agent_id, user_id, content, metadata, created_at FROM semantic_memory WHERE agent_id = $1 ORDER BY embedding <-> $2 LIMIT $3`,
		agentID, vec, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []model.SemanticMemory
	for rows.Next() {
		var m model.SemanticMemory
		var metadataJSON []byte
		if err := rows.Scan(&m.ID, &m.AgentID, &m.UserID, &m.Content, &metadataJSON, &m.CreatedAt); err != nil {
			return nil, err
		}
		if metadataJSON != nil {
			if err := json.Unmarshal(metadataJSON, &m.Metadata); err != nil {
				m.Metadata = nil
			}
		}
		memories = append(memories, m)
	}
	return memories, nil
}

func (s *PostgresStorage) GetUserProfile(ctx context.Context, userID string, agentID int64) (*model.UserProfile, error) {
	var p model.UserProfile
	var profileJSON []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, agent_id, profile, embedding, created_at, updated_at FROM user_profile WHERE user_id = $1 AND agent_id = $2`,
		userID, agentID).Scan(&p.ID, &p.UserID, &p.AgentID, &profileJSON, &p.Embedding, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if profileJSON != nil {
		if err := json.Unmarshal(profileJSON, &p.Profile); err != nil {
			p.Profile = nil
		}
	}
	return &p, nil
}

func (s *PostgresStorage) UpsertUserProfile(ctx context.Context, userID string, agentID int64, profile map[string]any, embedding []float32) error {
	profileJSON, err := json.Marshal(profile)
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}
	vec := pgvector.NewVector(embedding)
	_, err = s.pool.Exec(ctx,
		`INSERT INTO user_profile (user_id, agent_id, profile, embedding) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_id, agent_id) DO UPDATE SET profile = EXCLUDED.profile, embedding = EXCLUDED.embedding, updated_at = NOW()`,
		userID, agentID, profileJSON, vec)
	return err
}

func (s *PostgresStorage) SearchUserProfile(ctx context.Context, userID string, agentID int64, embedding []float32) (*model.UserProfile, error) {
	vec := pgvector.NewVector(embedding)
	var p model.UserProfile
	var profileJSON []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, agent_id, profile, embedding, created_at, updated_at FROM user_profile WHERE user_id = $1 AND agent_id = $2 ORDER BY embedding <-> $3 LIMIT 1`,
		userID, agentID, vec).Scan(&p.ID, &p.UserID, &p.AgentID, &profileJSON, &p.Embedding, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if profileJSON != nil {
		if err := json.Unmarshal(profileJSON, &p.Profile); err != nil {
			p.Profile = nil
		}
	}
	return &p, nil
}
