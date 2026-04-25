package controller

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/fisk086/sya/internal/auth"
	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/service"
	"github.com/fisk086/sya/internal/storage"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type AuditController struct {
	auditService *service.AuditService
	jwtCfg       auth.JWTConfig
	userStore    storage.UserStore
}

func NewAuditController(auditService *service.AuditService, jwtCfg auth.JWTConfig, userStore storage.UserStore) *AuditController {
	return &AuditController{auditService: auditService, jwtCfg: jwtCfg, userStore: userStore}
}

func (c *AuditController) getUserForMiddleware(userID int64) (*auth.User, error) {
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

func (c *AuditController) RegisterRoutes(r *server.Hertz) {
	audit := r.Group("/api/v1/audit")
	if c.userStore == nil {
		audit.GET("", c.ListLogsLegacy)
		audit.GET("/:id", c.GetLogLegacy)
		return
	}

	protected := audit.Group("", auth.JWTMiddleware(c.jwtCfg, c.getUserForMiddleware))
	protected.GET("/logs/count", c.CountLogs)
	protected.POST("/logs", c.CreateLog)
	protected.GET("/logs", c.ListLogs)
	protected.DELETE("/logs", c.DeleteLogs)
	protected.GET("/:id", c.GetLog)
	protected.GET("", c.ListLogs)
}

func (c *AuditController) auditFilterFromQuery(ctx *app.RequestContext) *storage.AuditLogFilter {
	filter := &storage.AuditLogFilter{
		UserID:    string(ctx.Query("user_id")),
		SessionID: string(ctx.Query("session_id")),
		ToolName:  string(ctx.Query("tool_name")),
		RiskLevel: string(ctx.Query("risk_level")),
		Status:    string(ctx.Query("status")),
		Page:      queryInt(ctx, "page", 1),
		PageSize:  queryInt(ctx, "page_size", 50),
	}
	if agentIDStr := string(ctx.Query("agent_id")); agentIDStr != "" {
		if id, err := strconv.ParseInt(agentIDStr, 10, 64); err == nil {
			filter.AgentID = id
		}
	}
	return filter
}

// CreateLog godoc
// @Summary Record an audit event
// @Description Desktop / client posts a single audit row (stored in audit_logs). user_id is taken from JWT.
// @Tags audit
// @Accept json
// @Produce json
// @Param request body schema.AuditLogCreateRequest true "Event payload"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 401 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /audit/logs [post]
// @Security BearerAuth
func (c *AuditController) CreateLog(_ context.Context, ctx *app.RequestContext) {
	u := auth.GetCurrentUser(ctx)
	if u == nil {
		ctx.JSON(consts.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	var req schema.AuditLogCreateRequest
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid json"))
		return
	}
	if req.ToolName == "" {
		req.ToolName = "desktop"
	}
	if req.Action == "" {
		req.Action = "event"
	}
	if req.RiskLevel == "" {
		req.RiskLevel = "low"
	}
	if req.Status == "" {
		req.Status = "ok"
	}
	ip := strings.TrimSpace(ctx.ClientIP())
	log := &model.AuditLog{
		UserID:     strconv.FormatInt(u.ID, 10),
		AgentID:    req.AgentID,
		SessionID:  strings.TrimSpace(req.SessionID),
		ToolName:   req.ToolName,
		Action:     req.Action,
		RiskLevel:  req.RiskLevel,
		Input:      req.Input,
		Output:     req.Output,
		Error:      req.Error,
		Status:     req.Status,
		DurationMs: req.DurationMs,
		IPAddress:  ip,
	}
	out, err := c.auditService.CreateLog(log)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, schema.SuccessResponse(out))
}

// ListLogs godoc
// @Summary List audit logs
// @Tags audit
// @Produce json
// @Param user_id query string false "Filter by user_id"
// @Param agent_id query int false "Filter by agent_id"
// @Param session_id query string false "Filter by session_id"
// @Param tool_name query string false "Filter by tool_name"
// @Param risk_level query string false "Filter by risk_level"
// @Param status query string false "Filter by status"
// @Param page query int false "Page (default 1)"
// @Param page_size query int false "Page size (default 50)"
// @Success 200 {object} schema.APIResponse
// @Failure 401 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /audit/logs [get]
// @Security BearerAuth
func (c *AuditController) ListLogs(_ context.Context, ctx *app.RequestContext) {
	filter := c.auditFilterFromQuery(ctx)
	if u := auth.GetCurrentUser(ctx); u != nil && !u.IsAdmin {
		filter.UserID = strconv.FormatInt(u.ID, 10)
	}
	logs, total, err := c.auditService.ListLogs(filter)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"logs":  logs,
		"total": total,
		"page":  filter.Page,
	}))
}

