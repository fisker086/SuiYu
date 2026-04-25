package storage

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/schema"
	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
)

func extraStringSlice(ex map[string]any, key string) []string {
	if ex == nil {
		return nil
	}
	v, ok := ex[key]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case []string:
		out := make([]string, len(t))
		copy(out, t)
		return out
	case []any:
		var out []string
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		if strings.TrimSpace(t) != "" {
			return []string{t}
		}
		return nil
	default:
		return nil
	}
}

func reactStepsFromAgentExtra(ex map[string]any) []map[string]any {
	if ex == nil {
		return nil
	}
	v, ok := ex["react_steps"]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case []map[string]any:
		if len(t) == 0 {
			return nil
		}
		return t
	case []any:
		out := make([]map[string]any, 0, len(t))
		for _, x := range t {
			if mm, ok := x.(map[string]any); ok {
				out = append(out, mm)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		return nil
	}
}

type InMemoryStorage struct {
	mu                  sync.RWMutex
	agents              map[int64]*schema.AgentWithRuntime
	agentCount          int64
	skills              map[int64]*schema.Skill
	skillCount          int64
	mcpConfigs          map[int64]*schema.MCPConfig
	mcpTools            map[int64][]schema.MCPServer
	mcpCount            int64
	treeNodes           map[int64][]schema.CapabilityTreeNode
	treeVersion         map[int64]int
	memories            []model.AgentMemory
	memoryCount         int64
	semanticMemories    []model.SemanticMemory
	semanticMemoryCount int64
	userProfiles        map[string]*model.UserProfile
	chatSessions        []schema.ChatSession
	workflows           map[int64]schema.AgentWorkflow
	workflowKeyToID     map[string]int64
	workflowCount       int64
	channels            map[int64]*model.Channel
	channelCount        int64

	messageChannels     map[int64]*model.MessageChannel
	messageChannelCount int64
	agentMessages       []*model.AgentMessage
	agentMessageCount   int64
	a2aCards            map[int64]*model.A2ACard
	a2aCardCount        int64

	workflowDefs       map[int64]*model.WorkflowDefinition
	workflowDefKeyToID map[string]int64
	workflowDefCount   int64
	workflowExecutions []*model.WorkflowExecution
	workflowExecCount  int64

	roles                []*model.Role
	roleCount            int64
	userRoles            []*model.UserRole
	userRoleCount        int64
	roleAgentPermissions map[int64]map[int64]bool

	schedules              map[int64]*model.Schedule
	scheduleCount          int64
	scheduleExecutions     []*model.ScheduleExecution
	scheduleExecutionCount int64

	auditLogs     []*model.AuditLog
	auditLogCount int64

	approvalRequests     []*model.ApprovalRequest
	approvalRequestCount int64

	chatGroups           map[int64]*model.ChatGroup
	chatGroupCount       int64
	chatGroupMembers     []*model.ChatGroupMember
	chatGroupMemberCount int64
}

func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		agents:               make(map[int64]*schema.AgentWithRuntime),
		skills:               make(map[int64]*schema.Skill),
		mcpConfigs:           make(map[int64]*schema.MCPConfig),
		mcpTools:             make(map[int64][]schema.MCPServer),
		treeNodes:            make(map[int64][]schema.CapabilityTreeNode),
		treeVersion:          make(map[int64]int),
		workflows:            make(map[int64]schema.AgentWorkflow),
		workflowKeyToID:      make(map[string]int64),
		channels:             make(map[int64]*model.Channel),
		userProfiles:         make(map[string]*model.UserProfile),
		messageChannels:      make(map[int64]*model.MessageChannel),
		a2aCards:             make(map[int64]*model.A2ACard),
		workflowDefs:         make(map[int64]*model.WorkflowDefinition),
		workflowDefKeyToID:   make(map[string]int64),
		schedules:            make(map[int64]*model.Schedule),
		roleAgentPermissions: make(map[int64]map[int64]bool),
		chatGroups:           make(map[int64]*model.ChatGroup),
	}
}

func (s *InMemoryStorage) ListAgents() ([]*schema.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]*schema.Agent, 0, len(s.agents))
	for _, a := range s.agents {
		agents = append(agents, &schema.Agent{
			ID:        a.ID,
			PublicID:  a.PublicID,
			Name:      a.Name,
			Desc:      a.Desc,
			Category:  a.Category,
			IsBuiltin: a.IsBuiltin,
			IsActive:  a.IsActive,
			CreatedAt: a.CreatedAt,
			UpdatedAt: a.UpdatedAt,
		})
	}
	return agents, nil
}

func (s *InMemoryStorage) GetAgent(id int64) (*schema.AgentWithRuntime, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agent, ok := s.agents[id]
	if !ok {
		return nil, ErrAgentNotFound
	}
	return agent, nil
}

func (s *InMemoryStorage) GetAgentIDByName(ctx context.Context, name string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for id, agent := range s.agents {
		if agent.Agent.Name == name {
			return id, nil
		}
	}
	return 0, ErrAgentNotFound
}

func (s *InMemoryStorage) CreateAgent(req *schema.CreateAgentRequest) (*schema.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.agentCount++
	now := time.Now()
	agent := &schema.Agent{
		ID:        s.agentCount,
		PublicID:  uuid.New().String(),
		Name:      req.Name,
		Desc:      req.Description,
		Category:  req.Category,
		IsBuiltin: false,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	agentWithRuntime := &schema.AgentWithRuntime{
		Agent: *agent,
	}
	if req.RuntimeProfile != nil {
		agentWithRuntime.RuntimeProfile = req.RuntimeProfile
	}

	s.agents[agent.ID] = agentWithRuntime
	return agent, nil
}

func (s *InMemoryStorage) UpdateAgent(id int64, req *schema.UpdateAgentRequest) (*schema.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, ok := s.agents[id]
	if !ok {
		return nil, ErrAgentNotFound
	}

	if req.Name != "" {
		agent.Name = req.Name
	}
	if req.Description != "" {
		agent.Desc = req.Description
	}
	if req.Category != "" {
		agent.Category = req.Category
	}
	agent.UpdatedAt = time.Now()

	if req.RuntimeProfile != nil {
		agent.RuntimeProfile = req.RuntimeProfile
	}

	return &schema.Agent{
		ID:        agent.ID,
		PublicID:  agent.PublicID,
		Name:      agent.Name,
		Desc:      agent.Desc,
		Category:  agent.Category,
		IsBuiltin: agent.IsBuiltin,
		IsActive:  agent.IsActive,
		CreatedAt: agent.CreatedAt,
		UpdatedAt: agent.UpdatedAt,
	}, nil
}

func (s *InMemoryStorage) DeleteAgent(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.agents, id)
	delete(s.treeNodes, id)
	delete(s.treeVersion, id)
	return nil
}

func (s *InMemoryStorage) GetCapabilityTree(agentID int64) (*schema.CapabilityTree, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodes := s.treeNodes[agentID]
	version := s.treeVersion[agentID]
	return &schema.CapabilityTree{
		AgentID: agentID,
		Version: version,
		Nodes:   nodes,
	}, nil
}

func (s *InMemoryStorage) UpdateCapabilityTree(agentID int64, nodes []schema.CapabilityTreeNode) (*schema.CapabilityTree, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.treeNodes[agentID] = nodes
	s.treeVersion[agentID]++

	return &schema.CapabilityTree{
		AgentID: agentID,
		Version: s.treeVersion[agentID],
		Nodes:   nodes,
	}, nil
}

