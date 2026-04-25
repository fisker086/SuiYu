package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/schema"
)

type MessageRouter struct {
	mu              sync.RWMutex
	channels        map[int64]*schema.MessageChannel
	agentsByName    map[string]int64
	agentsByID      map[int64]string
	subscribers     map[int64][]chan *model.AgentMessage
	messageQueue    chan *routeTask
	store           MessageStore
	agentRuntime    AgentLookup
	channelProvider ChannelProvider
	a2aCardProvider A2ACardProvider
}

type routeTask struct {
	msg    *model.AgentMessage
	ctx    context.Context
	result chan error
}

type MessageStore interface {
	CreateMessage(ctx context.Context, msg *model.AgentMessage) (*model.AgentMessage, error)
	ListMessages(ctx context.Context, req *schema.ListMessagesRequest) ([]*model.AgentMessage, int64, error)
	UpdateMessageStatus(ctx context.Context, id int64, status string) error
	CreateMessageChannel(ctx context.Context, req *schema.CreateMessageChannelRequest) (*model.MessageChannel, error)
	GetMessageChannel(ctx context.Context, id int64) (*model.MessageChannel, error)
	ListMessageChannels(ctx context.Context, agentID int64) ([]*model.MessageChannel, error)
	UpdateMessageChannel(ctx context.Context, id int64, req *schema.UpdateMessageChannelRequest) (*model.MessageChannel, error)
	DeleteMessageChannel(ctx context.Context, id int64) error
	CreateA2ACard(ctx context.Context, req *schema.CreateA2ACardRequest) (*model.A2ACard, error)
	ListA2ACards(ctx context.Context, agentID int64) ([]*model.A2ACard, error)
	GetA2ACard(ctx context.Context, id int64) (*model.A2ACard, error)
	DeleteA2ACard(ctx context.Context, id int64) error
}

type AgentLookup interface {
	GetAgent(id int64) (*schema.AgentWithRuntime, bool)
	GetAgentByName(name string) (*schema.AgentWithRuntime, bool)
	ListAgents() []*schema.AgentWithRuntime
}

type ChannelProvider interface {
	GetChannels(agentID int64) []*schema.MessageChannel
	GetDefaultChannel(agentID int64) *schema.MessageChannel
	EnsureDefaultChannel(agentID int64, agentName string) (*schema.MessageChannel, error)
}

type A2ACardProvider interface {
	GetCards(agentID int64) []*schema.A2ACard
	GetCard(agentID int64) *schema.A2ACard
	EnsureDefaultCard(agentID int64, agentName string) (*schema.A2ACard, error)
}

type defaultChannelProvider struct {
	store MessageStore
}

func NewDefaultChannelProvider(store MessageStore) ChannelProvider {
	return &defaultChannelProvider{store: store}
}

func (p *defaultChannelProvider) GetChannels(agentID int64) []*schema.MessageChannel {
	chs, err := p.store.ListMessageChannels(context.Background(), agentID)
	if err != nil || len(chs) == 0 {
		return nil
	}
	result := make([]*schema.MessageChannel, 0, len(chs))
	for _, ch := range chs {
		result = append(result, &schema.MessageChannel{
			ID:          ch.ID,
			Name:        ch.Name,
			AgentID:     ch.AgentID,
			Kind:        ch.Kind,
			Description: ch.Description,
			IsPublic:    ch.IsPublic,
			Metadata:    ch.Metadata,
			IsActive:    ch.IsActive,
			CreatedAt:   ch.CreatedAt,
			UpdatedAt:   ch.UpdatedAt,
		})
	}
	return result
}

func (p *defaultChannelProvider) GetDefaultChannel(agentID int64) *schema.MessageChannel {
	chs, err := p.store.ListMessageChannels(context.Background(), agentID)
	if err != nil || len(chs) == 0 {
		return nil
	}
	ch := chs[0]
	return &schema.MessageChannel{
		ID:          ch.ID,
		Name:        ch.Name,
		AgentID:     ch.AgentID,
		Kind:        ch.Kind,
		Description: ch.Description,
		IsPublic:    ch.IsPublic,
		Metadata:    ch.Metadata,
		IsActive:    ch.IsActive,
		CreatedAt:   ch.CreatedAt,
		UpdatedAt:   ch.UpdatedAt,
	}
}

