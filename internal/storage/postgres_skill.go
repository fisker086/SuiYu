package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fisk086/sya/internal/schema"
)

// skillSelectCols uses COALESCE on nullable TEXT columns so pgx can scan into Go string (NULL would otherwise fail the whole list).
const skillSelectCols = `id, key, name,
		COALESCE(description,''), COALESCE(content,''), COALESCE(source_ref,''),
		COALESCE(risk_level,''), COALESCE(execution_mode,''), COALESCE(prompt_hint,''),
		COALESCE(is_active, true), created_at, COALESCE(updated_at, created_at)`

func (s *PostgresStorage) ListSkills() ([]*schema.Skill, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT `+skillSelectCols+` FROM skills ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var skills []*schema.Skill
	for rows.Next() {
		var sk schema.Skill
		if err := rows.Scan(&sk.ID, &sk.Key, &sk.Name, &sk.Description, &sk.Content, &sk.SourceRef, &sk.RiskLevel, &sk.ExecutionMode, &sk.PromptHint, &sk.IsActive, &sk.CreatedAt, &sk.UpdatedAt); err != nil {
			return nil, err
		}
		skills = append(skills, &sk)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return skills, nil
}

func (s *PostgresStorage) GetSkill(id int64) (*schema.Skill, error) {
	var sk schema.Skill
	err := s.pool.QueryRow(context.Background(),
		`SELECT `+skillSelectCols+` FROM skills WHERE id = $1`, id).
		Scan(&sk.ID, &sk.Key, &sk.Name, &sk.Description, &sk.Content, &sk.SourceRef, &sk.RiskLevel, &sk.ExecutionMode, &sk.PromptHint, &sk.IsActive, &sk.CreatedAt, &sk.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sk, nil
}

func (s *PostgresStorage) CreateSkill(req *schema.CreateSkillRequest) (*schema.Skill, error) {
	var id int64
	var createdAt, updatedAt time.Time
	content := req.Content
	if content == "" {
		content = fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n# %s\n\n%s", req.Name, req.Description, req.Name, req.Description)
	}
	err := s.pool.QueryRow(context.Background(),
		`INSERT INTO skills (key, name, description, content, source_ref, risk_level, execution_mode, prompt_hint) VALUES ($1, $2, $3, $4, $5, $6, '', '') RETURNING id, created_at, updated_at`,
		req.Key, req.Name, req.Description, content, req.SourceRef, schema.RiskLevelLow).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	return &schema.Skill{ID: id, Key: req.Key, Name: req.Name, Description: req.Description, Content: content, SourceRef: req.SourceRef, RiskLevel: schema.RiskLevelLow, IsActive: true, CreatedAt: createdAt, UpdatedAt: updatedAt}, nil
}

func (s *PostgresStorage) UpdateSkill(id int64, req *schema.UpdateSkillRequest) (*schema.Skill, error) {
	var sets []string
	var args []interface{}
	n := 1
	if req.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", n))
		args = append(args, *req.Name)
		n++
	}
	if req.Description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", n))
		args = append(args, *req.Description)
		n++
	}
	if req.Content != nil {
		sets = append(sets, fmt.Sprintf("content = $%d", n))
		args = append(args, *req.Content)
		n++
	}
	if req.SourceRef != nil {
		sets = append(sets, fmt.Sprintf("source_ref = $%d", n))
		args = append(args, *req.SourceRef)
		n++
	}
	if req.RiskLevel != nil {
		sets = append(sets, fmt.Sprintf("risk_level = $%d", n))
		args = append(args, *req.RiskLevel)
		n++
	}
	if req.ExecutionMode != nil {
		sets = append(sets, fmt.Sprintf("execution_mode = $%d", n))
		args = append(args, *req.ExecutionMode)
		n++
	}
	if req.PromptHint != nil {
		sets = append(sets, fmt.Sprintf("prompt_hint = $%d", n))
		args = append(args, *req.PromptHint)
		n++
	}
	if req.IsActive != nil {
		sets = append(sets, fmt.Sprintf("is_active = $%d", n))
		args = append(args, *req.IsActive)
		n++
	}
	if len(sets) == 0 {
		return s.GetSkill(id)
	}
	sets = append(sets, "updated_at = NOW()")
	query := fmt.Sprintf("UPDATE skills SET %s WHERE id = $%d", strings.Join(sets, ", "), n)
	args = append(args, id)
	ct, err := s.pool.Exec(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() == 0 {
		return nil, fmt.Errorf("skill not found: %d", id)
	}
	return s.GetSkill(id)
}

func (s *PostgresStorage) DeleteSkill(id int64) error {
	_, err := s.pool.Exec(context.Background(), `DELETE FROM skills WHERE id = $1`, id)
	return err
}

func (s *PostgresStorage) GetSkillByKey(key string) (*schema.Skill, error) {
	var sk schema.Skill
	err := s.pool.QueryRow(context.Background(),
		`SELECT `+skillSelectCols+` FROM skills WHERE key = $1`, key).
		Scan(&sk.ID, &sk.Key, &sk.Name, &sk.Description, &sk.Content, &sk.SourceRef, &sk.RiskLevel, &sk.ExecutionMode, &sk.PromptHint, &sk.IsActive, &sk.CreatedAt, &sk.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sk, nil
}

func (s *PostgresStorage) UpsertSkill(req *schema.CreateSkillRequest) (*schema.Skill, error) {
	var id int64
	var createdAt, updatedAt time.Time
	content := req.Content
	if content == "" {
		content = fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n# %s\n\n%s", req.Name, req.Description, req.Name, req.Description)
	}
	err := s.pool.QueryRow(context.Background(),
		`INSERT INTO skills (key, name, description, content, source_ref, risk_level, execution_mode, prompt_hint)
		 VALUES ($1, $2, $3, $4, $5, $6, '', '')
		 ON CONFLICT (key) DO UPDATE SET
		   name = EXCLUDED.name,
		   description = EXCLUDED.description,
		   content = EXCLUDED.content,
		   source_ref = EXCLUDED.source_ref,
		   updated_at = NOW()
		 RETURNING id, created_at, updated_at`,
		req.Key, req.Name, req.Description, content, req.SourceRef, schema.RiskLevelLow).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	return &schema.Skill{ID: id, Key: req.Key, Name: req.Name, Description: req.Description, Content: content, SourceRef: req.SourceRef, RiskLevel: schema.RiskLevelLow, IsActive: true, CreatedAt: createdAt, UpdatedAt: updatedAt}, nil
}