func (s *InMemoryStorage) ListSkills() ([]*schema.Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	skills := make([]*schema.Skill, 0, len(s.skills))
	for _, sk := range s.skills {
		skills = append(skills, sk)
	}
	return skills, nil
}

func (s *InMemoryStorage) GetSkill(id int64) (*schema.Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.skills[id], nil
}

func (s *InMemoryStorage) GetSkillByKey(key string) (*schema.Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, sk := range s.skills {
		if sk.Key == key {
			return sk, nil
		}
	}
	return nil, fmt.Errorf("skill not found by key: %s", key)
}

func (s *InMemoryStorage) CreateSkill(req *schema.CreateSkillRequest) (*schema.Skill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.skillCount++
	now := time.Now()
	content := req.Content
	if content == "" {
		content = fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n# %s\n\n%s", req.Name, req.Description, req.Name, req.Description)
	}
	skill := &schema.Skill{
		ID:          s.skillCount,
		Key:         req.Key,
		Name:        req.Name,
		Description: req.Description,
		Content:     content,
		SourceRef:   req.SourceRef,
		RiskLevel:   schema.RiskLevelLow,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.skills[skill.ID] = skill
	return skill, nil
}

func (s *InMemoryStorage) UpsertSkill(req *schema.CreateSkillRequest) (*schema.Skill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sk := range s.skills {
		if sk.Key == req.Key {
			sk.Name = req.Name
			sk.Description = req.Description
			sk.Content = req.Content
			sk.SourceRef = req.SourceRef
			sk.UpdatedAt = time.Now()
			return sk, nil
		}
	}

	s.skillCount++
	now := time.Now()
	content := req.Content
	if content == "" {
		content = fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n# %s\n\n%s", req.Name, req.Description, req.Name, req.Description)
	}
	skill := &schema.Skill{
		ID:          s.skillCount,
		Key:         req.Key,
		Name:        req.Name,
		Description: req.Description,
		Content:     content,
		SourceRef:   req.SourceRef,
		RiskLevel:   schema.RiskLevelLow,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.skills[skill.ID] = skill
	return skill, nil
}

func (s *InMemoryStorage) UpdateSkill(id int64, req *schema.UpdateSkillRequest) (*schema.Skill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sk, ok := s.skills[id]
	if !ok {
		return nil, fmt.Errorf("skill not found: %d", id)
	}
	if req.Name != nil {
		sk.Name = *req.Name
	}
	if req.Description != nil {
		sk.Description = *req.Description
	}
	if req.Content != nil {
		sk.Content = *req.Content
	}
	if req.SourceRef != nil {
		sk.SourceRef = *req.SourceRef
	}
	if req.RiskLevel != nil {
		sk.RiskLevel = *req.RiskLevel
	}
	if req.ExecutionMode != nil {
		sk.ExecutionMode = *req.ExecutionMode
	}
	if req.PromptHint != nil {
		sk.PromptHint = *req.PromptHint
	}
	if req.IsActive != nil {
		sk.IsActive = *req.IsActive
	}
	sk.UpdatedAt = time.Now()
	return sk, nil
}

func (s *InMemoryStorage) DeleteSkill(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.skills, id)
	return nil
}

func (s *InMemoryStorage) ListMCPConfigs() ([]*schema.MCPConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cfgs := make([]*schema.MCPConfig, 0, len(s.mcpConfigs))
	for _, cfg := range s.mcpConfigs {
		cfgs = append(cfgs, cfg)
	}
	return cfgs, nil
}

func (s *InMemoryStorage) GetMCPConfig(id int64) (*schema.MCPConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cfg, ok := s.mcpConfigs[id]
	if !ok {
		return nil, ErrMCPConfigNotFound
	}
	return cfg, nil
}

func (s *InMemoryStorage) CreateMCPConfig(req *schema.CreateMCPConfigRequest) (*schema.MCPConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.mcpCount++
	now := time.Now()
	cfg := &schema.MCPConfig{
		ID:           s.mcpCount,
		Key:          req.Key,
		Name:         req.Name,
		Transport:    req.Transport,
		Endpoint:     req.Endpoint,
		Config:       req.Config,
		IsActive:     true,
		HealthStatus: "unknown",
		ToolCount:    0,
		CreatedAt:    now,
	}
	s.mcpConfigs[cfg.ID] = cfg
	return cfg, nil
}

func (s *InMemoryStorage) UpdateMCPConfig(id int64, req *schema.CreateMCPConfigRequest) (*schema.MCPConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, ok := s.mcpConfigs[id]
	if !ok {
		return nil, ErrMCPConfigNotFound
	}

	if req.Key != "" {
		cfg.Key = req.Key
	}
	if req.Name != "" {
		cfg.Name = req.Name
	}
	if req.Transport != "" {
		cfg.Transport = req.Transport
	}
	if req.Endpoint != "" {
		cfg.Endpoint = req.Endpoint
	}
	if req.Config != nil {
		cfg.Config = req.Config
	}

	return cfg, nil
}

func (s *InMemoryStorage) DeleteMCPConfig(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.mcpConfigs, id)
	delete(s.mcpTools, id)
	return nil
}

var learnings []*schema.Learning
var learningID int64