func (p *defaultChannelProvider) EnsureDefaultChannel(agentID int64, agentName string) (*schema.MessageChannel, error) {
	chs, err := p.store.ListMessageChannels(context.Background(), agentID)
	if err == nil && len(chs) > 0 {
		ch := chs[0]
		return &schema.MessageChannel{
			ID:          ch.ID,
			Name:        ch.Name,
			AgentID:     ch.AgentID,
			Kind:        ch.Kind,
			Description: ch.Description,
			IsPublic:    ch.IsPublic,
			Metadata:    ch.Metadata,
			IsActive:    ch.IsActive,
			CreatedAt:   ch.CreatedAt,
			UpdatedAt:   ch.UpdatedAt,
		}, nil
	}

	req := &schema.CreateMessageChannelRequest{
		Name:        fmt.Sprintf("%s-default", agentName),
		AgentID:     agentID,
		Kind:        "direct",
		Description: "Default channel for agent communication",
		IsPublic:    false,
		IsActive:    true,
	}
	dbCh, err := p.store.CreateMessageChannel(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to create default channel: %w", err)
	}
	return &schema.MessageChannel{
		ID:          dbCh.ID,
		Name:        dbCh.Name,
		AgentID:     dbCh.AgentID,
		Kind:        dbCh.Kind,
		Description: dbCh.Description,
		IsPublic:    dbCh.IsPublic,
		Metadata:    dbCh.Metadata,
		IsActive:    dbCh.IsActive,
		CreatedAt:   dbCh.CreatedAt,
		UpdatedAt:   dbCh.UpdatedAt,
	}, nil
}

type defaultA2ACardProvider struct {
	store MessageStore
}

func NewDefaultA2ACardProvider(store MessageStore) A2ACardProvider {
	return &defaultA2ACardProvider{store: store}
}

func (p *defaultA2ACardProvider) GetCards(agentID int64) []*schema.A2ACard {
	cards, err := p.store.ListA2ACards(context.Background(), agentID)
	if err != nil || len(cards) == 0 {
		return nil
	}
	result := make([]*schema.A2ACard, 0, len(cards))
	for _, c := range cards {
		result = append(result, &schema.A2ACard{
			ID:           c.ID,
			AgentID:      c.AgentID,
			Name:         c.Name,
			Description:  c.Description,
			URL:          c.URL,
			Version:      c.Version,
			Capabilities: c.Capabilities,
			IsActive:     c.IsActive,
			CreatedAt:    c.CreatedAt.Format(time.RFC3339),
		})
	}
	return result
}

func (p *defaultA2ACardProvider) GetCard(agentID int64) *schema.A2ACard {
	cards, err := p.store.ListA2ACards(context.Background(), agentID)
	if err != nil || len(cards) == 0 {
		return nil
	}
	c := cards[0]
	return &schema.A2ACard{
		ID:           c.ID,
		AgentID:      c.AgentID,
		Name:         c.Name,
		Description:  c.Description,
		URL:          c.URL,
		Version:      c.Version,
		Capabilities: c.Capabilities,
		IsActive:     c.IsActive,
		CreatedAt:    c.CreatedAt.Format(time.RFC3339),
	}
}

func (p *defaultA2ACardProvider) EnsureDefaultCard(agentID int64, agentName string) (*schema.A2ACard, error) {
	cards, err := p.store.ListA2ACards(context.Background(), agentID)
	if err == nil && len(cards) > 0 {
		c := cards[0]
		return &schema.A2ACard{
			ID:           c.ID,
			AgentID:      c.AgentID,
			Name:         c.Name,
			Description:  c.Description,
			URL:          c.URL,
			Version:      c.Version,
			Capabilities: c.Capabilities,
			IsActive:     c.IsActive,
			CreatedAt:    c.CreatedAt.Format(time.RFC3339),
		}, nil
	}

	req := &schema.CreateA2ACardRequest{
		AgentID:     agentID,
		Name:        fmt.Sprintf("%s-a2a-card", agentName),
		Description: fmt.Sprintf("A2A card for agent %s", agentName),
		URL:         fmt.Sprintf("/a2a/agents/%d", agentID),
		Version:     "1.0.0",
		IsActive:    true,
	}
	card, err := p.store.CreateA2ACard(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to create default A2A card: %w", err)
	}
	return &schema.A2ACard{
		ID:           card.ID,
		AgentID:      card.AgentID,
		Name:         card.Name,
		Description:  card.Description,
		URL:          card.URL,
		Version:      card.Version,
		Capabilities: card.Capabilities,
		IsActive:     card.IsActive,
		CreatedAt:    card.CreatedAt.Format(time.RFC3339),
	}, nil
}

