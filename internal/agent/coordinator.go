package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/schema"
	storepkg "github.com/fisk086/sya/internal/storage"
)

type AgentCapability struct {
	AgentID   int64
	AgentName string
	SkillIDs  []string
	MCPIDs    []int64
}

type GroupCapabilities struct {
	GroupID      int64
	AgentCaps    map[int64]*AgentCapability
	AllSkills   map[string]*schema.Skill
	AllMCPs     map[int64]*schema.MCPConfig
	SkillIndex  map[string][]int64
	MCPIndex    map[int64][]int64
}

type GroupCoordinator struct {
	store      storepkg.Storage
	runtime   *Runtime
	caller    AgentCaller
}

func NewGroupCoordinator(store storepkg.Storage, runtime *Runtime, caller AgentCaller) *GroupCoordinator {
	return &GroupCoordinator{
		store:   store,
		runtime: runtime,
		caller:  caller,
	}
}

func (c *GroupCoordinator) CollectGroupCapabilities(ctx context.Context, groupID int64) (*GroupCapabilities, error) {
	gc := &GroupCapabilities{
		GroupID:    groupID,
		AgentCaps:  make(map[int64]*AgentCapability),
		AllSkills:  make(map[string]*schema.Skill),
		AllMCPs:   make(map[int64]*schema.MCPConfig),
		SkillIndex: make(map[string][]int64),
		MCPIndex:   make(map[int64][]int64),
	}

	group, err := c.store.GetChatGroup(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat group: %w", err)
	}

	if len(group.Members) == 0 {
		return gc, nil
	}

	agentIDs := make([]int64, len(group.Members))
	for i, m := range group.Members {
		agentIDs[i] = m.AgentID
	}

	allSkills, err := c.store.ListSkills()
	if err != nil {
		logger.Warn("coordinator: failed to list skills", "err", err)
	}
	for _, sk := range allSkills {
		if sk != nil {
			gc.AllSkills[sk.Key] = sk
		}
	}

	allMCPs, err := c.store.ListMCPConfigs()
	if err != nil {
		logger.Warn("coordinator: failed to list MCP configs", "err", err)
	}
	for _, mcp := range allMCPs {
		if mcp != nil {
			gc.AllMCPs[mcp.ID] = mcp
		}
	}

	agents, err := c.store.ListAgents()
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	agentMap := make(map[int64]*schema.Agent)
	for _, a := range agents {
		agentMap[a.ID] = a
	}

	for _, m := range group.Members {
		agentID := m.AgentID
		agent, ok := agentMap[agentID]
		if !ok {
			continue
		}

		cap := &AgentCapability{
			AgentID:   agentID,
			AgentName: agent.Name,
			SkillIDs: agent.SkillIDs,
			MCPIDs:   agent.MCPConfigIDs,
		}
		gc.AgentCaps[agentID] = cap

		for _, skillKey := range agent.SkillIDs {
			gc.SkillIndex[skillKey] = append(gc.SkillIndex[skillKey], agentID)
		}
		for _, mcpID := range agent.MCPConfigIDs {
			gc.MCPIndex[mcpID] = append(gc.MCPIndex[mcpID], agentID)
		}
	}

	return gc, nil
}

func (gc *GroupCapabilities) FindDuplicateSkills() map[string][]int64 {
	dupes := make(map[string][]int64)
	for skillKey, agentIDs := range gc.SkillIndex {
		if len(agentIDs) > 1 {
			dupes[skillKey] = agentIDs
		}
	}
	return dupes
}

func (gc *GroupCapabilities) FindSkillProviders(skillKey string) []int64 {
	return gc.SkillIndex[skillKey]
}

func (gc *GroupCapabilities) FindMCPProviders(mcpID int64) []int64 {
	return gc.MCPIndex[mcpID]
}

func (gc *GroupCapabilities) Summary() string {
	skillsByAgent := make(map[int64]int)
	mcpsByAgent := make(map[int64]int)

	for _, cap := range gc.AgentCaps {
		skillsByAgent[cap.AgentID] = len(cap.SkillIDs)
		mcpsByAgent[cap.AgentID] = len(cap.MCPIDs)
	}

	uniqueSkills := len(gc.AllSkills)
	uniqueMCPs := len(gc.AllMCPs)

	return fmt.Sprintf("群聊 %d: %d 个 agent, %d 个技能, %d 个 MCP",
		gc.GroupID, len(gc.AgentCaps), uniqueSkills, uniqueMCPs)
}

