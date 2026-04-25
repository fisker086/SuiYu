package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/skills"
	"github.com/fisk086/sya/internal/storage"
)

type SkillService struct {
	store storage.Storage
}

func NewSkillService(store storage.Storage) *SkillService {
	return &SkillService{store: store}
}

func enrichSkillMetadata(sk *schema.Skill) {
	if sk == nil {
		return
	}
	if sk.Category == "" {
		if c, ok := schema.DefaultSkillCategory[sk.Key]; ok {
			sk.Category = c
		}
	}
	if sk.RiskLevel == "" {
		if r, ok := schema.DefaultSkillRiskLevel[sk.Key]; ok {
			sk.RiskLevel = r
		} else {
			sk.RiskLevel = schema.RiskLevelLow
		}
	}
	if sk.ExecutionMode == "" {
		sk.ExecutionMode = defaultSkillExecutionMode(sk.Key)
	}
}

// defaultSkillExecutionMode mirrors SKILL.md execution_mode for built-ins when DB has no explicit execution_mode.
// If skills.execution_mode is set in the database, that value is authoritative at runtime (see agent.getToolExecutionModeFromTools).
func defaultSkillExecutionMode(skillKey string) string {
	switch skillKey {
	case "builtin_skill.browser_client", "builtin_skill.visible_browser",
		"builtin_skill.test_runner":
		return schema.ExecutionModeClient
	case "builtin_skill.docker_operator", "builtin_skill.git_operator", "builtin_skill.file_parser",
		"builtin_skill.system_monitor", "builtin_skill.cron_manager", "builtin_skill.network_tools",
		"builtin_skill.cert_checker", "builtin_skill.nginx_diagnose", "builtin_skill.dns_lookup",
		"builtin_skill.datetime", "builtin_skill.regex", "builtin_skill.json_parser",
		"builtin_skill.csv_analyzer", "builtin_skill.log_analyzer", "builtin_skill.image_analyzer",
		"builtin_skill.terraform_plan":
		return schema.ExecutionModeClient
	default:
		return schema.ExecutionModeServer
	}
}

func (s *SkillService) ListSkills() ([]*schema.Skill, error) {
	list, err := s.store.ListSkills()
	if err != nil {
		return nil, err
	}
	for _, sk := range list {
		enrichSkillMetadata(sk)
	}
	return list, nil
}

// GetSkill loads a skill by id. If skillsDir is non-empty and stored content is empty, fills Content from skillsDir/<SourceRef>/SKILL.md when the file exists (built-in skills).
func (s *SkillService) GetSkill(id int64, skillsDir ...string) (*schema.Skill, error) {
	sk, err := s.store.GetSkill(id)
	if err != nil {
		return nil, err
	}
	if sk == nil {
		return nil, nil
	}
	enrichSkillMetadata(sk)
	dir := ""
	if len(skillsDir) > 0 {
		dir = skillsDir[0]
	}
	if dir != "" && strings.TrimSpace(sk.Content) == "" && strings.TrimSpace(sk.SourceRef) != "" {
		p := filepath.Join(dir, sk.SourceRef, "SKILL.md")
		if b, err := os.ReadFile(p); err == nil {
			sk.Content = string(b)
		}
	}
	return sk, nil
}

func (s *SkillService) CreateSkill(req *schema.CreateSkillRequest) (*schema.Skill, error) {
	sk, err := s.store.CreateSkill(req)
	if err != nil {
		return nil, err
	}
	enrichSkillMetadata(sk)
	return sk, nil
}

func normalizeSkillRiskLevel(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case schema.RiskLevelLow, schema.RiskLevelMedium, schema.RiskLevelHigh, schema.RiskLevelCritical:
		return s
	default:
		return ""
	}
}

func (s *SkillService) UpdateSkill(id int64, req *schema.UpdateSkillRequest) (*schema.Skill, error) {
	if req == nil {
		return nil, fmt.Errorf("invalid request")
	}
	patched := *req
	if req.RiskLevel != nil {
		r := normalizeSkillRiskLevel(*req.RiskLevel)
		if r == "" {
			return nil, fmt.Errorf("invalid risk_level (use low|medium|high|critical)")
		}
		patched.RiskLevel = &r
	}
	if req.ExecutionMode != nil {
		m := strings.ToLower(strings.TrimSpace(*req.ExecutionMode))
		if m != schema.ExecutionModeClient && m != schema.ExecutionModeServer {
			return nil, fmt.Errorf("invalid execution_mode (use client|server)")
		}
		patched.ExecutionMode = &m
	}
	sk, err := s.store.UpdateSkill(id, &patched)
	if err != nil {
		return nil, err
	}
	enrichSkillMetadata(sk)
	return sk, nil
}

func (s *SkillService) DeleteSkill(id int64) error {
	return s.store.DeleteSkill(id)
}

// SyncBuiltinSkills inserts skills from the registry that are missing in storage (e.g. skills/…/SKILL.md).
// Safe to call on startup or via API; does not overwrite existing rows.
func (s *SkillService) SyncBuiltinSkills(registry *skills.Registry, skillsDir string) (created int) {
	if registry == nil {
		return 0
	}
	existing, err := s.store.ListSkills()
	if err != nil {
		logger.Warn("list skills before builtin seed", "err", err)
		existing = nil
	}
	have := make(map[string]struct{}, len(existing))
	for _, sk := range existing {
		if sk != nil && sk.Key != "" {
			have[sk.Key] = struct{}{}
		}
	}
	for _, def := range registry.List() {
		if _, ok := have[def.Key]; ok {
			continue
		}
		var content string
		if def.SourceRef != "" && skillsDir != "" {
			skillFile := filepath.Join(skillsDir, def.SourceRef, "SKILL.md")
			if data, err := os.ReadFile(skillFile); err == nil {
				content = string(data)
			} else {
				logger.Warn("failed to read skill file", "source_ref", def.SourceRef, "err", err)
			}
		}
		req := &schema.CreateSkillRequest{
			Key:         def.Key,
			Name:        def.Name,
			Description: def.Description,
			Content:     content,
			SourceRef:   def.SourceRef,
		}
		skill, err := s.store.CreateSkill(req)
		if err != nil {
			// Unique violation: row exists but was not visible in have (e.g. old ListSkills failed on NULL scans).
			var pe *pgconn.PgError
			if errors.As(err, &pe) && pe.Code == "23505" {
				logger.Debug("builtin skill already in database", "key", def.Key)
				continue
			}
			logger.Error("failed to create builtin skill", "key", def.Key, "name", def.Name, "err", err)
			continue
		}
		have[def.Key] = struct{}{}
		created++
		logger.Info("created builtin skill", "key", def.Key, "name", skill.Name, "skill_id", skill.ID)
	}
	return created
}
