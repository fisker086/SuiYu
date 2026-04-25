package controller

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/fisk086/sya/internal/auth"
	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/service"
	"github.com/fisk086/sya/internal/storage"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type ApprovalController struct {
	approvalService *service.ApprovalService
	rbacService     *service.RBACService
	jwtCfg          auth.JWTConfig
	userStore       storage.UserStore
}

func NewApprovalController(approvalService *service.ApprovalService, rbacService *service.RBACService, jwtCfg auth.JWTConfig, userStore storage.UserStore) *ApprovalController {
	return &ApprovalController{approvalService: approvalService, rbacService: rbacService, jwtCfg: jwtCfg, userStore: userStore}
}

func (c *ApprovalController) getUserForMiddleware(userID int64) (*auth.User, error) {
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

func (c *ApprovalController) RegisterRoutes(r *server.Hertz) {
	g := r.Group("/api/v1/approvals")
	if c.userStore != nil {
		g = g.Group("", auth.OptionalJWTMiddleware(c.jwtCfg, c.getUserForMiddleware))
	}
	g.GET("", c.ListRequests)
	g.GET("/:id", c.GetRequest)
	g.POST("/:id/approve", c.Approve)
	g.POST("/:id/reject", c.Reject)
	g.POST("/callback", c.Callback)
}

func (c *ApprovalController) ListRequests(_ context.Context, ctx *app.RequestContext) {
	filter := &storage.ApprovalRequestFilter{
		Status:   string(ctx.Query("status")),
		Page:     queryInt(ctx, "page", 1),
		PageSize: queryInt(ctx, "page_size", 50),
	}

	if agentIDStr := string(ctx.Query("agent_id")); agentIDStr != "" {
		if id, err := strconv.ParseInt(agentIDStr, 10, 64); err == nil {
			filter.AgentID = id
		}
	}

	if sessionID := string(ctx.Query("session_id")); sessionID != "" {
		filter.SessionID = sessionID
	}

	user := auth.GetCurrentUser(ctx)
	if user != nil {
		if string(ctx.Query("my")) == "true" {
			filter.UserID = user.Username
		}
	}

	viewer := c.approvalViewer(auth.GetCurrentUser(ctx))
	reqs, total, err := c.approvalService.ListRequests(filter, viewer)
	if err != nil {
		logger.Error("approvals list failed", "status", filter.Status, "page", filter.Page, "err", err)
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{
		"requests": reqs,
		"total":    total,
		"page":     filter.Page,
	}))
}

func (c *ApprovalController) GetRequest(_ context.Context, ctx *app.RequestContext) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}

	viewer := c.approvalViewer(auth.GetCurrentUser(ctx))
	req, err := c.approvalService.GetRequest(id, viewer)
	if err != nil {
		logger.Warn("approvals get failed", "id", id, "err", err)
		ctx.JSON(http.StatusNotFound, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, schema.SuccessResponse(req))
}

func (c *ApprovalController) Approve(_ context.Context, ctx *app.RequestContext) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}

	user := auth.GetCurrentUser(ctx)
	if user == nil {
		ctx.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}

	hasRole := c.hasApproverRole(user)
	if err := c.approvalService.ValidateApprovePermission(id, user.Username, hasRole); err != nil {
		if errors.Is(err, service.ErrNotApprover) {
			ctx.JSON(http.StatusForbidden, schema.ErrorResponse(err.Error()))
			return
		}
		logger.Error("approvals approve permission check failed", "id", id, "err", err)
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	approverID := user.Username

	comment := string(ctx.FormValue("comment"))
	if err := c.approvalService.Approve(id, approverID, comment); err != nil {
		logger.Error("approvals approve failed", "id", id, "err", err)
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, schema.SuccessResponse(map[string]string{"status": "approved"}))
}

func (c *ApprovalController) Reject(_ context.Context, ctx *app.RequestContext) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}

	user := auth.GetCurrentUser(ctx)
	if user == nil {
		ctx.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}

	hasRole := c.hasApproverRole(user)
	if err := c.approvalService.ValidateApprovePermission(id, user.Username, hasRole); err != nil {
		if errors.Is(err, service.ErrNotApprover) {
			ctx.JSON(http.StatusForbidden, schema.ErrorResponse(err.Error()))
			return
		}
		logger.Error("approvals reject permission check failed", "id", id, "err", err)
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	approverID := user.Username

	comment := string(ctx.FormValue("comment"))
	if err := c.approvalService.Reject(id, approverID, comment); err != nil {
		logger.Error("approvals reject failed", "id", id, "err", err)
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, schema.SuccessResponse(map[string]string{"status": "rejected"}))
}

func (c *ApprovalController) hasApproverRole(user *auth.User) bool {
	if user == nil {
		return false
	}
	if c.rbacService == nil {
		return true
	}
	roles, err := c.rbacService.ListUserRoles(user.ID)
	if err != nil {
		return false
	}
	for _, role := range roles {
		if role.Role != nil && role.Role.IsApprover {
			return true
		}
	}
	return false
}

func (c *ApprovalController) approvalViewer(user *auth.User) *service.ApprovalViewer {
	if user == nil {
		return nil
	}
	return &service.ApprovalViewer{
		Username:        user.Username,
		HasApproverRole: c.hasApproverRole(user),
	}
}

type ApprovalCallbackRequest struct {
	ExternalID string `json:"external_id"`
	Status     string `json:"status"`
	ApproverID string `json:"approver_id"`
	Comment    string `json:"comment"`
}

func (c *ApprovalController) Callback(_ context.Context, ctx *app.RequestContext) {
	var req ApprovalCallbackRequest
	if err := ctx.Bind(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid request"))
		return
	}

	if req.ExternalID == "" {
		ctx.JSON(http.StatusBadRequest, schema.ErrorResponse("external_id is required"))
		return
	}

	filter := &storage.ApprovalRequestFilter{
		ExternalID: req.ExternalID,
	}

	reqs, _, err := c.approvalService.ListRequests(filter, nil)
	if err != nil || len(reqs) == 0 {
		ctx.JSON(http.StatusNotFound, schema.ErrorResponse("approval request not found"))
		return
	}

	approvalReq := reqs[0]
	var updateErr error
	if req.Status == "approved" {
		updateErr = c.approvalService.Approve(approvalReq.ID, req.ApproverID, req.Comment)
	} else if req.Status == "rejected" {
		updateErr = c.approvalService.Reject(approvalReq.ID, req.ApproverID, req.Comment)
	} else {
		ctx.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid status"))
		return
	}

	if updateErr != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(updateErr.Error()))
		return
	}

	ctx.JSON(http.StatusOK, schema.SuccessResponse(map[string]string{"status": req.Status}))
}
