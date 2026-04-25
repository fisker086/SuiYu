package controller

import (
	"context"
	"net/http"
	"sort"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/service"
	"github.com/fisk086/sya/internal/telegrambot"
)

type TelegramBotStatus struct {
	AgentID   int64  `json:"agent_id"`
	ChatID    string `json:"chat_id"`
	AgentName string `json:"agent_name"`
	IsRunning bool   `json:"is_running"`
}

func NewTelegramBotController(agentService *service.AgentService) *TelegramBotController {
	return &TelegramBotController{agentService: agentService}
}

type TelegramBotController struct {
	agentService *service.AgentService
}

func (c *TelegramBotController) RegisterRoutes(r *server.Hertz) {
	r.GET("/api/v1/telegrambots", c.ListBots)
	r.POST("/api/v1/telegrambots/start", c.Start)
	r.POST("/api/v1/telegrambots/stop", c.Stop)
	r.POST("/api/v1/telegrambots/:agentId/register", c.RegisterForAgent)
	r.POST("/api/v1/telegrambots/:agentId/ws/start", c.StartAgentWS)
	r.POST("/api/v1/telegrambots/:agentId/ws/stop", c.StopAgentWS)
	r.DELETE("/api/v1/telegrambots/:agentId", c.UnregisterForAgent)
	r.POST("/api/v1/telegrambots/webhook", c.Webhook)
}

func (c *TelegramBotController) ListBots(ctx context.Context, hc *app.RequestContext) {
	client := telegrambot.Global()

	var entries []TelegramBotStatus
	if c.agentService != nil {
		agents, err := c.agentService.ListAgents()
		if err != nil {
			hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
			return
		}
		for _, a := range agents {
			if a == nil {
				continue
			}
			full, err := c.agentService.GetAgent(a.ID)
			if err != nil || full == nil || full.RuntimeProfile == nil {
				continue
			}
			if full.RuntimeProfile.IMEnabled != "telegram" {
				continue
			}
			token := full.RuntimeProfile.IMConfig.TelegramToken
			if token == "" {
				continue
			}
			inMem := client.GetBotConfig(token) != nil
			isRunning := inMem && client.IsBotRunning(token)
			entries = append(entries, TelegramBotStatus{
				AgentID:   a.ID,
				ChatID:    full.RuntimeProfile.IMConfig.TelegramChatID,
				AgentName: full.Name,
				IsRunning: isRunning,
			})
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].AgentID < entries[j].AgentID })
	} else {
		bots := client.GetBots()
		for token, entry := range bots {
			entries = append(entries, TelegramBotStatus{
				AgentID:   entry.Config.AgentID,
				ChatID:    entry.Config.ChatID,
				AgentName: "",
				IsRunning: client.IsBotRunning(token),
			})
		}
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"bots":      entries,
		"running":   client.IsRunning(),
		"bot_count": len(entries),
	}))
}

func (c *TelegramBotController) Start(ctx context.Context, hc *app.RequestContext) {
	client := telegrambot.Global()

	if client.GetBotCount() == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("no bots registered"))
		return
	}

	go func() {
		client.Start(ctx)
	}()

	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"started":   true,
		"bot_count": client.GetBotCount(),
	}))
}

func (c *TelegramBotController) Stop(ctx context.Context, hc *app.RequestContext) {
	client := telegrambot.Global()
	client.Stop()

	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"stopped": true,
	}))
}

func (c *TelegramBotController) RegisterForAgent(ctx context.Context, hc *app.RequestContext) {
	agentID := parseInt64Param(hc, "agentId")
	if agentID == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}

	ctrl := GetAgentController()
	if ctrl == nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse("agent controller not available"))
		return
	}

	agent, err := ctrl.GetAgentByID(agentID)
	if err != nil || agent == nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("agent not found"))
		return
	}

	if agent.RuntimeProfile == nil || agent.RuntimeProfile.IMEnabled != "telegram" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("agent does not have telegram im enabled"))
		return
	}

	ctrl.RegisterTelegramBotForAgent(agent)

	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"registered":     true,
		"agent_id":       agentID,
		"telegram_token": agent.RuntimeProfile.IMConfig.TelegramToken,
	}))
}

func (c *TelegramBotController) UnregisterForAgent(ctx context.Context, hc *app.RequestContext) {
	agentID := parseInt64Param(hc, "agentId")
	if agentID == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}

	ctrl := GetAgentController()
	if ctrl == nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse("agent controller not available"))
		return
	}

	ctrl.UnregisterTelegramBotForAgent(agentID)

	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"unregistered": true,
		"agent_id":     agentID,
	}))
}

func (c *TelegramBotController) StartAgentWS(ctx context.Context, hc *app.RequestContext) {
	agentID := parseInt64Param(hc, "agentId")
	if agentID == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}
	ctrl := GetAgentController()
	if ctrl == nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse("agent controller not available"))
		return
	}
	agent, err := ctrl.GetAgentByID(agentID)
	if err != nil || agent == nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("agent not found"))
		return
	}
	if agent.RuntimeProfile == nil || agent.RuntimeProfile.IMEnabled != "telegram" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("agent does not have telegram im enabled"))
		return
	}
	token := agent.RuntimeProfile.IMConfig.TelegramToken
	if token == "" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("telegram token is empty"))
		return
	}
	ctrl.RegisterTelegramBotForAgent(agent)
	client := telegrambot.Global()
	if client.GetBotConfig(token) == nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("bot not registered: check Telegram Token and save the agent, or ensure IM channel is Telegram"))
		return
	}
	if err := client.StartBot(context.Background(), token); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"started":      true,
		"token_prefix": token[:8],
	}))
}

func (c *TelegramBotController) StopAgentWS(ctx context.Context, hc *app.RequestContext) {
	agentID := parseInt64Param(hc, "agentId")
	if agentID == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}
	ctrl := GetAgentController()
	if ctrl == nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse("agent controller not available"))
		return
	}
	agent, err := ctrl.GetAgentByID(agentID)
	if err != nil || agent == nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("agent not found"))
		return
	}
	if agent.RuntimeProfile == nil || agent.RuntimeProfile.IMEnabled != "telegram" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("agent does not have telegram im enabled"))
		return
	}
	token := agent.RuntimeProfile.IMConfig.TelegramToken
	if token == "" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("telegram token is empty"))
		return
	}
	telegrambot.Global().StopBot(token)
	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"stopped":      true,
		"token_prefix": token[:8],
	}))
}

func (c *TelegramBotController) Webhook(ctx context.Context, hc *app.RequestContext) {
	body := hc.Request.Body()
	if body == nil {
		logger.Error("telegrambot: failed to read request body", "err", "nil body")
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid request body"))
		return
	}

	client := telegrambot.Global()
	if err := client.HandleUpdate(ctx, body); err != nil {
		logger.Warn("telegrambot: failed to handle update", "err", err)
		hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{"error": err.Error()}))
		return
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{"ok": true}))
}
