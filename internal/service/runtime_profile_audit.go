package service

import (
	"log/slog"
	"slices"
	"strings"

	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/schema"
)

// diffRuntimeProfilePersistence returns field names where the client sent a non-empty / meaningful
// value that does not match what was read back from storage. This catches silent drops (e.g. DB
// column missing) without treating omitted JSON fields as mismatches.
func diffRuntimeProfilePersistence(want, got *schema.RuntimeProfile) []string {
	if want == nil || got == nil {
		return nil
	}
	var diffs []string

	if want.SourceAgent != "" && got.SourceAgent != want.SourceAgent {
		diffs = append(diffs, "source_agent")
	}
	if want.Archetype != "" && got.Archetype != want.Archetype {
		diffs = append(diffs, "archetype")
	}
	if want.ExecutionMode != "" && got.ExecutionMode != want.ExecutionMode {
		diffs = append(diffs, "execution_mode")
	}
	if want.MaxIterations > 0 && got.MaxIterations != want.MaxIterations {
		diffs = append(diffs, "max_iterations")
	}
	if want.PlanPrompt != "" && got.PlanPrompt != want.PlanPrompt {
		diffs = append(diffs, "plan_prompt")
	}
	if want.ReflectionDepth > 0 && got.ReflectionDepth != want.ReflectionDepth {
		diffs = append(diffs, "reflection_depth")
	}
	if want.Role != "" && got.Role != want.Role {
		diffs = append(diffs, "role")
	}
	if want.Goal != "" && got.Goal != want.Goal {
		diffs = append(diffs, "goal")
	}
	if want.Backstory != "" && got.Backstory != want.Backstory {
		diffs = append(diffs, "backstory")
	}
	if want.SystemPrompt != "" && got.SystemPrompt != want.SystemPrompt {
		diffs = append(diffs, "system_prompt")
	}
	if want.LlmModel != "" && got.LlmModel != want.LlmModel {
		diffs = append(diffs, "llm_model")
	}
	// Omit temperature / stream_enabled / memory_enabled: JSON omitempty makes zero values
	// indistinguishable from omitted fields and causes false-positive mismatches.
	if len(want.SkillIDs) > 0 && !slices.Equal(want.SkillIDs, got.SkillIDs) {
		diffs = append(diffs, "skill_ids")
	}
	if len(want.MCPConfigIDs) > 0 && !slices.Equal(want.MCPConfigIDs, got.MCPConfigIDs) {
		diffs = append(diffs, "mcp_config_ids")
	}
	return diffs
}

// logRuntimeProfilePersistenceCheck reloads the agent and logs a warning if requested runtime
// fields did not round-trip. Use after create/update when runtime_profile was sent.
func logRuntimeProfilePersistenceCheck(op string, agentID int64, want *schema.RuntimeProfile, got *schema.RuntimeProfile) {
	if want == nil || got == nil {
		return
	}
	diffs := diffRuntimeProfilePersistence(want, got)
	if len(diffs) == 0 {
		logger.Debug("runtime_profile persistence check ok",
			slog.String("op", op),
			slog.Int64("agent_id", agentID),
			slog.String("execution_mode", got.ExecutionMode),
		)
		return
	}
	logger.Warn("runtime_profile persistence mismatch: requested values did not round-trip from storage; check DB schema and UPDATE/INSERT for these fields",
		slog.String("op", op),
		slog.Int64("agent_id", agentID),
		slog.String("fields", strings.Join(diffs, ",")),
	)
}