// ListLogsLegacy lists without auth (only when user store is unavailable).
func (c *AuditController) ListLogsLegacy(_ context.Context, ctx *app.RequestContext) {
	filter := c.auditFilterFromQuery(ctx)
	logs, total, err := c.auditService.ListLogs(filter)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"logs":  logs,
		"total": total,
		"page":  filter.Page,
	}))
}

// CountLogs godoc
// @Summary Count audit logs
// @Tags audit
// @Produce json
// @Param user_id query string false "Filter by user_id"
// @Param agent_id query int false "Filter by agent_id"
// @Param session_id query string false "Filter by session_id"
// @Param tool_name query string false "Filter by tool_name"
// @Param risk_level query string false "Filter by risk_level"
// @Param status query string false "Filter by status"
// @Success 200 {object} schema.APIResponse
// @Failure 401 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /audit/logs/count [get]
// @Security BearerAuth
func (c *AuditController) CountLogs(_ context.Context, ctx *app.RequestContext) {
	filter := c.auditFilterFromQuery(ctx)
	if u := auth.GetCurrentUser(ctx); u != nil && !u.IsAdmin {
		filter.UserID = strconv.FormatInt(u.ID, 10)
	}
	n, err := c.auditService.CountLogs(filter)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{"count": n}))
}

// DeleteLogs godoc
// @Summary Delete audit logs
// @Description Non-admin: only rows for the current user. Admin: optional filters; use purge=true to delete all rows.
// @Tags audit
// @Produce json
// @Param user_id query string false "Filter by user_id"
// @Param agent_id query int false "Filter by agent_id"
// @Param session_id query string false "Filter by session_id"
// @Param tool_name query string false "Filter by tool_name"
// @Param risk_level query string false "Filter by risk_level"
// @Param status query string false "Filter by status"
// @Param purge query bool false "Admin only: delete all rows (no other filters)"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 401 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /audit/logs [delete]
// @Security BearerAuth
func (c *AuditController) DeleteLogs(_ context.Context, ctx *app.RequestContext) {
	u := auth.GetCurrentUser(ctx)
	if u == nil {
		ctx.JSON(consts.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	purge := strings.EqualFold(string(ctx.Query("purge")), "true")

	if u.IsAdmin && purge {
		n, err := c.auditService.DeleteLogs(nil)
		if err != nil {
			ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
			return
		}
		ctx.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{"deleted": n}))
		return
	}

	filter := c.auditFilterFromQuery(ctx)
	if !u.IsAdmin {
		filter.UserID = strconv.FormatInt(u.ID, 10)
	} else {
		hasScope := filter.UserID != "" || filter.AgentID != 0 || filter.SessionID != "" ||
			filter.ToolName != "" || filter.RiskLevel != "" || filter.Status != ""
		if !hasScope {
			ctx.JSON(http.StatusBadRequest, schema.ErrorResponse("admin: specify filters or set purge=true to delete all"))
			return
		}
	}

	n, err := c.auditService.DeleteLogs(filter)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{"deleted": n}))
}

// GetLog godoc
// @Summary Get audit log by id
// @Tags audit
// @Produce json
// @Param id path int true "Log id"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 401 {object} schema.APIResponse
// @Failure 404 {object} schema.APIResponse
// @Router /audit/{id} [get]
// @Security BearerAuth
func (c *AuditController) GetLog(_ context.Context, ctx *app.RequestContext) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}

	log, err := c.auditService.GetLog(id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, schema.ErrorResponse(err.Error()))
		return
	}
	if u := auth.GetCurrentUser(ctx); u != nil && !u.IsAdmin {
		if log.UserID != strconv.FormatInt(u.ID, 10) {
			ctx.JSON(consts.StatusForbidden, schema.ErrorResponse("forbidden"))
			return
		}
	}
	ctx.JSON(http.StatusOK, schema.SuccessResponse(log))
}

func (c *AuditController) GetLogLegacy(_ context.Context, ctx *app.RequestContext) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}
	log, err := c.auditService.GetLog(id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, schema.ErrorResponse(err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, schema.SuccessResponse(log))
}

func queryInt(ctx *app.RequestContext, key string, defaultVal int) int {
	valStr := string(ctx.Query(key))
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}