func NewMessageRouter(store MessageStore, agentRuntime AgentLookup, queueSize int) *MessageRouter {
	channelProvider := NewDefaultChannelProvider(store)
	a2aCardProvider := NewDefaultA2ACardProvider(store)
	mr := &MessageRouter{
		channels:        make(map[int64]*schema.MessageChannel),
		agentsByName:    make(map[string]int64),
		agentsByID:      make(map[int64]string),
		subscribers:     make(map[int64][]chan *model.AgentMessage),
		messageQueue:    make(chan *routeTask, queueSize),
		store:           store,
		agentRuntime:    agentRuntime,
		channelProvider: channelProvider,
		a2aCardProvider: a2aCardProvider,
	}

	go mr.processQueue()
	return mr
}

func (mr *MessageRouter) RegisterAgentChannel(agentID int64, channel *schema.MessageChannel) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	mr.channels[channel.ID] = channel

	if agent, ok := mr.agentRuntime.GetAgent(agentID); ok {
		mr.agentsByID[agentID] = agent.Name
		mr.agentsByName[agent.Name] = agentID
	}

	if _, exists := mr.subscribers[channel.ID]; !exists {
		mr.subscribers[channel.ID] = make([]chan *model.AgentMessage, 0)
	}

	logger.Info("registered agent channel", "agent_id", agentID, "channel_id", channel.ID, "channel_name", channel.Name)
}