func (c *GroupCoordinator) BuildGroupCapabilityHint(ctx context.Context, groupID int64, callerAgentID int64) string {
	gc, err := c.CollectGroupCapabilities(ctx, groupID)
	if err != nil {
		logger.Warn("coordinator: failed to collect capabilities", "group_id", groupID, "err", err)
		return ""
	}

	if len(gc.AgentCaps) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## 群聊成员能力 (Group Peers)\n")
	b.WriteString("当前群聊有以下成员及其能力，当需要其他成员协助时可以调用 builtin_group_send_message:\n")

	for _, cap := range gc.AgentCaps {
		if cap.AgentID == callerAgentID {
			continue
		}

		skillDescs := make([]string, 0, len(cap.SkillIDs))
		for _, sk := range cap.SkillIDs {
			if s, ok := gc.AllSkills[sk]; ok && s != nil {
				skillDescs = append(skillDescs, s.Name)
			} else {
				skillDescs = append(skillDescs, sk)
			}
		}

		mcpDescs := make([]string, 0, len(cap.MCPIDs))
		for _, mcpID := range cap.MCPIDs {
			if m, ok := gc.AllMCPs[mcpID]; ok && m != nil {
				mcpDescs = append(mcpDescs, m.Name)
			} else {
				mcpDescs = append(mcpDescs, fmt.Sprintf("mcp-%d", mcpID))
			}
		}

		b.WriteString(fmt.Sprintf("- %s (id %d): ", cap.AgentName, cap.AgentID))
		if len(skillDescs) > 0 {
			b.WriteString(fmt.Sprintf("技能[%s]", strings.Join(skillDescs, ", ")))
		}
		if len(mcpDescs) > 0 {
			if len(skillDescs) > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("MCP[%s]", strings.Join(mcpDescs, ", ")))
		}
		b.WriteString("\n")
	}

	dupes := gc.FindDuplicateSkills()
	if len(dupes) > 0 {
		b.WriteString("\n⚠️ 重复技能 (请知晓，避免重复调用):\n")
		for skillKey, agentIDs := range dupes {
			agentNames := make([]string, 0, len(agentIDs))
			for _, aid := range agentIDs {
				if cap, ok := gc.AgentCaps[aid]; ok {
					agentNames = append(agentNames, cap.AgentName)
				}
			}
			b.WriteString(fmt.Sprintf("  %s: %s\n", skillKey, strings.Join(agentNames, ", ")))
		}
	}

	return b.String()
}

func (c *GroupCoordinator) BuildCapabilityPrompt(ctx context.Context, groupID int64) (string, error) {
	gc, err := c.CollectGroupCapabilities(ctx, groupID)
	if err != nil {
		return "", err
	}

	if len(gc.AgentCaps) == 0 {
		return "群聊中暂无 agent 成员", nil
	}

	dupes := gc.FindDuplicateSkills()

	lines := []string{
		"群聊 Agent 能力清单:",
	}

	for _, cap := range gc.AgentCaps {
		skillDescs := make([]string, 0, len(cap.SkillIDs))
		for _, sk := range cap.SkillIDs {
			if s, ok := gc.AllSkills[sk]; ok && s != nil {
				skillDescs = append(skillDescs, fmt.Sprintf("%s(%s)", s.Name, s.Description))
			} else {
				skillDescs = append(skillDescs, sk)
			}
		}

		mcpDescs := make([]string, 0, len(cap.MCPIDs))
		for _, mcpID := range cap.MCPIDs {
			if m, ok := gc.AllMCPs[mcpID]; ok && m != nil {
				mcpDescs = append(mcpDescs, m.Name)
			} else {
				mcpDescs = append(mcpDescs, fmt.Sprintf("mcp-%d", mcpID))
			}
		}

		line := fmt.Sprintf("- %s: 技能 %v, MCP %v", cap.AgentName, skillDescs, mcpDescs)
		lines = append(lines, line)
	}

	if len(dupes) > 0 {
		lines = append(lines, "\n重复技能:")
		for skillKey, agentIDs := range dupes {
			agentNames := make([]string, 0, len(agentIDs))
			for _, aid := range agentIDs {
				if cap, ok := gc.AgentCaps[aid]; ok {
					agentNames = append(agentNames, cap.AgentName)
				}
			}
			line := fmt.Sprintf("  %s: %v", skillKey, agentNames)
			lines = append(lines, line)
		}
	}

	result := gc.Summary()
	lines = append(lines, "\n"+result)

	str := ""
	for _, line := range lines {
		if str != "" {
			str += "\n"
		}
		str += line
	}

	return str, nil
}