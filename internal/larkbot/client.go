package larkbot

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fisk086/sya/internal/agent"
	"github.com/fisk086/sya/internal/logger"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/larksuite/oapi-sdk-go/v3/ws"
)

type BotConfig struct {
	AppID         string
	AppSecret     string
	AgentID       int64
	InvokeTimeout int
	// OpenAPIDomain is the Open Platform API base (e.g. https://open.feishu.cn or https://open.larksuite.com).
	OpenAPIDomain string
	// NoAutoStartWS when true, Start() (server boot / global start) skips opening WebSocket for this app;
	// StartBot may still be used from the API for manual per-agent start.
	NoAutoStartWS bool
}

type BotEntry struct {
	Config    *BotConfig
	Runtime   *agent.Runtime
	wsClient  *ws.Client
	wsCancel  context.CancelFunc
	wsSession uint64
}

var botWSSessionSeq uint64

type Client struct {
	mu      sync.RWMutex
	bots    map[string]*BotEntry
	running bool
}

var globalClient *Client

func Global() *Client {
	if globalClient == nil {
		globalClient = NewClient()
	}
	return globalClient
}

func NewClient() *Client {
	return &Client{
		bots: make(map[string]*BotEntry),
	}
}

func (c *Client) RegisterBot(cfg *BotConfig, runtime *agent.Runtime) error {
	if cfg == nil || cfg.AppID == "" || cfg.AgentID == 0 {
		return fmt.Errorf("invalid bot config: app_id and agent_id required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.bots[cfg.AppID]; exists {
		logger.Warn("larkbot: app_id already bound to another agent, one-to-many not supported",
			"app_id", cfg.AppID, "new_agent_id", cfg.AgentID, "existing_agent_id", c.bots[cfg.AppID].Config.AgentID)
		return fmt.Errorf("app_id %s already bound to agent %d, one-to-one only (one-to-many not supported)", cfg.AppID, c.bots[cfg.AppID].Config.AgentID)
	}

	c.bots[cfg.AppID] = &BotEntry{
		Config:  cfg,
		Runtime: runtime,
	}

	logger.Info("larkbot: bot registered", "app_id", cfg.AppID, "agent_id", cfg.AgentID)
	return nil
}

func (c *Client) UnregisterBot(appID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.bots[appID]
	if ok {
		if entry != nil && entry.wsCancel != nil {
			entry.wsCancel()
		}
		delete(c.bots, appID)
		logger.Info("larkbot: bot unregistered (ws client will disconnect when context is cancelled)", "app_id", appID)
	}
}

func (c *Client) UpdateBotConfig(cfg *BotConfig) error {
	if cfg == nil || cfg.AppID == "" || cfg.AgentID == 0 {
		return fmt.Errorf("invalid bot config: app_id and agent_id required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.bots[cfg.AppID]
	if !exists {
		return fmt.Errorf("bot not found for app_id: %s", cfg.AppID)
	}

	if entry.Config.AgentID != cfg.AgentID {
		return fmt.Errorf("agent_id mismatch: cannot change agent binding")
	}

	entry.Config = cfg
	logger.Info("larkbot: bot config updated", "app_id", cfg.AppID, "agent_id", cfg.AgentID)
	return nil
}

func newLarkWSClient(cfg *BotConfig) *ws.Client {
	domain := openAPIDomainFromBot(cfg)
	return ws.NewClient(cfg.AppID, cfg.AppSecret,
		ws.WithDomain(domain),
		ws.WithLogLevel(larkcore.LogLevelInfo),
		ws.WithAutoReconnect(true),
	)
}

func (c *Client) RefreshBot(ctx context.Context, appID string) error {
	c.mu.Lock()
	entry, exists := c.bots[appID]
	if !exists || entry == nil {
		c.mu.Unlock()
		return fmt.Errorf("bot not found for app_id: %s", appID)
	}
	if entry.wsCancel != nil {
		entry.wsCancel()
		entry.wsClient = nil
		entry.wsCancel = nil
	}
	cfg := entry.Config
	wsClient := newLarkWSClient(cfg)
	ctx2, cancel := context.WithCancel(ctx)
	sess := atomic.AddUint64(&botWSSessionSeq, 1)
	entry.wsClient = wsClient
	entry.wsCancel = cancel
	entry.wsSession = sess
	c.syncRunningFlag()
	c.mu.Unlock()

	go func() {
		logger.Info("larkbot: refreshing ws client", "app_id", appID)
		err := wsClient.Start(ctx2)
		c.mu.Lock()
		if e, ok := c.bots[appID]; ok && e.wsSession == sess {
			e.wsClient = nil
			e.wsCancel = nil
		}
		c.syncRunningFlag()
		c.mu.Unlock()
		if err != nil {
			logger.Error("larkbot: ws client refresh failed", "app_id", appID, "err", err)
		}
	}()

	return nil
}

func (c *Client) RefreshAllBots(ctx context.Context) {
	appIDs := c.appIDsForGlobalWS("refresh")
	for _, appID := range appIDs {
		if err := c.RefreshBot(ctx, appID); err != nil {
			logger.Warn("larkbot: refresh bot skipped", "app_id", appID, "err", err)
		}
	}
	logger.Info("larkbot: all eligible ws clients refreshed", "bot_count", len(appIDs))
}

// appIDsForGlobalWS returns app_ids that participate in global Start/Refresh (ws_enabled true or legacy nil).
// Skips NoAutoStartWS bots (ws_enabled false); logs each skip so operators can see why.
func (c *Client) appIDsForGlobalWS(op string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.bots) == 0 {
		return nil
	}
	appIDs := make([]string, 0, len(c.bots))
	for id, e := range c.bots {
		if e != nil && e.Config != nil && e.Config.NoAutoStartWS {
			logger.Info("larkbot: skip global "+op+" (im ws_enabled false)", "app_id", id)
			continue
		}
		appIDs = append(appIDs, id)
	}
	return appIDs
}

func (c *Client) Start(ctx context.Context) {
	appIDs := c.appIDsForGlobalWS("start")
	if len(appIDs) == 0 {
		if c.GetBotCount() == 0 {
			logger.Info("larkbot: no bots registered, skip starting")
		} else {
			logger.Info("larkbot: no bots eligible for global start (all ws_enabled false)")
		}
		return
	}

	for _, appID := range appIDs {
		if err := c.StartBot(ctx, appID); err != nil {
			logger.Warn("larkbot: start bot skipped", "app_id", appID, "err", err)
		}
	}
	logger.Info("larkbot: all eligible ws clients started", "bot_count", len(appIDs))
}

// StartBot opens the WebSocket for a single registered app_id.
func (c *Client) StartBot(parentCtx context.Context, appID string) error {
	c.mu.Lock()
	entry, ok := c.bots[appID]
	if !ok || entry == nil {
		c.mu.Unlock()
		return fmt.Errorf("bot not found for app_id: %s", appID)
	}
	if entry.wsCancel != nil {
		c.mu.Unlock()
		return nil
	}
	cfg := entry.Config
	wsClient := newLarkWSClient(cfg)
	ctx, cancel := context.WithCancel(parentCtx)
	sess := atomic.AddUint64(&botWSSessionSeq, 1)
	entry.wsClient = wsClient
	entry.wsCancel = cancel
	entry.wsSession = sess
	c.syncRunningFlag()
	c.mu.Unlock()

	go func() {
		logger.Info("larkbot: ws client starting", "app_id", appID)
		err := wsClient.Start(ctx)
		c.mu.Lock()
		if e, ok2 := c.bots[appID]; ok2 && e.wsSession == sess {
			e.wsClient = nil
			e.wsCancel = nil
		}
		c.syncRunningFlag()
		c.mu.Unlock()
		if err != nil {
			logger.Error("larkbot: ws client start failed", "app_id", appID, "err", err)
		}
	}()
	return nil
}

// StopBot stops the WebSocket for one app_id without removing registration.
func (c *Client) StopBot(appID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.bots[appID]
	if !ok || entry == nil {
		return
	}
	if entry.wsCancel != nil {
		entry.wsCancel()
	}
	entry.wsClient = nil
	entry.wsCancel = nil
	c.syncRunningFlag()
	logger.Info("larkbot: ws client stopped for app", "app_id", appID)
}

func (c *Client) Stop() {
	c.mu.Lock()
	for appID := range c.bots {
		entry := c.bots[appID]
		if entry != nil && entry.wsCancel != nil {
			entry.wsCancel()
			entry.wsClient = nil
			entry.wsCancel = nil
		}
	}
	c.syncRunningFlag()
	c.mu.Unlock()
	logger.Info("larkbot: all ws clients stopped")
}

func (c *Client) syncRunningFlag() {
	c.running = false
	for _, e := range c.bots {
		if e != nil && e.wsCancel != nil {
			c.running = true
			return
		}
	}
}

// IsBotWSRunning reports whether this app_id has an active WebSocket session.
func (c *Client) IsBotWSRunning(appID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.bots[appID]
	return ok && e != nil && e.wsCancel != nil
}

func (c *Client) GetBotRuntime(appID string) *agent.Runtime {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if entry, ok := c.bots[appID]; ok {
		return entry.Runtime
	}
	return nil
}

func (c *Client) GetBotConfig(appID string) *BotConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if entry, ok := c.bots[appID]; ok {
		return entry.Config
	}
	return nil
}

func (c *Client) GetBots() map[string]*BotEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*BotEntry, len(c.bots))
	for k, v := range c.bots {
		result[k] = v
	}
	return result
}

