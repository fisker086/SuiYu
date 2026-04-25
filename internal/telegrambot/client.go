package telegrambot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fisk086/sya/internal/agent"
	"github.com/fisk086/sya/internal/logger"
)

type BotConfig struct {
	Token          string
	ChatID         string
	AgentID        int64
	InvokeTimeout  int
	WebhookURL     string
	WebhookEnabled bool
}

type BotEntry struct {
	Config  *BotConfig
	Runtime *agent.Runtime
	httpSrv *http.Server
}

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
	if cfg == nil || cfg.Token == "" || cfg.AgentID == 0 {
		return fmt.Errorf("invalid bot config: token and agent_id required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.bots[cfg.Token]; exists {
		logger.Warn("telegrambot: token already bound to another agent",
			"token_prefix", cfg.Token[:8], "new_agent_id", cfg.AgentID, "existing_agent_id", c.bots[cfg.Token].Config.AgentID)
		return fmt.Errorf("token already bound to agent %d", c.bots[cfg.Token].Config.AgentID)
	}

	c.bots[cfg.Token] = &BotEntry{
		Config:  cfg,
		Runtime: runtime,
	}

	logger.Info("telegrambot: bot registered", "agent_id", cfg.AgentID, "has_chat_id", cfg.ChatID != "")
	return nil
}

func (c *Client) UnregisterBot(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.bots[token]
	if ok {
		if entry != nil && entry.httpSrv != nil {
			entry.httpSrv.Close()
		}
		delete(c.bots, token)
		logger.Info("telegrambot: bot unregistered", "token_prefix", token[:8])
	}
}

func (c *Client) UpdateBotConfig(cfg *BotConfig) error {
	if cfg == nil || cfg.Token == "" || cfg.AgentID == 0 {
		return fmt.Errorf("invalid bot config: token and agent_id required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.bots[cfg.Token]
	if !exists {
		return fmt.Errorf("bot not found for token")
	}

	if entry.Config.AgentID != cfg.AgentID {
		return fmt.Errorf("agent_id mismatch: cannot change agent binding")
	}

	entry.Config = cfg
	logger.Info("telegrambot: bot config updated", "agent_id", cfg.AgentID)
	return nil
}

func (c *Client) GetBotConfig(token string) *BotConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if entry, ok := c.bots[token]; ok {
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

func (c *Client) Start(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.bots) == 0 {
		logger.Info("telegrambot: no bots registered, skip starting")
		return
	}

	for token, entry := range c.bots {
		if entry.Config.WebhookEnabled && entry.Config.WebhookURL != "" {
			if err := c.setWebhook(ctx, token, entry.Config.WebhookURL); err != nil {
				logger.Warn("telegrambot: set webhook failed", "agent_id", entry.Config.AgentID, "err", err)
			}
		}
	}

	c.running = true
	logger.Info("telegrambot: all bots started", "bot_count", len(c.bots))
}

func (c *Client) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, entry := range c.bots {
		if entry != nil && entry.httpSrv != nil {
			entry.httpSrv.Close()
			entry.httpSrv = nil
		}
	}
	c.running = false
	logger.Info("telegrambot: all bots stopped")
}

func (c *Client) StartBot(ctx context.Context, token string) error {
	c.mu.Lock()
	entry, ok := c.bots[token]
	if !ok || entry == nil {
		c.mu.Unlock()
		return fmt.Errorf("bot not found for token")
	}
	if entry.httpSrv != nil {
		c.mu.Unlock()
		return nil
	}
	cfg := entry.Config
	c.mu.Unlock()

	if cfg.WebhookEnabled && cfg.WebhookURL != "" {
		if err := c.setWebhook(ctx, token, cfg.WebhookURL); err != nil {
			return err
		}
	}

	c.running = true
	c.mu.Unlock()

	logger.Info("telegrambot: bot started", "agent_id", cfg.AgentID)
	return nil
}

func (c *Client) StopBot(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.bots[token]
	if !ok || entry == nil {
		return
	}
	if entry.httpSrv != nil {
		entry.httpSrv.Close()
		entry.httpSrv = nil
	}
	logger.Info("telegrambot: bot stopped", "agent_id", entry.Config.AgentID)
}

func (c *Client) IsBotRunning(token string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.bots[token]
	return ok && entry != nil && entry.httpSrv != nil
}

func (c *Client) setWebhook(ctx context.Context, token, url string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/setWebhook", token)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(fmt.Sprintf(`{"url": "%s"}`, url)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook setup failed: %s", body)
	}

	logger.Info("telegrambot: webhook set", "url", url)
	return nil
}

func (c *Client) HandleUpdate(ctx context.Context, payload []byte) error {
	var update Update
	if err := json.Unmarshal(payload, &update); err != nil {
		logger.Warn("telegrambot: failed to parse update", "err", err)
		return err
	}

	if update.Message == nil {
		return nil
	}

	msg := update.Message
	userInput := strings.TrimSpace(msg.Text)
	if userInput == "" {
		logger.Info("telegrambot: empty message, ignoring")
		return nil
	}

	botToken := msg.Bot
	if botToken == "" {
		logger.Warn("telegrambot: no bot token in message")
		return nil
	}

	runtime := c.GetBotRuntime(botToken)
	if runtime == nil {
		logger.Error("telegrambot: no bot runtime for token", "token_prefix", botToken[:8])
		return nil
	}

	cfg := c.GetBotConfig(botToken)
	chatID := getChatID(msg.Chat)

	timeout := cfg.InvokeTimeout
	if timeout <= 0 {
		timeout = 120
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	respText, err := c.invokeAgent(ctx, runtime, cfg.AgentID, userInput, chatID)
	if err != nil {
		respText = fmt.Sprintf("处理消息失败: %v", err)
		logger.Error("telegrambot: invoke agent failed", "err", err)
	}

	if chatID != "" {
		if err := c.SendMessage(ctx, botToken, chatID, respText); err != nil {
			logger.Warn("telegrambot: send reply failed", "err", err)
		}
	}

	return nil
}

func (c *Client) invokeAgent(ctx context.Context, runtime *agent.Runtime, agentID int64, userInput, userID string) (string, error) {
	resp, err := runtime.Chat(ctx, agentID, userInput, userID, "")
	return resp, err
}

func (c *Client) SendMessage(ctx context.Context, token, chatID, text string) error {
	if chatID == "" {
		return nil
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	payload := fmt.Sprintf(`{"chat_id": "%s", "text": "%s"}`, chatID, escapeJSON(text))
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send message failed: %s", body)
	}

	return nil
}

func (c *Client) GetBotRuntime(token string) *agent.Runtime {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if entry, ok := c.bots[token]; ok {
		return entry.Runtime
	}
	return nil
}

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	Chat      *Chat  `json:"chat"`
	From      *User  `json:"from,omitempty"`
	Text      string `json:"text"`
	Bot       string `json:"bot,omitempty"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type User struct {
	ID       int64  `json:"id"`
	IsBot    bool   `json:"is_bot"`
	Username string `json:"username,omitempty"`
}

func getChatID(chat *Chat) string {
	if chat == nil {
		return ""
	}
	return fmt.Sprintf("%d", chat.ID)
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}