func (s *InMemoryStorage) CreateLearning(req *schema.CreateLearningRequest) (*schema.Learning, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, l := range learnings {
		if l.UserID == req.UserID && l.ErrorType == req.ErrorType {
			l.Times++
			l.Lesson = req.Lesson
			l.Fix = req.Fix
			l.RootCause = req.RootCause
			l.Context = req.Context
			l.UpdatedAt = time.Now()
			return l, nil
		}
	}
	learningID++
	now := time.Now()
	l := &schema.Learning{
		ID:        learningID,
		UserID:    req.UserID,
		ErrorType: req.ErrorType,
		Context:   req.Context,
		RootCause: req.RootCause,
		Fix:       req.Fix,
		Lesson:    req.Lesson,
		Times:     1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	learnings = append(learnings, l)
	return l, nil
}

func (s *InMemoryStorage) ListLearnings(userID *int64) ([]*schema.Learning, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*schema.Learning
	for _, l := range learnings {
		if userID == nil {
			if l.UserID == nil {
				result = append(result, l)
			}
		} else {
			if l.UserID == nil || *l.UserID == *userID {
				result = append(result, l)
			}
		}
	}
	return result, nil
}

func (s *InMemoryStorage) GetLearning(userID *int64, errorType string) (*schema.Learning, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, l := range learnings {
		if l.ErrorType == errorType && (userID == nil && l.UserID == nil || userID != nil && l.UserID != nil && *l.UserID == *userID) {
			return l, nil
		}
	}
	return nil, fmt.Errorf("learning not found")
}

func (s *InMemoryStorage) DeleteLearning(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, l := range learnings {
		if l.ID == id {
			copy(learnings[i:], learnings[i+1:])
			return nil
		}
	}
	return nil
}

func (s *InMemoryStorage) ListMCPTools(configID int64) ([]schema.MCPServer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools, ok := s.mcpTools[configID]
	if !ok || len(tools) == 0 {
		return []schema.MCPServer{}, nil
	}
	out := make([]schema.MCPServer, len(tools))
	copy(out, tools)
	return out, nil
}

func (s *InMemoryStorage) SyncMCPServer(id int64, req *schema.SyncMCPServerRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, ok := s.mcpConfigs[id]
	if !ok {
		return ErrMCPConfigNotFound
	}

	cfg.ToolCount = len(req.Tools)
	cfg.HealthStatus = "ready"
	if len(req.Tools) == 0 {
		delete(s.mcpTools, id)
	} else {
		stored := make([]schema.MCPServer, len(req.Tools))
		copy(stored, req.Tools)
		s.mcpTools[id] = stored
	}
	return nil
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func (s *InMemoryStorage) StoreMemory(ctx context.Context, agentID int64, userID, sessionID, role, content string, embedding []float32, extra map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.memoryCount++
	now := time.Now()
	s.memories = append(s.memories, model.AgentMemory{
		ID:        s.memoryCount,
		AgentID:   agentID,
		UserID:    userID,
		SessionID: sessionID,
		Role:      role,
		Content:   content,
		Extra:     extra,
		CreatedAt: now,
	})
	if sessionID != "" {
		for i := range s.chatSessions {
			if s.chatSessions[i].SessionID == sessionID {
				s.chatSessions[i].UpdatedAt = now
				if role == "user" && strings.TrimSpace(s.chatSessions[i].Title) == "" {
					if t := SessionTitleFromFirstMessage(content); t != "" {
						s.chatSessions[i].Title = t
					}
				}
				break
			}
		}
	}
	return nil
}

func (s *InMemoryStorage) SearchMemory(ctx context.Context, agentID int64, embedding []float32, limit int) ([]model.AgentMemory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type scoredMemory struct {
		memory model.AgentMemory
		score  float64
	}

	var scored []scoredMemory
	for _, m := range s.memories {
		if m.AgentID == agentID && len(m.Embedding.Slice()) > 0 {
			score := cosineSimilarity(embedding, m.Embedding.Slice())
			scored = append(scored, scoredMemory{memory: m, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	if limit > 0 && limit < len(scored) {
		scored = scored[:limit]
	}

	result := make([]model.AgentMemory, len(scored))
	for i, sm := range scored {
		result[i] = sm.memory
	}
	return result, nil
}

func (s *InMemoryStorage) StoreSemanticMemory(ctx context.Context, agentID int64, userID, content string, metadata map[string]any, embedding []float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.semanticMemoryCount++
	s.semanticMemories = append(s.semanticMemories, model.SemanticMemory{
		ID:        s.semanticMemoryCount,
		AgentID:   agentID,
		UserID:    userID,
		Content:   content,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	})
	return nil
}

func (s *InMemoryStorage) SearchSemanticMemory(ctx context.Context, agentID int64, embedding []float32, limit int) ([]model.SemanticMemory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type scoredMemory struct {
		memory model.SemanticMemory
		score  float64
	}

	var scored []scoredMemory
	for _, m := range s.semanticMemories {
		if m.AgentID == agentID && len(m.Embedding.Slice()) > 0 {
			score := cosineSimilarity(embedding, m.Embedding.Slice())
			scored = append(scored, scoredMemory{memory: m, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	if limit > 0 && limit < len(scored) {
		scored = scored[:limit]
	}

	result := make([]model.SemanticMemory, len(scored))
	for i, sm := range scored {
		result[i] = sm.memory
	}
	return result, nil
}

func (s *InMemoryStorage) GetUserProfile(ctx context.Context, userID string, agentID int64) (*model.UserProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s:%d", userID, agentID)
	p, ok := s.userProfiles[key]
	if !ok {
		return nil, fmt.Errorf("user profile not found")
	}
	return p, nil
}

func (s *InMemoryStorage) UpsertUserProfile(ctx context.Context, userID string, agentID int64, profile map[string]any, embedding []float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%d", userID, agentID)
	now := time.Now()
	existing, ok := s.userProfiles[key]
	if ok {
		existing.Profile = profile
		existing.Embedding = pgvector.NewVector(embedding)
		existing.UpdatedAt = now
	} else {
		s.userProfiles[key] = &model.UserProfile{
			ID:        s.semanticMemoryCount + 1,
			UserID:    userID,
			AgentID:   agentID,
			Profile:   profile,
			Embedding: pgvector.NewVector(embedding),
			CreatedAt: now,
			UpdatedAt: now,
		}
		s.semanticMemoryCount++
	}
	return nil
}

func (s *InMemoryStorage) SearchUserProfile(ctx context.Context, userID string, agentID int64, embedding []float32) (*model.UserProfile, error) {
	return s.GetUserProfile(ctx, userID, agentID)
}

func (s *InMemoryStorage) CreateChatSession(ctx context.Context, agentID int64, userID string, groupID int64) (*schema.ChatSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.agents[agentID]; !ok {
		return nil, ErrAgentNotFound
	}
	now := time.Now()
	sess := schema.ChatSession{
		SessionID: uuid.NewString(),
		AgentID:   agentID,
		UserID:    userID,
		Title:     "",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if groupID > 0 {
		sess.GroupID = groupID
	}
	s.chatSessions = append(s.chatSessions, sess)
	return &sess, nil
}

func (s *InMemoryStorage) GetChatSession(ctx context.Context, sessionID string) (*schema.ChatSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.chatSessions {
		if s.chatSessions[i].SessionID == sessionID {
			sess := s.chatSessions[i]
			return &sess, nil
		}
	}
	return nil, ErrSessionNotFound
}

func (s *InMemoryStorage) UpdateChatSessionTitle(ctx context.Context, sessionID, userID, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	title = strings.TrimSpace(title)
	rs := []rune(title)
	if len(rs) > 512 {
		title = string(rs[:512])
	}
	for i := range s.chatSessions {
		if s.chatSessions[i].SessionID != sessionID {
			continue
		}
		if s.chatSessions[i].UserID != userID {
			return ErrSessionForbidden
		}
		s.chatSessions[i].Title = title
		s.chatSessions[i].UpdatedAt = time.Now()
		return nil
	}
	return ErrSessionNotFound
}

func (s *InMemoryStorage) DeleteChatSession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	kept := s.chatSessions[:0]
	for _, sess := range s.chatSessions {
		if sess.SessionID == sessionID {
			found = true
			continue
		}
		kept = append(kept, sess)
	}
	if !found {
		return ErrSessionNotFound
	}
	s.chatSessions = kept

	filtered := s.memories[:0]
	for _, m := range s.memories {
		if m.SessionID != sessionID {
			filtered = append(filtered, m)
		}
	}
	s.memories = filtered
	return nil
}

func (s *InMemoryStorage) ListChatSessions(ctx context.Context, agentID int64, userID string, limit, offset int) ([]schema.ChatSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var matched []schema.ChatSession
	for _, sess := range s.chatSessions {
		if sess.AgentID != agentID {
			continue
		}
		if userID != "" && sess.UserID != userID {
			continue
		}
		matched = append(matched, sess)
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].UpdatedAt.After(matched[j].UpdatedAt)
	})
	if offset >= len(matched) {
		return []schema.ChatSession{}, nil
	}
	end := offset + limit
	if end > len(matched) {
		end = len(matched)
	}
	return matched[offset:end], nil
}

func (s *InMemoryStorage) ListRecentSessionMessages(ctx context.Context, sessionID string, limit int) ([]schema.ChatHistoryMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var msgs []schema.ChatHistoryMessage
	for _, m := range s.memories {
		if m.SessionID != sessionID {
			continue
		}
		hm := schema.ChatHistoryMessage{
			ID: m.ID, AgentID: m.AgentID, Role: m.Role, Content: m.Content, CreatedAt: m.CreatedAt,
		}
		hm.ImageURLs = extraStringSlice(m.Extra, "image_urls")
		hm.FileURLs = extraStringSlice(m.Extra, "file_urls")
		hm.ReactSteps = reactStepsFromAgentExtra(m.Extra)
		msgs = append(msgs, hm)
	}
	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].CreatedAt.Before(msgs[j].CreatedAt)
	})
	if len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}
	if msgs == nil {
		return []schema.ChatHistoryMessage{}, nil
	}
	return msgs, nil
}

func (s *InMemoryStorage) ListSessionMessagesPage(ctx context.Context, sessionID string, offset, limit int) ([]schema.ChatHistoryMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	var msgs []schema.ChatHistoryMessage
	for _, m := range s.memories {
		if m.SessionID != sessionID {
			continue
		}
		hm := schema.ChatHistoryMessage{
			ID: m.ID, AgentID: m.AgentID, Role: m.Role, Content: m.Content, CreatedAt: m.CreatedAt,
		}
		hm.ImageURLs = extraStringSlice(m.Extra, "image_urls")
		hm.FileURLs = extraStringSlice(m.Extra, "file_urls")
		hm.ReactSteps = reactStepsFromAgentExtra(m.Extra)
		msgs = append(msgs, hm)
	}
	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].CreatedAt.Before(msgs[j].CreatedAt)
	})
	if offset >= len(msgs) {
		return []schema.ChatHistoryMessage{}, nil
	}
	end := offset + limit
	if end > len(msgs) {
		end = len(msgs)
	}
	return msgs[offset:end], nil
}

func (s *InMemoryStorage) ListWorkflows(ctx context.Context) ([]schema.AgentWorkflow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]schema.AgentWorkflow, 0, len(s.workflows))
	for _, w := range s.workflows {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *InMemoryStorage) GetWorkflow(ctx context.Context, id int64) (*schema.AgentWorkflow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w, ok := s.workflows[id]
	if !ok {
		return nil, ErrWorkflowNotFound
	}
	return &w, nil
}