func (c *Client) GetBotCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.bots)
}

func (c *Client) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

func (c *Client) handleMessage(event *larkim.P2MessageReceiveV1) error {
	if event == nil || event.Event == nil {
		return nil
	}

	msg := event.Event.Message
	if msg == nil {
		return nil
	}

	messageID := getStringPtr(msg.MessageId)
	chatID := getStringPtr(msg.ChatId)
	senderID := getSenderIDFromSender(event.Event.Sender)
	senderType := getStringPtr(event.Event.Sender.SenderType)

	logger.Info("larkbot: message received",
		"message_id", messageID,
		"chat_id", chatID,
		"sender_type", senderType,
	)

	if senderType != "user" {
		logger.Info("larkbot: ignoring non-user message")
		return nil
	}

	var userInput string
	msgType := getStringPtr(msg.MessageType)
	switch msgType {
	case "text":
		userInput = extractTextFromContent(getStringPtr(msg.Content))
	case "post":
		userInput = extractTextFromPost(getStringPtr(msg.Content))
	}

	if userInput == "" {
		logger.Info("larkbot: empty message, ignoring")
		return nil
	}

	appID := getAppID(event)

	runtime := c.GetBotRuntime(appID)
	if runtime == nil {
		logger.Error("larkbot: no bot runtime for app_id", "app_id", appID)
		return nil
	}

	cfg := c.GetBotConfig(appID)
	invokeCtx := context.Background()
	respText, err := c.invokeAgent(invokeCtx, runtime, cfg, userInput, senderID)
	if err != nil {
		respText = fmt.Sprintf("处理消息失败: %v", err)
		logger.Error("larkbot: invoke agent failed", "err", err)
	}

	if messageID != "" {
		logger.Info("larkbot: reply message", "root_id", messageID, "text", respText)
	}

	return nil
}

