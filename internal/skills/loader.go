package skills

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/fisk086/sya/internal/schema"
)

type Loader struct {
	skillsDir string
}

func NewLoader(skillsDir string) *Loader {
	return &Loader{skillsDir: skillsDir}
}

type SkillDefinition struct {
	Key                string
	Name               string
	Description        string
	SourceRef          string
	ActivationKeywords []string
	PromptHint         string
	ToolName           string
	ExecutionMode      string
	IsActive           bool
}

func (l *Loader) LoadAll() ([]*SkillDefinition, error) {
	entries, err := os.ReadDir(l.skillsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read skills dir: %w", err)
	}

	var skills []*SkillDefinition
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(l.skillsDir, entry.Name())
		skill, err := l.LoadSkill(skillPath)
		if err != nil {
			slog.Warn("skip skill dir: invalid or unreadable SKILL.md", "dir", entry.Name(), "path", skillPath, "err", err)
			continue
		}
		skills = append(skills, skill)
	}

	return skills, nil
}

func (l *Loader) LoadSkill(skillPath string) (*SkillDefinition, error) {
	skillFile := filepath.Join(skillPath, "SKILL.md")
	data, err := os.ReadFile(skillFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read SKILL.md: %w", err)
	}

	frontmatter := parseFrontmatter(string(data))
	if frontmatter == nil {
		return nil, fmt.Errorf("missing frontmatter")
	}

	skillKey := filepath.Base(skillPath)
	skillName := getString(frontmatter, "name", skillKey)
	description := getString(frontmatter, "description", "")
	executionMode := getString(frontmatter, "execution_mode", "server")

	return &SkillDefinition{
		Key:                fmt.Sprintf("builtin_skill.%s", skillKey),
		Name:               skillName,
		Description:        description,
		SourceRef:          skillKey,
		ActivationKeywords: parseKeywords(getString(frontmatter, "activation_keywords", "")),
		PromptHint:         extractContent(string(data)),
		ExecutionMode:      executionMode,
		IsActive:           true,
	}, nil
}

func parseFrontmatter(content string) map[string]string {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return nil
	}

	meta := make(map[string]string)
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			break
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			meta[key] = value
		}
	}
	return meta
}

func getString(m map[string]string, key, defaultVal string) string {
	if v, ok := m[key]; ok && v != "" {
		return v
	}
	return defaultVal
}

func parseKeywords(keywords string) []string {
	if keywords == "" {
		return nil
	}

	keywords = strings.TrimSpace(keywords)
	if len(keywords) > 1 && (strings.HasPrefix(keywords, "[") || strings.HasPrefix(keywords, "(")) {
		keywords = keywords[1 : len(keywords)-1]
	}

	var result []string
	for _, k := range strings.Split(keywords, ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			result = append(result, k)
		}
	}
	return result
}

func extractContent(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inFrontmatter := false
	for i, line := range lines {
		if i == 0 && strings.TrimSpace(line) == "---" {
			inFrontmatter = true
			continue
		}
		if inFrontmatter && strings.TrimSpace(line) == "---" {
			inFrontmatter = false
			continue
		}
		if !inFrontmatter {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

type Registry struct {
	skills map[string]*SkillDefinition
}

func NewRegistry() *Registry {
	return &Registry{skills: make(map[string]*SkillDefinition)}
}

func (r *Registry) Register(skill *SkillDefinition) {
	r.skills[skill.Key] = skill
}

func (r *Registry) Get(key string) (*SkillDefinition, bool) {
	skill, ok := r.skills[key]
	return skill, ok
}

func (r *Registry) List() []*SkillDefinition {
	skills := make([]*SkillDefinition, 0, len(r.skills))
	for _, s := range r.skills {
		skills = append(skills, s)
	}
	return skills
}

func (r *Registry) ListByKeywords(query string) []*SkillDefinition {
	query = strings.ToLower(query)
	var result []*SkillDefinition
	for _, s := range r.skills {
		if !s.IsActive {
			continue
		}
		for _, kw := range s.ActivationKeywords {
			if strings.Contains(strings.ToLower(query), strings.ToLower(kw)) {
				result = append(result, s)
				break
			}
		}
	}
	return result
}

func (r *Registry) ToSchema() []*schema.Skill {
	skills := make([]*schema.Skill, 0, len(r.skills))
	for _, s := range r.skills {
		cat := schema.DefaultSkillCategory[s.Key]
		if cat == "" {
			cat = schema.SkillCategorySafe
		}
		risk := schema.DefaultSkillRiskLevel[s.Key]
		if risk == "" {
			risk = schema.RiskLevelLow
		}
		skills = append(skills, &schema.Skill{
			Key:           s.Key,
			Name:          s.Name,
			Description:   s.Description,
			SourceRef:     s.SourceRef,
			Category:      cat,
			RiskLevel:     risk,
			ExecutionMode: s.ExecutionMode,
			IsActive:      s.IsActive,
		})
	}
	return skills
}