func (s *InMemoryStorage) GetWorkflowByKey(ctx context.Context, key string) (*schema.AgentWorkflow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.workflowKeyToID[key]
	if !ok {
		return nil, ErrWorkflowNotFound
	}
	w := s.workflows[id]
	return &w, nil
}

func (s *InMemoryStorage) CreateWorkflow(ctx context.Context, req *schema.CreateWorkflowRequest) (*schema.AgentWorkflow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.workflowKeyToID[req.Key]; exists {
		return nil, fmt.Errorf("workflow key already exists: %s", req.Key)
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	s.workflowCount++
	now := time.Now()
	w := schema.AgentWorkflow{
		ID:           s.workflowCount,
		Key:          req.Key,
		Name:         req.Name,
		Description:  req.Description,
		Kind:         req.Kind,
		StepAgentIDs: append([]int64(nil), req.StepAgentIDs...),
		Config:       req.Config,
		IsActive:     active,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.workflows[w.ID] = w
	s.workflowKeyToID[w.Key] = w.ID
	return &w, nil
}

func (s *InMemoryStorage) UpdateWorkflow(ctx context.Context, id int64, req *schema.UpdateWorkflowRequest) (*schema.AgentWorkflow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cur, ok := s.workflows[id]
	if !ok {
		return nil, ErrWorkflowNotFound
	}
	if req.Name != "" {
		cur.Name = req.Name
	}
	if req.Description != "" {
		cur.Description = req.Description
	}
	if req.Kind != "" {
		cur.Kind = req.Kind
	}
	if req.StepAgentIDs != nil {
		cur.StepAgentIDs = append([]int64(nil), req.StepAgentIDs...)
	}
	if req.Config != nil {
		cur.Config = req.Config
	}
	if req.IsActive != nil {
		cur.IsActive = *req.IsActive
	}
	cur.UpdatedAt = time.Now()
	s.workflows[id] = cur
	return &cur, nil
}

func (s *InMemoryStorage) DeleteWorkflow(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	w, ok := s.workflows[id]
	if !ok {
		return ErrWorkflowNotFound
	}
	delete(s.workflowKeyToID, w.Key)
	delete(s.workflows, id)
	return nil
}

func (s *InMemoryStorage) ListChannels() ([]*model.Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Channel, 0, len(s.channels))
	for _, ch := range s.channels {
		cp := *ch
		out = append(out, &cp)
	}
	return out, nil
}

func (s *InMemoryStorage) GetChannel(id int64) (*model.Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ch, ok := s.channels[id]
	if !ok {
		return nil, ErrChannelNotFound
	}
	cp := *ch
	return &cp, nil
}

func (s *InMemoryStorage) CreateChannel(req *schema.CreateChannelRequest) (*model.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.channelCount++
	now := time.Now()
	ch := &model.Channel{
		ID:         s.channelCount,
		Name:       req.Name,
		Kind:       req.Kind,
		WebhookURL: req.WebhookURL,
		AppID:      req.AppID,
		AppSecret:  req.AppSecret,
		Extra:      cloneStringMap(req.Extra),
		IsActive:   req.IsActive,
		CreatedAt:  now,
	}
	s.channels[ch.ID] = ch
	cp := *ch
	return &cp, nil
}

func (s *InMemoryStorage) UpdateChannel(id int64, req *schema.UpdateChannelRequest) (*model.Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch, ok := s.channels[id]
	if !ok {
		return nil, ErrChannelNotFound
	}
	if req.Name != "" {
		ch.Name = req.Name
	}
	if req.WebhookURL != "" {
		ch.WebhookURL = req.WebhookURL
	}
	if req.AppID != "" {
		ch.AppID = req.AppID
	}
	if req.AppSecret != "" {
		ch.AppSecret = req.AppSecret
	}
	if req.Extra != nil {
		ch.Extra = cloneStringMap(req.Extra)
	}
	if req.IsActive != nil {
		ch.IsActive = *req.IsActive
	}
	cp := *ch
	return &cp, nil
}

func (s *InMemoryStorage) DeleteChannel(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.channels[id]; !ok {
		return ErrChannelNotFound
	}
	delete(s.channels, id)
	return nil
}

func cloneStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// RBAC

func (s *InMemoryStorage) ListRoles() ([]model.Role, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	roleUserCount := make(map[int64]int64)
	for _, ur := range s.userRoles {
		if ur.IsActive {
			roleUserCount[ur.RoleID]++
		}
	}

	roleAgentCount := make(map[int64]int64)
	if s.roleAgentPermissions != nil {
		for rid, m := range s.roleAgentPermissions {
			roleAgentCount[rid] = int64(len(m))
		}
	}

	roles := make([]model.Role, 0, len(s.roles))
	for _, r := range s.roles {
		role := *r
		role.UserCount = roleUserCount[r.ID]
		role.AgentCount = roleAgentCount[r.ID]
		roles = append(roles, role)
	}
	return roles, nil
}

func (s *InMemoryStorage) GetRole(id int64) (*model.Role, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.roles {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, fmt.Errorf("role not found")
}

func (s *InMemoryStorage) CreateRole(role *model.Role) (*model.Role, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	role.ID = s.roleCount + 1
	s.roles = append(s.roles, role)
	s.roleCount++
	return role, nil
}

func (s *InMemoryStorage) UpdateRole(id int64, role *model.Role) (*model.Role, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, r := range s.roles {
		if r.ID == id {
			s.roles[i] = role
			role.ID = id
			return role, nil
		}
	}
	return nil, fmt.Errorf("role not found")
}

func (s *InMemoryStorage) DeleteRole(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, r := range s.roles {
		if r.ID == id && !r.IsSystem {
			s.roles = append(s.roles[:i], s.roles[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("role not found or is system role")
}

func (s *InMemoryStorage) GetRoleAgentPermissions(ctx context.Context, roleID int64) (map[int64]bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[int64]bool)
	if s.roleAgentPermissions != nil {
		for agentID := range s.roleAgentPermissions[roleID] {
			result[agentID] = true
		}
	}
	return result, nil
}

func (s *InMemoryStorage) SetRoleAgentPermissions(ctx context.Context, roleID int64, agentIDs []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.roleAgentPermissions == nil {
		s.roleAgentPermissions = make(map[int64]map[int64]bool)
	}

	rolePerms := make(map[int64]bool)
	for _, agentID := range agentIDs {
		rolePerms[agentID] = true
	}
	s.roleAgentPermissions[roleID] = rolePerms
	return nil
}

func (s *InMemoryStorage) GetUserAgentPermissions(ctx context.Context, userID int64) (map[int64]bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[int64]bool)
	for _, ur := range s.userRoles {
		if ur.UserID == userID && ur.IsActive {
			if ur.ExpiresAt == nil || ur.ExpiresAt.After(time.Now()) {
				for agentID := range s.roleAgentPermissions[ur.RoleID] {
					result[agentID] = true
				}
			}
		}
	}
	return result, nil
}

func (s *InMemoryStorage) ListUserRoles(userID int64) ([]model.UserRole, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var urs []model.UserRole
	for _, ur := range s.userRoles {
		if ur.UserID == userID {
			if ur.ExpiresAt == nil || ur.ExpiresAt.After(time.Now()) {
				urs = append(urs, *ur)
			}
		}
	}
	return urs, nil
}

func (s *InMemoryStorage) GetUserRolesByRoleID(ctx context.Context, roleID int64) ([]model.UserRole, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var urs []model.UserRole
	for _, ur := range s.userRoles {
		if ur.RoleID == roleID && ur.IsActive {
			if ur.ExpiresAt == nil || ur.ExpiresAt.After(time.Now()) {
				urs = append(urs, *ur)
			}
		}
	}
	return urs, nil
}

func (s *InMemoryStorage) AssignRole(userID int64, roleID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ur := range s.userRoles {
		if ur.UserID == userID && ur.RoleID == roleID {
			ur.IsActive = true
			return nil
		}
	}
	ur := &model.UserRole{
		ID:       s.userRoleCount + 1,
		UserID:   userID,
		RoleID:   roleID,
		IsActive: true,
	}
	s.userRoles = append(s.userRoles, ur)
	s.userRoleCount++
	return nil
}

func (s *InMemoryStorage) RevokeRole(userID int64, roleID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ur := range s.userRoles {
		if ur.UserID == userID && ur.RoleID == roleID {
			ur.IsActive = false
			return nil
		}
	}
	return fmt.Errorf("user role not found")
}

func (s *InMemoryStorage) GetUserPermissions(ctx context.Context, userID int64) ([]model.Permission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type permKey struct {
		resourceType string
		resourceName string
		actions      model.Action
	}
	permMap := make(map[permKey]model.Permission)

	for _, ur := range s.userRoles {
		if ur.UserID == userID && ur.IsActive {
			if ur.ExpiresAt != nil && ur.ExpiresAt.Before(time.Now()) {
				continue
			}
			for _, r := range s.roles {
				if r.ID == ur.RoleID {
					for _, p := range r.Permissions {
						key := permKey{p.ResourceType, p.ResourceName, p.Actions}
						permMap[key] = p
					}
				}
			}
		}
	}

	perms := make([]model.Permission, 0, len(permMap))
	for _, p := range permMap {
		perms = append(perms, p)
	}
	return perms, nil
}

func (s *InMemoryStorage) GetChatStats(ctx context.Context, userID string, isAdmin bool) (map[string]int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agentSet := make(map[int64]bool)
	var totalSessions int
	for _, sess := range s.chatSessions {
		if sess.UserID == userID {
			agentSet[sess.AgentID] = true
			totalSessions++
		}
	}

	var totalAgents int64
	if isAdmin {
		totalAgents = int64(len(s.agents))
	} else {
		uid, err := strconv.ParseInt(userID, 10, 64)
		if err != nil {
			totalAgents = 0
		} else {
			accessibleAgents, err := s.GetUserAgentPermissions(ctx, uid)
			if err != nil {
				totalAgents = 0
			} else {
				totalAgents = int64(len(accessibleAgents))
			}
		}
	}

	return map[string]int64{
		"total_chats":    int64(len(agentSet)),
		"total_sessions": int64(totalSessions),
		"total_messages": int64(len(s.memories)),
		"total_agents":   totalAgents,
	}, nil
}

func (s *InMemoryStorage) GetRecentChats(ctx context.Context, userID string, limit int) ([]map[string]any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > 50 {
		limit = 10
	}

	type sessionWithAgent struct {
		session   schema.ChatSession
		agentName string
	}

	var matched []sessionWithAgent
	for _, sess := range s.chatSessions {
		if sess.UserID == userID {
			agentName := ""
			if a, ok := s.agents[sess.AgentID]; ok {
				agentName = a.Name
			}
			matched = append(matched, sessionWithAgent{sess, agentName})
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].session.UpdatedAt.After(matched[j].session.UpdatedAt)
	})

	if len(matched) > limit {
		matched = matched[:limit]
	}

	results := make([]map[string]any, 0, len(matched))
	for _, m := range matched {
		agentPublicID := ""
		if ag, ok := s.agents[m.session.AgentID]; ok {
			agentPublicID = ag.PublicID
		}
		results = append(results, map[string]any{
			"session_id":      m.session.SessionID,
			"agent_id":        m.session.AgentID,
			"agent_public_id": agentPublicID,
			"agent_name":      m.agentName,
			"updated_at":      m.session.UpdatedAt.Format("2006-01-02 15:04:05"),
			"title":           m.session.Title,
			"is_active":       true,
		})
	}
	return results, nil
}

func (s *InMemoryStorage) GetChatActivity(ctx context.Context, userID string, days int) ([]map[string]any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if days <= 0 || days > 30 {
		days = 7
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	activityMap := make(map[string]int64)

	for _, sess := range s.chatSessions {
		if sess.UserID == userID && sess.CreatedAt.After(cutoff) {
			date := sess.CreatedAt.Format("01-02")
			activityMap[date]++
		}
	}

	results := make([]map[string]any, 0, len(activityMap))
	for date, count := range activityMap {
		results = append(results, map[string]any{
			"date":  date,
			"count": count,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i]["date"].(string) < results[j]["date"].(string)
	})

	return results, nil
}

func (s *InMemoryStorage) CreateMessage(ctx context.Context, msg *model.AgentMessage) (*model.AgentMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.agentMessageCount++
	msg.ID = s.agentMessageCount
	msg.CreatedAt = time.Now()
	s.agentMessages = append(s.agentMessages, msg)
	cp := *msg
	return &cp, nil
}

func (s *InMemoryStorage) ListMessages(ctx context.Context, req *schema.ListMessagesRequest) ([]*model.AgentMessage, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []*model.AgentMessage
	for _, msg := range s.agentMessages {
		if req.ChannelID > 0 && msg.ChannelID != req.ChannelID {
			continue
		}
		if req.AgentID > 0 && msg.FromAgentID != req.AgentID && msg.ToAgentID != req.AgentID {
			continue
		}
		if req.SessionID != "" && msg.SessionID != req.SessionID {
			continue
		}
		if req.Status != "" && msg.Status != req.Status {
			continue
		}
		filtered = append(filtered, msg)
	}

	total := int64(len(filtered))

	offset := req.Offset
	if offset < 0 {
		offset = 0
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if offset >= len(filtered) {
		return []*model.AgentMessage{}, total, nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}

	result := make([]*model.AgentMessage, 0, end-offset)
	for _, msg := range filtered[offset:end] {
		cp := *msg
		result = append(result, &cp)
	}
	return result, total, nil
}

func (s *InMemoryStorage) UpdateMessageStatus(ctx context.Context, id int64, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, msg := range s.agentMessages {
		if msg.ID == id {
			msg.Status = status
			if status == "delivered" && msg.DeliveredAt == nil {
				now := time.Now()
				msg.DeliveredAt = &now
			}
			return nil
		}
	}
	return fmt.Errorf("message not found: %d", id)
}

func (s *InMemoryStorage) CreateMessageChannel(ctx context.Context, req *schema.CreateMessageChannelRequest) (*model.MessageChannel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messageChannelCount++
	now := time.Now()
	ch := &model.MessageChannel{
		ID:          s.messageChannelCount,
		Name:        req.Name,
		AgentID:     req.AgentID,
		Kind:        req.Kind,
		Description: req.Description,
		IsPublic:    req.IsPublic,
		Metadata:    cloneStringMap(req.Metadata),
		IsActive:    req.IsActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.messageChannels[ch.ID] = ch
	cp := *ch
	return &cp, nil
}

func (s *InMemoryStorage) GetMessageChannel(ctx context.Context, id int64) (*model.MessageChannel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ch, ok := s.messageChannels[id]
	if !ok {
		return nil, fmt.Errorf("message channel not found: %d", id)
	}
	cp := *ch
	return &cp, nil
}

func (s *InMemoryStorage) ListMessageChannels(ctx context.Context, agentID int64) ([]*model.MessageChannel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*model.MessageChannel
	for _, ch := range s.messageChannels {
		if agentID > 0 && ch.AgentID != agentID {
			continue
		}
		cp := *ch
		result = append(result, &cp)
	}
	return result, nil
}

func (s *InMemoryStorage) UpdateMessageChannel(ctx context.Context, id int64, req *schema.UpdateMessageChannelRequest) (*model.MessageChannel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch, ok := s.messageChannels[id]
	if !ok {
		return nil, fmt.Errorf("message channel not found: %d", id)
	}

	if req.Name != nil {
		ch.Name = *req.Name
	}
	if req.Description != nil {
		ch.Description = *req.Description
	}
	if req.IsPublic != nil {
		ch.IsPublic = *req.IsPublic
	}
	if req.Metadata != nil {
		ch.Metadata = cloneStringMap(req.Metadata)
	}
	if req.IsActive != nil {
		ch.IsActive = *req.IsActive
	}
	ch.UpdatedAt = time.Now()

	cp := *ch
	return &cp, nil
}

func (s *InMemoryStorage) DeleteMessageChannel(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.messageChannels[id]; !ok {
		return fmt.Errorf("message channel not found: %d", id)
	}
	delete(s.messageChannels, id)
	return nil
}

func (s *InMemoryStorage) CreateA2ACard(ctx context.Context, req *schema.CreateA2ACardRequest) (*model.A2ACard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.a2aCardCount++
	now := time.Now()
	card := &model.A2ACard{
		ID:           s.a2aCardCount,
		AgentID:      req.AgentID,
		Name:         req.Name,
		Description:  req.Description,
		URL:          req.URL,
		Version:      req.Version,
		Capabilities: req.Capabilities,
		IsActive:     req.IsActive,
		CreatedAt:    now,
	}
	s.a2aCards[card.ID] = card
	cp := *card
	return &cp, nil
}

func (s *InMemoryStorage) ListA2ACards(ctx context.Context, agentID int64) ([]*model.A2ACard, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*model.A2ACard
	for _, card := range s.a2aCards {
		if agentID > 0 && card.AgentID != agentID {
			continue
		}
		cp := *card
		result = append(result, &cp)
	}
	return result, nil
}

func (s *InMemoryStorage) GetA2ACard(ctx context.Context, id int64) (*model.A2ACard, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	card, ok := s.a2aCards[id]
	if !ok {
		return nil, fmt.Errorf("a2a card not found: %d", id)
	}
	cp := *card
	return &cp, nil
}

func (s *InMemoryStorage) DeleteA2ACard(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.a2aCards[id]; !ok {
		return fmt.Errorf("a2a card not found: %d", id)
	}
	delete(s.a2aCards, id)
	return nil
}

func (s *InMemoryStorage) GetWorkflowDefinition(ctx context.Context, id int64) (*model.WorkflowDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	def, ok := s.workflowDefs[id]
	if !ok {
		return nil, fmt.Errorf("workflow definition not found: %d", id)
	}
	cp := *def
	return &cp, nil
}

func (s *InMemoryStorage) GetWorkflowDefinitionByKey(ctx context.Context, key string) (*model.WorkflowDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.workflowDefKeyToID[key]
	if !ok {
		return nil, fmt.Errorf("workflow definition not found: %s", key)
	}
	def := s.workflowDefs[id]
	cp := *def
	return &cp, nil
}

func (s *InMemoryStorage) ListWorkflowDefinitions(ctx context.Context) ([]*model.WorkflowDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*model.WorkflowDefinition, 0, len(s.workflowDefs))
	for _, def := range s.workflowDefs {
		cp := *def
		result = append(result, &cp)
	}
	return result, nil
}

func (s *InMemoryStorage) CreateWorkflowDefinition(ctx context.Context, def *model.WorkflowDefinition) (*model.WorkflowDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.workflowDefKeyToID[def.Key]; exists {
		return nil, fmt.Errorf("workflow key already exists: %s", def.Key)
	}

	s.workflowDefCount++
	now := time.Now()
	def.ID = s.workflowDefCount
	def.Version = 1
	def.CreatedAt = now
	def.UpdatedAt = now

	s.workflowDefs[def.ID] = def
	s.workflowDefKeyToID[def.Key] = def.ID

	cp := *def
	return &cp, nil
}

func (s *InMemoryStorage) UpdateWorkflowDefinition(ctx context.Context, id int64, def *model.WorkflowDefinition) (*model.WorkflowDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.workflowDefs[id]
	if !ok {
		return nil, fmt.Errorf("workflow definition not found: %d", id)
	}

	if def.Name != "" {
		existing.Name = def.Name
	}
	if def.Description != "" {
		existing.Description = def.Description
	}
	if def.Kind != "" {
		existing.Kind = def.Kind
	}
	if def.Nodes != nil {
		existing.Nodes = def.Nodes
	}
	if def.Edges != nil {
		existing.Edges = def.Edges
	}
	if def.Variables != nil {
		existing.Variables = def.Variables
	}
	if def.InputSchema != nil {
		existing.InputSchema = def.InputSchema
	}
	if def.OutputSchema != nil {
		existing.OutputSchema = def.OutputSchema
	}
	existing.Version++
	existing.UpdatedAt = time.Now()

	cp := *existing
	return &cp, nil
}

func (s *InMemoryStorage) DeleteWorkflowDefinition(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	def, ok := s.workflowDefs[id]
	if !ok {
		return fmt.Errorf("workflow definition not found: %d", id)
	}

	delete(s.workflowDefKeyToID, def.Key)
	delete(s.workflowDefs, id)
	return nil
}

func (s *InMemoryStorage) CreateWorkflowExecution(ctx context.Context, exec *model.WorkflowExecution) (*model.WorkflowExecution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workflowExecCount++
	exec.ID = s.workflowExecCount
	s.workflowExecutions = append(s.workflowExecutions, exec)
	return exec, nil
}

func (s *InMemoryStorage) UpdateWorkflowExecution(ctx context.Context, id int64, exec *model.WorkflowExecution) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.workflowExecutions {
		if e.ID == id {
			s.workflowExecutions[i] = exec
			return nil
		}
	}
	return fmt.Errorf("workflow execution not found: %d", id)
}

func (s *InMemoryStorage) ListWorkflowExecutions(ctx context.Context, workflowID int64, limit int) ([]*model.WorkflowExecution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.WorkflowExecution
	for _, e := range s.workflowExecutions {
		if e.WorkflowID == workflowID {
			cp := *e
			result = append(result, &cp)
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *InMemoryStorage) GetWorkflowExecution(ctx context.Context, id int64) (*model.WorkflowExecution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.workflowExecutions {
		if e.ID == id {
			cp := *e
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("workflow execution not found: %d", id)
}

func (s *InMemoryStorage) ListSchedules() ([]*model.Schedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*model.Schedule, 0)
	for _, sch := range s.schedules {
		cp := *sch
		result = append(result, &cp)
	}
	return result, nil
}

func (s *InMemoryStorage) GetSchedule(id int64) (*model.Schedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sch, ok := s.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule not found: %d", id)
	}
	cp := *sch
	return &cp, nil
}

func (s *InMemoryStorage) CreateSchedule(schedule *model.Schedule) (*model.Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.scheduleCount++
	schedule.ID = s.scheduleCount
	schedule.CreatedAt = time.Now()
	schedule.UpdatedAt = time.Now()
	s.schedules[schedule.ID] = schedule
	cp := *schedule
	return &cp, nil
}

func (s *InMemoryStorage) UpdateSchedule(id int64, schedule *model.Schedule) (*model.Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule not found: %d", id)
	}

	if schedule.Name != "" {
		existing.Name = schedule.Name
	}
	if schedule.Description != "" {
		existing.Description = schedule.Description
	}
	if schedule.AgentID > 0 {
		existing.AgentID = schedule.AgentID
	}
	if schedule.ScheduleKind != "" {
		existing.ScheduleKind = schedule.ScheduleKind
	}
	if schedule.CronExpr != "" {
		existing.CronExpr = schedule.CronExpr
	}
	if schedule.At != "" {
		existing.At = schedule.At
	}
	if schedule.EveryMs > 0 {
		existing.EveryMs = schedule.EveryMs
	}
	if schedule.Timezone != "" {
		existing.Timezone = schedule.Timezone
	}
	if schedule.WakeMode != "" {
		existing.WakeMode = schedule.WakeMode
	}
	if schedule.SessionTarget != "" {
		existing.SessionTarget = schedule.SessionTarget
	}
	if schedule.Prompt != "" {
		existing.Prompt = schedule.Prompt
	}
	if schedule.StaggerMs > 0 {
		existing.StaggerMs = schedule.StaggerMs
	}
	existing.Enabled = schedule.Enabled
	existing.ChannelID = schedule.ChannelID
	if schedule.OwnerUserID != "" {
		existing.OwnerUserID = schedule.OwnerUserID
	}
	if schedule.ChatSessionID != "" {
		existing.ChatSessionID = schedule.ChatSessionID
	}
	existing.UpdatedAt = time.Now()

	cp := *existing
	return &cp, nil
}

func (s *InMemoryStorage) UpdateScheduleChatSessionID(id int64, chatSessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.schedules[id]
	if !ok {
		return fmt.Errorf("schedule not found: %d", id)
	}
	existing.ChatSessionID = chatSessionID
	existing.UpdatedAt = time.Now()
	return nil
}

func (s *InMemoryStorage) DeleteSchedule(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.schedules[id]; !ok {
		return fmt.Errorf("schedule not found: %d", id)
	}
	delete(s.schedules, id)
	return nil
}

func (s *InMemoryStorage) ListScheduleExecutions(scheduleID int64, limit int) ([]*model.ScheduleExecution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*model.ScheduleExecution
	for _, exec := range s.scheduleExecutions {
		if scheduleID > 0 && exec.ScheduleID != scheduleID {
			continue
		}
		cp := *exec
		result = append(result, &cp)
	}

	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

func (s *InMemoryStorage) CreateScheduleExecution(exec *model.ScheduleExecution) (*model.ScheduleExecution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.scheduleExecutionCount++
	exec.ID = s.scheduleExecutionCount
	exec.StartedAt = time.Now()
	s.scheduleExecutions = append(s.scheduleExecutions, exec)
	cp := *exec
	return &cp, nil
}

func (s *InMemoryStorage) UpdateScheduleExecution(id int64, status string, result, err string, durationMs int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, exec := range s.scheduleExecutions {
		if exec.ID == id {
			exec.Status = status
			exec.Result = result
			exec.Error = err
			exec.DurationMs = durationMs
			now := time.Now()
			exec.FinishedAt = &now
			return nil
		}
	}
	return fmt.Errorf("schedule execution not found: %d", id)
}

func (s *InMemoryStorage) CreateAuditLog(log *model.AuditLog) (*model.AuditLog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.auditLogCount++
	log.ID = s.auditLogCount
	log.CreatedAt = time.Now()
	cp := *log
	s.auditLogs = append(s.auditLogs, &cp)
	return &cp, nil
}

func auditLogMatchesFilter(log *model.AuditLog, filter *AuditLogFilter) bool {
	if filter == nil {
		return true
	}
	if filter.UserID != "" && log.UserID != filter.UserID {
		return false
	}
	if filter.AgentID != 0 && log.AgentID != filter.AgentID {
		return false
	}
	if filter.SessionID != "" && log.SessionID != filter.SessionID {
		return false
	}
	if filter.ToolName != "" && log.ToolName != filter.ToolName {
		return false
	}
	if filter.RiskLevel != "" && log.RiskLevel != filter.RiskLevel {
		return false
	}
	if filter.Status != "" && log.Status != filter.Status {
		return false
	}
	return true
}

func (s *InMemoryStorage) ListAuditLogs(filter *AuditLogFilter) ([]*model.AuditLog, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*model.AuditLog, 0, len(s.auditLogs))
	for _, log := range s.auditLogs {
		if !auditLogMatchesFilter(log, filter) {
			continue
		}
		result = append(result, log)
	}

	total := int64(len(result))

	page := 1
	pageSize := 50
	if filter != nil {
		if filter.Page > 0 {
			page = filter.Page
		}
		if filter.PageSize > 0 {
			pageSize = filter.PageSize
		}
	}

	start := (page - 1) * pageSize
	if start >= len(result) {
		return []*model.AuditLog{}, total, nil
	}
	end := start + pageSize
	if end > len(result) {
		end = len(result)
	}

	return result[start:end], total, nil
}

func (s *InMemoryStorage) GetAuditLog(id int64) (*model.AuditLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, log := range s.auditLogs {
		if log.ID == id {
			return log, nil
		}
	}
	return nil, fmt.Errorf("audit log not found: %d", id)
}

func (s *InMemoryStorage) CountAuditLogs(filter *AuditLogFilter) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var n int64
	for _, log := range s.auditLogs {
		if auditLogMatchesFilter(log, filter) {
			n++
		}
	}
	return n, nil
}

func (s *InMemoryStorage) DeleteAuditLogs(filter *AuditLogFilter) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var kept []*model.AuditLog
	var removed int64
	for _, log := range s.auditLogs {
		if auditLogMatchesFilter(log, filter) {
			removed++
			continue
		}
		kept = append(kept, log)
	}
	s.auditLogs = kept
	return removed, nil
}

func (s *InMemoryStorage) fillApprovalAgentName(req *model.ApprovalRequest) {
	if req == nil {
		return
	}
	if ag, ok := s.agents[req.AgentID]; ok && ag != nil {
		req.AgentName = ag.Agent.Name
	}
}

func (s *InMemoryStorage) CreateApprovalRequest(req *model.ApprovalRequest) (*model.ApprovalRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.approvalRequestCount++
	req.ID = s.approvalRequestCount
	req.CreatedAt = time.Now()
	if req.Status == "" {
		req.Status = "pending"
	}
	cp := *req
	s.approvalRequests = append(s.approvalRequests, &cp)
	return &cp, nil
}

func (s *InMemoryStorage) ListApprovalRequests(filter *ApprovalRequestFilter) ([]*model.ApprovalRequest, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*model.ApprovalRequest, 0, len(s.approvalRequests))
	for _, req := range s.approvalRequests {
		if filter != nil {
			if filter.AgentID != 0 && req.AgentID != filter.AgentID {
				continue
			}
			if filter.Status != "" && req.Status != filter.Status {
				continue
			}
			if filter.ExternalID != "" && req.ExternalID != filter.ExternalID {
				continue
			}
			if filter.SessionID != "" && req.SessionID != filter.SessionID {
				continue
			}
			if filter.UserID != "" && req.UserID != filter.UserID {
				continue
			}
		}
		cp := *req
		s.fillApprovalAgentName(&cp)
		result = append(result, &cp)
	}

	total := int64(len(result))
	page := 1
	pageSize := 50
	if filter != nil {
		if filter.Page > 0 {
			page = filter.Page
		}
		if filter.PageSize > 0 {
			pageSize = filter.PageSize
		}
	}

	start := (page - 1) * pageSize
	if start >= len(result) {
		return []*model.ApprovalRequest{}, total, nil
	}
	end := start + pageSize
	if end > len(result) {
		end = len(result)
	}

	return result[start:end], total, nil
}

func (s *InMemoryStorage) GetApprovalRequest(id int64) (*model.ApprovalRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, req := range s.approvalRequests {
		if req.ID == id {
			cp := *req
			s.fillApprovalAgentName(&cp)
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("approval request not found: %d", id)
}

func (s *InMemoryStorage) UpdateApprovalRequest(id int64, status, approverID, comment string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, req := range s.approvalRequests {
		if req.ID == id {
			req.Status = status
			req.ApproverID = approverID
			req.Comment = comment
			req.ApprovedAt = &now
			return nil
		}
	}
	return fmt.Errorf("approval request not found: %d", id)
}

// Chat Group operations (in-memory stubs)

func (s *InMemoryStorage) CreateChatGroup(ctx context.Context, req *schema.CreateGroupRequest, userID string) (*model.ChatGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.chatGroupCount++
	now := time.Now()
	group := &model.ChatGroup{
		ID:        s.chatGroupCount,
		Name:      req.Name,
		CreatedBy: userID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.chatGroups[group.ID] = group

	for _, agentID := range req.AgentIDs {
		name := ""
		if ag, ok := s.agents[agentID]; ok {
			name = ag.Agent.Name
		}
		member := model.ChatGroupMember{
			ID:        s.chatGroupMemberCount,
			GroupID:   group.ID,
			AgentID:   agentID,
			AgentName: name,
		}
		s.chatGroupMembers = append(s.chatGroupMembers, &member)
	}

	return group, nil
}

func (s *InMemoryStorage) GetChatGroup(ctx context.Context, id int64) (*model.ChatGroup, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	group, ok := s.chatGroups[id]
	if !ok {
		return nil, fmt.Errorf("chat group not found: %d", id)
	}

	cp := *group
	cp.UpdatedAt = groupLastActivity(s, group.ID, group.CreatedAt)
	for _, m := range s.chatGroupMembers {
		if m.GroupID == id {
			cp.Members = append(cp.Members, *m)
		}
	}
	return &cp, nil
}

func groupLastActivity(s *InMemoryStorage, groupID int64, fallback time.Time) time.Time {
	maxT := fallback
	for i := range s.chatSessions {
		if s.chatSessions[i].GroupID == groupID && s.chatSessions[i].UpdatedAt.After(maxT) {
			maxT = s.chatSessions[i].UpdatedAt
		}
	}
	return maxT
}

func (s *InMemoryStorage) ListChatGroups(ctx context.Context, userID string) ([]*model.ChatGroup, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*model.ChatGroup, 0, len(s.chatGroups))
	for _, g := range s.chatGroups {
		cp := *g
		cp.UpdatedAt = groupLastActivity(s, g.ID, g.CreatedAt)
		for _, m := range s.chatGroupMembers {
			if m.GroupID == g.ID {
				cp.Members = append(cp.Members, *m)
			}
		}
		result = append(result, &cp)
	}
	return result, nil
}

func (s *InMemoryStorage) UpdateChatGroup(ctx context.Context, id int64, req *schema.UpdateGroupRequest) (*model.ChatGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	group, ok := s.chatGroups[id]
	if !ok {
		return nil, fmt.Errorf("chat group not found: %d", id)
	}

	if req.Name != nil {
		group.Name = *req.Name
	}

	if len(req.AgentIDs) > 0 {
		// Replace members
		var kept []*model.ChatGroupMember
		for _, m := range s.chatGroupMembers {
			if m.GroupID != id {
				kept = append(kept, m)
			}
		}
		s.chatGroupMembers = kept

		for _, agentID := range req.AgentIDs {
			name := ""
			if ag, ok := s.agents[agentID]; ok {
				name = ag.Agent.Name
			}
			member := model.ChatGroupMember{
				ID:        s.chatGroupMemberCount,
				GroupID:   id,
				AgentID:   agentID,
				AgentName: name,
			}
			s.chatGroupMembers = append(s.chatGroupMembers, &member)
		}
	}

	return s.GetChatGroup(ctx, id)
}

func (s *InMemoryStorage) DeleteChatGroup(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.chatGroups[id]; !ok {
		return fmt.Errorf("chat group not found: %d", id)
	}

	delete(s.chatGroups, id)

	// Also delete members
	var kept []*model.ChatGroupMember
	for _, m := range s.chatGroupMembers {
		if m.GroupID != id {
			kept = append(kept, m)
		}
	}
	s.chatGroupMembers = kept

	return nil
}
