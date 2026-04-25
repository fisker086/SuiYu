package skills

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/schema"
)

type SkillExecutor struct {
	registry *Registry
}

func NewSkillExecutor(registry *Registry) *SkillExecutor {
	return &SkillExecutor{registry: registry}
}

func (e *SkillExecutor) GetTools() ([]*schema.ToolInfo, error) {
	skills := e.registry.List()
	tools := make([]*schema.ToolInfo, 0, len(skills))

	for _, skill := range skills {
		if !skill.IsActive {
			continue
		}

		params := make(map[string]*schema.ParameterInfo)
		params["input"] = &schema.ParameterInfo{
			Type:     schema.String,
			Desc:     "Skill input",
			Required: true,
		}

		tool := &schema.ToolInfo{
			Name:        skill.ToolName,
			Desc:        skill.Description,
			ParamsOneOf: schema.NewParamsOneOfByParams(params),
		}
		tools = append(tools, tool)
	}

	return tools, nil
}

func (e *SkillExecutor) Execute(ctx context.Context, skillKey string, input map[string]any) (string, error) {
	skill, ok := e.registry.Get(skillKey)
	if !ok {
		return "", fmt.Errorf("skill not found: %s", skillKey)
	}

	if skill.PromptHint == "" {
		return fmt.Sprintf("Skill %s executed (no prompt content)", skill.Name), nil
	}

	return skill.PromptHint, nil
}
