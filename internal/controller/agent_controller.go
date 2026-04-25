package controller

import (
	"context"
	"errors"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/fisk086/sya/internal/agent"
	"github.com/fisk086/sya/internal/auth"
	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/service"
	"github.com/fisk086/sya/internal/storage"
)

type AgentController struct {
	agentService *service.AgentService
	chatService  *service.ChatService
	runtime      *agent.Runtime
	rbacService  *service.RBACService
	jwtCfg       auth.JWTConfig
	userStore    storage.UserStore
	imManager    IMManagerInterface
}

type IMManagerInterface interface {
	RegisterAgent(agent *schema.AgentWithRuntime, runtime *agent.Runtime) error
	UnregisterAgent(agentID int64)
}

func NewAgentController(agentService *service.AgentService, chatService *service.ChatService, runtime *agent.Runtime, jwtCfg auth.JWTConfig, userStore storage.UserStore, rbacService ...*service.RBACService) *AgentController {
	ctrl := &AgentController{
		agentService: agentService,
		chatService:  chatService,
		runtime:      runtime,
		jwtCfg:       jwtCfg,
		userStore:    userStore,
	}
	if len(rbacService) > 0 {
		ctrl.rbacService = rbacService[0]
	}
	globalAgentController = ctrl
	return ctrl
}

func (c *AgentController) SetIMManager(mgr IMManagerInterface) {
	c.imManager = mgr
}

func (c *AgentController) registerLarkBotIfNeeded(agent *schema.AgentWithRuntime) {
	if c.imManager == nil || agent == nil || agent.RuntimeProfile == nil {
		return
	}

	if err := c.imManager.RegisterAgent(agent, c.runtime); err != nil {
		logger.Error("failed to register im bot for agent", "agent_id", agent.ID, "err", err)
	}
}

func (c *AgentController) unregisterLarkBotIfNeeded(agentID int64) {
	if c.imManager == nil {
		return
	}
	c.imManager.UnregisterAgent(agentID)
}

func (c *AgentController) GetAgentByID(id int64) (*schema.AgentWithRuntime, error) {
	return c.agentService.GetAgent(id)
}

func (c *AgentController) RegisterLarkBotForAgent(agent *schema.AgentWithRuntime) {
	c.registerLarkBotIfNeeded(agent)
}

func (c *AgentController) UnregisterLarkBotForAgent(agentID int64) {
	c.unregisterLarkBotIfNeeded(agentID)
}

func (c *AgentController) RegisterTelegramBotForAgent(agent *schema.AgentWithRuntime) {
	c.registerLarkBotIfNeeded(agent)
}

func (c *AgentController) UnregisterTelegramBotForAgent(agentID int64) {
	c.unregisterLarkBotIfNeeded(agentID)
}

func (c *AgentController) getUserForMiddleware(userID int64) (*auth.User, error) {
	if c.userStore == nil {
		return nil, errors.New("user store not configured")
	}
	user, err := c.userStore.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	return &auth.User{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Status:   string(user.Status),
		IsAdmin:  user.IsAdmin,
	}, nil
}

func (c *AgentController) RegisterRoutes(r *server.Hertz) {
	agents := r.Group("/api/v1/agents")
	if c.userStore != nil {
		agents.Use(auth.OptionalJWTMiddleware(c.jwtCfg, c.getUserForMiddleware))
	}
	agents.GET("", c.ListAgents)
	agents.GET("/all", c.ListAllAgents)
	agents.GET("/for-schedule", c.ListAgentsForSchedule)
	agents.GET("/:id", c.GetAgent)
	agents.POST("", c.CreateAgent)
	agents.PUT("/:id", c.UpdateAgent)
	agents.DELETE("/:id", c.DeleteAgent)
	agents.GET("/:id/capability-tree", c.GetCapabilityTree)
	agents.PUT("/:id/capability-tree", c.UpdateCapabilityTree)
	agents.GET("/:id/capabilities", c.GetCapabilities)
}

// @Summary List all agents
// @Description Get a list of all agents
// @Tags agents
// @Accept json
// @Produce json
// @Success 200 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /agents [get]
func (c *AgentController) ListAgents(ctx context.Context, hc *app.RequestContext) {
	agents, err := c.agentService.ListAgents()
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	if c.rbacService != nil {
		user := auth.GetCurrentUser(hc)
		if user != nil {
			var filtered []schema.Agent
			for _, a := range agents {
				if c.rbacService.CheckAgentAccess(ctx, user.ID, a.Name, user.IsAdmin) {
					filtered = append(filtered, *a)
				}
			}
			hc.JSON(http.StatusOK, schema.SuccessResponse(filtered))
			return
		}
		// No JWT user in context: hide catalog when we can authenticate (DB + optional middleware); otherwise keep legacy behavior (in-memory dev without user store).
		if c.userStore != nil {
			hc.JSON(http.StatusOK, schema.SuccessResponse([]schema.Agent{}))
			return
		}
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(agents))
}