func (c *Client) invokeAgent(ctx context.Context, runtime *agent.Runtime, cfg *BotConfig, userInput, larkUserID string) (string, error) {
	timeout := cfg.InvokeTimeout
	if timeout <= 0 {
		timeout = 120
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	resp, err := runtime.Chat(ctx, cfg.AgentID, userInput, "", larkUserID)
	return resp, err
}

func getStringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func getSenderIDFromSender(sender *larkim.EventSender) string {
	if sender == nil || sender.SenderId == nil {
		return ""
	}
	if sender.SenderId.OpenId != nil {
		return *sender.SenderId.OpenId
	}
	if sender.SenderId.UnionId != nil {
		return *sender.SenderId.UnionId
	}
	if sender.SenderId.UserId != nil {
		return *sender.SenderId.UserId
	}
	return ""
}

func extractTextFromContent(content string) string {
	if content == "" {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(content), &m); err != nil {
		return content
	}
	if t, ok := m["text"].(string); ok {
		return t
	}
	return content
}

func extractTextFromPost(content string) string {
	if content == "" {
		return ""
	}
	return content
}

func getAppID(event *larkim.P2MessageReceiveV1) string {
	if event == nil {
		return ""
	}
	if event.EventV2Base != nil && event.EventV2Base.Header != nil {
		return event.EventV2Base.Header.AppID
	}
	return ""
}
