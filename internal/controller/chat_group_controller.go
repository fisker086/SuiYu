// Chat group HTTP handlers: POST /chat/groups/stream (SSE) and CRUD /chat/groups[/:id].
// Routes are registered from ChatController.RegisterRoutes in chat_controller.go.
package controller

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fisk086/sya/internal/auth"
	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/schema"
	"github.com/cloudwego/hertz/pkg/app"
)

// StreamGroupChat is POST /chat/groups/stream — SSE for group chat without agent_id/workflow_id on the legacy /chat/stream contract.
func (c *ChatController) StreamGroupChat(ctx context.Context, hc *app.RequestContext) {
	var req schema.GroupChatStreamRequest
	if err := hc.BindAndValidate(&req); err != nil {
		logger.Warn("group chat stream request validation failed", "err", err)
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	beforeCT := req.ClientType
	ua := string(hc.GetHeader("User-Agent"))
	req.ClientType = NormalizeClientTypeFromUserAgent(req.ClientType, ua)
	if req.ClientType != beforeCT {
		logger.Info("client_type reconciled from User-Agent", "from", beforeCT, "to", req.ClientType, "ua_len", len(ua))
	}

	for _, aid := range req.Mentions {
		if c.rbacService != nil && !c.checkAgentAccess(ctx, hc, aid) {
			hc.JSON(http.StatusForbidden, schema.ErrorResponse("no permission to use a mentioned agent"))
			return
		}
	}

	cr := req.ToChatRequest()
	chatUserID := chatUserIDFromContext(hc, c.userStore != nil)
	if err := c.fillImagePartsFromUploadURLs(cr, chatUserID); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	if err := c.appendChatFileContentsToMessage(cr, chatUserID); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	logger.Info("group chat stream request",
		"group_id", req.GroupID,
		"mentions_len", len(req.Mentions),
		"message_len", len(strings.TrimSpace(req.Message)),
		"has_session_id", req.SessionID != "",
	)

	start := time.Now()
	reader, err := c.chatService.GroupChatStream(ctx, cr, chatUserID)
	if err != nil {
		logger.Error("group chat stream failed", "group_id", req.GroupID, "err", err)
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	defer reader.Close()

	hc.SetStatusCode(http.StatusOK)
	hc.Response.Header.Set("Content-Type", "text/event-stream")
	hc.Response.Header.Set("Cache-Control", "no-cache")
	hc.Response.Header.Set("Connection", "keep-alive")
	hc.Response.Header.Set("X-Accel-Buffering", "no")
	if cr.SessionID != "" {
		hc.Response.Header.Set("X-Session-ID", cr.SessionID)
	}

	buf := make([]byte, 4096)
	var written int64
	var readErr error
	for {
		select {
		case <-ctx.Done():
			elapsed := time.Since(start).Milliseconds()
			hc.Response.Header.Set("X-Duration-MS", strconv.FormatInt(elapsed, 10))
			hc.Flush()
			logger.Info("group chat stream response closed",
				"group_id", req.GroupID,
				"session_id", cr.SessionID,
				"duration_ms", elapsed,
				"bytes_written", written,
				"end_reason", "context_canceled",
				"ctx_err", ctx.Err(),
			)
			return
		default:
		}
		var n int
		n, readErr = reader.Read(buf)
		if n > 0 {
			written += int64(n)
			flushSSEBytes(hc, buf[:n])
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				logger.Warn("group chat stream read error", "group_id", req.GroupID, "err", readErr)
			}
			break
		}
	}

	elapsed := time.Since(start).Milliseconds()
	hc.Response.Header.Set("X-Duration-MS", strconv.FormatInt(elapsed, 10))
	hc.Flush()
	endReason := "eof"
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		endReason = "read_error"
	}
	logger.Info("group chat stream response closed",
		"group_id", req.GroupID,
		"session_id", cr.SessionID,
		"duration_ms", elapsed,
		"bytes_written", written,
		"end_reason", endReason,
	)
}

// @Summary List chat groups
// @Description Get all chat groups for the current user
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Success 200 {object} schema.APIResponse
// @Router /chat/groups [get]
func (c *ChatController) ListGroups(ctx context.Context, hc *app.RequestContext) {
	user := auth.GetCurrentUser(hc)
	if user == nil {
		hc.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}

	groups, err := c.chatService.ListChatGroups(ctx, user.Username)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(groups))
}

// @Summary Create chat group
// @Description Create a new chat group with selected agents
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param request body schema.CreateGroupRequest true "Group creation request"
// @Success 200 {object} schema.APIResponse
// @Router /chat/groups [post]
func (c *ChatController) CreateGroup(ctx context.Context, hc *app.RequestContext) {
	user := auth.GetCurrentUser(hc)
	if user == nil {
		hc.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}

	var req schema.CreateGroupRequest
	if err := hc.Bind(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	if req.Name == "" || len(req.AgentIDs) == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("name and agent_ids are required"))
		return
	}

	group, err := c.chatService.CreateChatGroup(ctx, &req, user.Username)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(group))
}

// @Summary Get chat group
// @Description Get a specific chat group by ID
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param id path int true "Group ID"
// @Success 200 {object} schema.APIResponse
// @Router /chat/groups/{id} [get]
func (c *ChatController) GetGroup(ctx context.Context, hc *app.RequestContext) {
	idStr := hc.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid group id"))
		return
	}

	group, err := c.chatService.GetChatGroup(ctx, id)
	if err != nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("group not found"))
		return
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(group))
}

// @Summary Update chat group
// @Description Update group name or members
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param id path int true "Group ID"
// @Param request body schema.UpdateGroupRequest true "Group update request"
// @Success 200 {object} schema.APIResponse
// @Router /chat/groups/{id} [put]
func (c *ChatController) UpdateGroup(ctx context.Context, hc *app.RequestContext) {
	idStr := hc.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid group id"))
		return
	}

	var req schema.UpdateGroupRequest
	if err := hc.Bind(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	group, err := c.chatService.UpdateChatGroup(ctx, id, &req)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(group))
}

// @Summary Delete chat group
// @Description Delete a chat group
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param id path int true "Group ID"
// @Success 200 {object} schema.APIResponse
// @Router /chat/groups/{id} [delete]
func (c *ChatController) DeleteGroup(ctx context.Context, hc *app.RequestContext) {
	idStr := hc.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid group id"))
		return
	}

	if err := c.chatService.DeleteChatGroup(ctx, id); err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(nil))
}