// @Summary List all agents (admin only, no RBAC filter)
// @Description Get a list of all agents without RBAC filtering
// @Tags agents
// @Accept json
// @Produce json
// @Success 200 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /agents/all [get]
func (c *AgentController) ListAllAgents(ctx context.Context, hc *app.RequestContext) {
	agents, err := c.agentService.ListAgents()
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(agents))
}

// @Summary List agents available for schedule
// @Description Get a list of agents filtered by excluding those with client execution mode
// @Tags agents
// @Accept json
// @Produce json
// @Success 200 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /agents/for-schedule [get]
func (c *AgentController) ListAgentsForSchedule(ctx context.Context, hc *app.RequestContext) {
	agents, err := c.agentService.ListAgentsForSchedule()
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(agents))
}

// @Summary Get agent by ID
// @Description Get a single agent by its ID
// @Tags agents
// @Accept json
// @Produce json
// @Param id path int true "Agent ID"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 404 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /agents/{id} [get]
func (c *AgentController) GetAgent(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}

	agent, err := c.agentService.GetAgent(id)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	if agent == nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("agent not found"))
		return
	}

	if c.rbacService != nil && c.userStore != nil {
		user := auth.GetCurrentUser(hc)
		if user == nil {
			hc.JSON(http.StatusNotFound, schema.ErrorResponse("agent not found"))
			return
		}
		if !c.rbacService.CheckAgentAccess(ctx, user.ID, agent.Name, user.IsAdmin) {
			hc.JSON(http.StatusNotFound, schema.ErrorResponse("agent not found"))
			return
		}
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(agent))
}

// @Summary Create a new agent
// @Description Create a new agent with the specified configuration
// @Tags agents
// @Accept json
// @Produce json
// @Param agent body schema.CreateAgentRequest true "Agent data"
// @Success 201 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /agents [post]
func (c *AgentController) CreateAgent(ctx context.Context, hc *app.RequestContext) {
	var req schema.CreateAgentRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	agent, err := c.agentService.CreateAgent(&req)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	fullAgent, _ := c.agentService.GetAgent(agent.ID)
	if fullAgent != nil {
		c.runtime.RegisterAgent(fullAgent)
		c.registerLarkBotIfNeeded(fullAgent)
	}

	hc.JSON(http.StatusCreated, schema.SuccessResponse(agent))
}

// @Summary Update an agent
// @Description Update an existing agent by its ID
// @Tags agents
// @Accept json
// @Produce json
// @Param id path int true "Agent ID"
// @Param agent body schema.UpdateAgentRequest true "Agent data"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 404 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /agents/{id} [put]
func (c *AgentController) UpdateAgent(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}

	var req schema.UpdateAgentRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	agent, err := c.agentService.UpdateAgent(id, &req)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	if agent == nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("agent not found"))
		return
	}

	fullAgent, _ := c.agentService.GetAgent(id)
	if fullAgent != nil {
		c.runtime.RegisterAgent(fullAgent)
		c.registerLarkBotIfNeeded(fullAgent)
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(agent))
}

// @Summary Delete an agent
// @Description Delete an agent by its ID
// @Tags agents
// @Accept json
// @Produce json
// @Param id path int true "Agent ID"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /agents/{id} [delete]
func (c *AgentController) DeleteAgent(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}

	if err := c.agentService.DeleteAgent(id); err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	c.runtime.UnregisterAgent(id)
	c.unregisterLarkBotIfNeeded(id)

	hc.JSON(http.StatusOK, schema.SuccessResponse(nil))
}

// @Summary Get agent capability tree
// @Description Get the capability tree structure for an agent
// @Tags agents
// @Accept json
// @Produce json
// @Param id path int true "Agent ID"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /agents/{id}/capability-tree [get]
func (c *AgentController) GetCapabilityTree(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}

	tree, err := c.agentService.GetCapabilityTree(id)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(tree))
}

// @Summary Update agent capability tree
// @Description Update the capability tree structure for an agent
// @Tags agents
// @Accept json
// @Produce json
// @Param id path int true "Agent ID"
// @Param tree body schema.UpdateCapabilityTreeRequest true "Capability tree data"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /agents/{id}/capability-tree [put]
func (c *AgentController) UpdateCapabilityTree(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}

	var req schema.UpdateCapabilityTreeRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	tree, err := c.agentService.UpdateCapabilityTree(id, req.Nodes)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	fullAgent, _ := c.agentService.GetAgent(id)
	if fullAgent != nil {
		fullAgent.CapabilityTree = tree
		c.runtime.RegisterAgent(fullAgent)
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(tree))
}

// @Summary Get agent capabilities
// @Description Get the resolved capabilities for an agent
// @Tags agents
// @Accept json
// @Produce json
// @Param id path int true "Agent ID"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Router /agents/{id}/capabilities [get]
func (c *AgentController) GetCapabilities(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid agent id"))
		return
	}

	capabilities := c.chatService.GetCapabilities(id)
	hc.JSON(http.StatusOK, schema.SuccessResponse(capabilities))
}

var globalAgentController *AgentController

func GetAgentController() *AgentController {
	return globalAgentController
}

func init() {
	logger.Debug("agent controller package initialized")
}