func (mr *MessageRouter) Subscribe(channelID int64) (<-chan *model.AgentMessage, func()) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	ch := make(chan *model.AgentMessage, 64)
	mr.subscribers[channelID] = append(mr.subscribers[channelID], ch)

	cancel := func() {
		mr.mu.Lock()
		defer mr.mu.Unlock()
		subs := mr.subscribers[channelID]
		for i, sub := range subs {
			if sub == ch {
				close(sub)
				mr.subscribers[channelID] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
	}

	return ch, cancel
}

func (mr *MessageRouter) SendMessage(ctx context.Context, req *schema.SendMessageRequest) (*schema.MessageSendResponse, error) {
	if req.ToAgentID == 0 && req.ChannelID == 0 {
		return nil, fmt.Errorf("either to_agent_id or channel_id must be specified")
	}

	targetAgentID := req.ToAgentID
	targetChannelID := req.ChannelID

	if targetChannelID == 0 {
		channels, err := mr.store.ListMessageChannels(context.Background(), targetAgentID)
		if err != nil || len(channels) == 0 {
			return nil, fmt.Errorf("no channel found for agent %d", targetAgentID)
		}
		targetChannelID = channels[0].ID
	}

	if targetAgentID == 0 {
		if ch, ok := mr.channels[targetChannelID]; ok {
			targetAgentID = ch.AgentID
		} else {
			dbCh, err := mr.store.GetMessageChannel(context.Background(), targetChannelID)
			if err != nil {
				return nil, fmt.Errorf("channel not found: %d", targetChannelID)
			}
			targetAgentID = dbCh.AgentID
		}
	}

	msg := &model.AgentMessage{
		FromAgentID: req.FromAgentID,
		ToAgentID:   targetAgentID,
		ChannelID:   targetChannelID,
		SessionID:   req.SessionID,
		Kind:        req.Kind,
		Content:     req.Content,
		Metadata:    req.Metadata,
		Status:      "pending",
		Priority:    req.Priority,
		CreatedAt:   time.Now(),
	}

	storedMsg, err := mr.store.CreateMessage(context.Background(), msg)
	if err != nil {
		return nil, fmt.Errorf("failed to store message: %w", err)
	}

	task := &routeTask{
		msg:    storedMsg,
		ctx:    ctx,
		result: make(chan error, 1),
	}

	select {
	case mr.messageQueue <- task:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	err = <-task.result
	if err != nil {
		return nil, err
	}

	now := time.Now()
	resp := &schema.MessageSendResponse{
		MessageID:   storedMsg.ID,
		Status:      "delivered",
		DeliveredAt: now.Format(time.RFC3339),
	}

	return resp, nil
}

func (mr *MessageRouter) SendSpan(ctx context.Context, req *schema.MessageSpanRequest) (*schema.MessageSpanResponse, error) {
	spanID := fmt.Sprintf("span_%d_%d_%d", req.FromAgentID, req.ToAgentID, time.Now().UnixNano())
	traceID := fmt.Sprintf("trace_%d", time.Now().UnixNano())

	msgReq := &schema.SendMessageRequest{
		FromAgentID: req.FromAgentID,
		ToAgentID:   req.ToAgentID,
		ChannelID:   req.ChannelID,
		SessionID:   req.SessionID,
		Kind:        "event",
		Content:     req.Content,
		Metadata: map[string]any{
			"span_id":   spanID,
			"trace_id":  traceID,
			"span_type": "message_span",
		},
		Priority: 0,
	}

	if req.Metadata != nil {
		for k, v := range req.Metadata {
			msgReq.Metadata[k] = v
		}
	}

	sendResp, err := mr.SendMessage(ctx, msgReq)
	if err != nil {
		return nil, err
	}

	return &schema.MessageSpanResponse{
		SpanID:    spanID,
		MessageID: sendResp.MessageID,
		Status:    sendResp.Status,
		TraceID:   traceID,
	}, nil
}

func (mr *MessageRouter) ListMessages(req *schema.ListMessagesRequest) ([]*schema.AgentMessage, int64, error) {
	if req.Limit <= 0 {
		req.Limit = 50
	}

	messages, total, err := mr.store.ListMessages(context.Background(), req)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*schema.AgentMessage, 0, len(messages))
	for _, msg := range messages {
		schemaMsg := &schema.AgentMessage{
			ID:          msg.ID,
			FromAgentID: msg.FromAgentID,
			ToAgentID:   msg.ToAgentID,
			ChannelID:   msg.ChannelID,
			SessionID:   msg.SessionID,
			Kind:        msg.Kind,
			Content:     msg.Content,
			Metadata:    msg.Metadata,
			Status:      msg.Status,
			Priority:    msg.Priority,
			CreatedAt:   msg.CreatedAt,
			DeliveredAt: msg.DeliveredAt,
		}

		if name, ok := mr.agentsByID[msg.FromAgentID]; ok {
			schemaMsg.FromAgentName = name
		}
		if name, ok := mr.agentsByID[msg.ToAgentID]; ok {
			schemaMsg.ToAgentName = name
		}

		result = append(result, schemaMsg)
	}

	return result, total, nil
}

func (mr *MessageRouter) CreateChannel(req *schema.CreateMessageChannelRequest) (*schema.MessageChannel, error) {
	dbCh, err := mr.store.CreateMessageChannel(context.Background(), req)
	if err != nil {
		return nil, err
	}

	schemaCh := &schema.MessageChannel{
		ID:          dbCh.ID,
		Name:        dbCh.Name,
		AgentID:     dbCh.AgentID,
		Kind:        dbCh.Kind,
		Description: dbCh.Description,
		IsPublic:    dbCh.IsPublic,
		Metadata:    dbCh.Metadata,
		IsActive:    dbCh.IsActive,
		CreatedAt:   dbCh.CreatedAt,
		UpdatedAt:   dbCh.UpdatedAt,
	}

	if agent, ok := mr.agentRuntime.GetAgent(req.AgentID); ok {
		schemaCh.AgentName = agent.Name
	}

	mr.RegisterAgentChannel(req.AgentID, schemaCh)

	return schemaCh, nil
}

func (mr *MessageRouter) ListChannels(agentID int64) ([]*schema.MessageChannel, error) {
	channels, err := mr.store.ListMessageChannels(context.Background(), agentID)
	if err != nil {
		return nil, err
	}

	result := make([]*schema.MessageChannel, 0, len(channels))
	for _, ch := range channels {
		schemaCh := &schema.MessageChannel{
			ID:          ch.ID,
			Name:        ch.Name,
			AgentID:     ch.AgentID,
			Kind:        ch.Kind,
			Description: ch.Description,
			IsPublic:    ch.IsPublic,
			Metadata:    ch.Metadata,
			IsActive:    ch.IsActive,
			CreatedAt:   ch.CreatedAt,
			UpdatedAt:   ch.UpdatedAt,
		}

		if agent, ok := mr.agentRuntime.GetAgent(ch.AgentID); ok {
			schemaCh.AgentName = agent.Name
		}

		result = append(result, schemaCh)
	}

	return result, nil
}

func (mr *MessageRouter) UpdateChannel(id int64, req *schema.UpdateMessageChannelRequest) (*schema.MessageChannel, error) {
	dbCh, err := mr.store.UpdateMessageChannel(context.Background(), id, req)
	if err != nil {
		return nil, err
	}

	schemaCh := &schema.MessageChannel{
		ID:          dbCh.ID,
		Name:        dbCh.Name,
		AgentID:     dbCh.AgentID,
		Kind:        dbCh.Kind,
		Description: dbCh.Description,
		IsPublic:    dbCh.IsPublic,
		Metadata:    dbCh.Metadata,
		IsActive:    dbCh.IsActive,
		CreatedAt:   dbCh.CreatedAt,
		UpdatedAt:   dbCh.UpdatedAt,
	}

	if agent, ok := mr.agentRuntime.GetAgent(dbCh.AgentID); ok {
		schemaCh.AgentName = agent.Name
	}

	mr.mu.Lock()
	mr.channels[id] = schemaCh
	mr.mu.Unlock()

	return schemaCh, nil
}

func (mr *MessageRouter) DeleteChannel(id int64) error {
	return mr.store.DeleteMessageChannel(context.Background(), id)
}

func (mr *MessageRouter) CreateA2ACard(req *schema.CreateA2ACardRequest) (*schema.A2ACard, error) {
	card, err := mr.store.CreateA2ACard(context.Background(), req)
	if err != nil {
		return nil, err
	}

	return &schema.A2ACard{
		ID:           card.ID,
		AgentID:      card.AgentID,
		Name:         card.Name,
		Description:  card.Description,
		URL:          card.URL,
		Version:      card.Version,
		Capabilities: card.Capabilities,
		IsActive:     card.IsActive,
		CreatedAt:    card.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (mr *MessageRouter) ListA2ACards(agentID int64) ([]*schema.A2ACard, error) {
	cards, err := mr.store.ListA2ACards(context.Background(), agentID)
	if err != nil {
		return nil, err
	}

	result := make([]*schema.A2ACard, 0, len(cards))
	for _, card := range cards {
		result = append(result, &schema.A2ACard{
			ID:           card.ID,
			AgentID:      card.AgentID,
			Name:         card.Name,
			Description:  card.Description,
			URL:          card.URL,
			Version:      card.Version,
			Capabilities: card.Capabilities,
			IsActive:     card.IsActive,
			CreatedAt:    card.CreatedAt.Format(time.RFC3339),
		})
	}

	return result, nil
}

func (mr *MessageRouter) GetA2ACard(id int64) (*schema.A2ACard, error) {
	card, err := mr.store.GetA2ACard(context.Background(), id)
	if err != nil {
		return nil, err
	}

	return &schema.A2ACard{
		ID:           card.ID,
		AgentID:      card.AgentID,
		Name:         card.Name,
		Description:  card.Description,
		URL:          card.URL,
		Version:      card.Version,
		Capabilities: card.Capabilities,
		IsActive:     card.IsActive,
		CreatedAt:    card.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (mr *MessageRouter) DeleteA2ACard(id int64) error {
	return mr.store.DeleteA2ACard(context.Background(), id)
}

func (mr *MessageRouter) GetAgentChannels(agentID int64) []*schema.MessageChannel {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	result := make([]*schema.MessageChannel, 0)
	for _, ch := range mr.channels {
		if ch.AgentID == agentID {
			result = append(result, ch)
		}
	}

	return result
}

func (mr *MessageRouter) GetAgentIDByName(name string) (int64, bool) {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	id, ok := mr.agentsByName[name]
	return id, ok
}

func (mr *MessageRouter) InitAgents() {
	agents := mr.agentRuntime.ListAgents()
	for _, agent := range agents {
		channel, err := mr.channelProvider.EnsureDefaultChannel(agent.ID, agent.Name)
		if err != nil {
			logger.Warn("failed to ensure default channel", "agent_id", agent.ID, "agent_name", agent.Name, "err", err)
		} else {
			mr.RegisterAgentChannel(agent.ID, channel)
		}

		_, err = mr.a2aCardProvider.EnsureDefaultCard(agent.ID, agent.Name)
		if err != nil {
			logger.Warn("failed to ensure default A2A card", "agent_id", agent.ID, "agent_name", agent.Name, "err", err)
		}
	}
	logger.Info("initialized agent channels and A2A cards", "count", len(agents))
}

func (mr *MessageRouter) processQueue() {
	for task := range mr.messageQueue {
		err := mr.deliverMessage(task.ctx, task.msg)
		task.result <- err
	}
}

func (mr *MessageRouter) deliverMessage(ctx context.Context, msg *model.AgentMessage) error {
	mr.mu.RLock()
	subs := mr.subscribers[msg.ChannelID]
	mr.mu.RUnlock()

	if len(subs) == 0 {
		logger.Debug("no subscribers for channel", "channel_id", msg.ChannelID)
	}

	now := time.Now()
	msg.DeliveredAt = &now
	msg.Status = "delivered"

	if err := mr.store.UpdateMessageStatus(ctx, msg.ID, "delivered"); err != nil {
		logger.Warn("failed to update message status", "message_id", msg.ID, "err", err)
	}

	for _, sub := range subs {
		select {
		case sub <- msg:
		case <-ctx.Done():
			return ctx.Err()
		default:
			logger.Warn("subscriber channel full, dropping message", "channel_id", msg.ChannelID)
		}
	}

	return nil
}
