package controller

import (
	"context"
	"errors"
	"net/http"

	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/service"
	"github.com/fisk086/sya/internal/storage"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

type MCPController struct {
	mcpService *service.MCPService
}

func NewMCPController(mcpService *service.MCPService) *MCPController {
	return &MCPController{mcpService: mcpService}
}

func (c *MCPController) RegisterRoutes(r *server.Hertz) {
	mcp := r.Group("/api/v1/mcp")
	mcp.GET("/configs", c.ListConfigs)
	mcp.POST("/configs", c.CreateConfig)
	mcp.GET("/configs/:id/tools", c.ListTools)
	mcp.PUT("/configs/:id", c.UpdateConfig)
	mcp.DELETE("/configs/:id", c.DeleteConfig)
	mcp.POST("/configs/:id/sync", c.SyncServer)
}

// @Summary List all MCP configs
// @Description Get a list of all MCP server configurations
// @Tags mcp
// @Accept json
// @Produce json
// @Success 200 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /mcp/configs [get]
func (c *MCPController) ListConfigs(ctx context.Context, hc *app.RequestContext) {
	configs, err := c.mcpService.ListConfigs()
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(configs))
}

// @Summary Create a new MCP config
// @Description Create a new MCP server configuration
// @Tags mcp
// @Accept json
// @Produce json
// @Param config body schema.CreateMCPConfigRequest true "MCP config data"
// @Success 201 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /mcp/configs [post]
func (c *MCPController) CreateConfig(ctx context.Context, hc *app.RequestContext) {
	var req schema.CreateMCPConfigRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	cfg, err := c.mcpService.CreateConfig(&req)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	hc.JSON(http.StatusCreated, schema.SuccessResponse(cfg))
}

// @Summary List tools synced for an MCP config
// @Tags mcp
// @Produce json
// @Param id path int true "MCP Config ID"
// @Success 200 {object} schema.APIResponse
// @Failure 404 {object} schema.APIResponse
// @Router /mcp/configs/{id}/tools [get]
func (c *MCPController) ListTools(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid config id"))
		return
	}
	tools, err := c.mcpService.ListTools(id)
	if err != nil {
		if errors.Is(err, storage.ErrMCPConfigNotFound) {
			hc.JSON(http.StatusNotFound, schema.ErrorResponse(err.Error()))
			return
		}
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(tools))
}

// @Summary Update an MCP config
// @Description Update an existing MCP server configuration
// @Tags mcp
// @Accept json
// @Produce json
// @Param id path int true "MCP Config ID"
// @Param config body schema.CreateMCPConfigRequest true "MCP config data"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 404 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /mcp/configs/{id} [put]
func (c *MCPController) UpdateConfig(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid config id"))
		return
	}

	var req schema.CreateMCPConfigRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	cfg, err := c.mcpService.UpdateConfig(id, &req)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	if cfg == nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("config not found"))
		return
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(cfg))
}

// @Summary Delete an MCP config
// @Description Delete an MCP server configuration
// @Tags mcp
// @Accept json
// @Produce json
// @Param id path int true "MCP Config ID"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /mcp/configs/{id} [delete]
func (c *MCPController) DeleteConfig(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid config id"))
		return
	}

	if err := c.mcpService.DeleteConfig(id); err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(nil))
}

// @Summary Sync MCP server tools
// @Description Sync tools from an MCP server
// @Tags mcp
// @Accept json
// @Produce json
// @Param id path int true "MCP Config ID"
// @Param req body schema.SyncMCPServerRequest true "Sync request"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /mcp/configs/{id}/sync [post]
func (c *MCPController) SyncServer(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid config id"))
		return
	}

	var req schema.SyncMCPServerRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	if err := c.mcpService.SyncServer(ctx, id, &req); err != nil {
		if errors.Is(err, service.ErrMCPDiscoveryNeedsTarget) {
			hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
			return
		}
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(nil))
}
