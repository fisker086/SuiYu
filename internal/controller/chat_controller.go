package controller

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fisk086/sya/internal/auth"
	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/service"
	"github.com/fisk086/sya/internal/storage"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/google/uuid"
)

// sseFlushChunk：SSE 每次 Write 的最大字节数，随后立刻 Flush。单次大块 Write 常被框架或反向代理缓冲，
// 导致前端直到流结束才收到正文，打字机效果不明显。
const sseFlushChunk = 64

func flushSSEBytes(hc *app.RequestContext, p []byte) {
	for i := 0; i < len(p); i += sseFlushChunk {
		j := i + sseFlushChunk
		if j > len(p) {
			j = len(p)
		}
		_, _ = hc.Write(p[i:j])
		hc.Flush()
	}
}

type ChatController struct {
	chatService *service.ChatService
	jwtCfg      auth.JWTConfig
	userStore   storage.UserStore
	rbacService *service.RBACService
	uploadDir   string
}

func NewChatController(chatService *service.ChatService, jwtCfg auth.JWTConfig, userStore storage.UserStore, uploadDir string, rbacService ...*service.RBACService) *ChatController {
	ctrl := &ChatController{chatService: chatService, jwtCfg: jwtCfg, userStore: userStore, uploadDir: uploadDir}
	if len(rbacService) > 0 {
		ctrl.rbacService = rbacService[0]
	}
	return ctrl
}

func (c *ChatController) getUserForMiddleware(userID int64) (*auth.User, error) {
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

func (c *ChatController) RegisterRoutes(r *server.Hertz) {
	chat := r.Group("/api/v1/chat")

	if c.userStore == nil {
		chat.POST("", c.Chat)
		chat.POST("/stream", c.StreamChat)
		chat.GET("/sessions/:session_id", c.chatSessionsUnavailable)
		chat.GET("/sessions/:session_id/messages", c.chatSessionsUnavailable)
		chat.GET("/sessions", c.chatSessionsUnavailable)
		chat.POST("/sessions", c.chatSessionsUnavailable)
		return
	}
	protected := chat.Group("", auth.JWTMiddleware(c.jwtCfg, c.getUserForMiddleware))
	protected.POST("", c.Chat)
	protected.POST("/stream", c.StreamChat)
	protected.POST("/stop", c.StopChat)
	protected.POST("/upload", c.UploadFile)
	protected.POST("/tool_result", c.SubmitToolResult)
	protected.POST("/tool_result/stream", c.SubmitToolResultStream)
	protected.GET("/sessions/:session_id", c.GetSession)
	protected.GET("/sessions/:session_id/messages", c.ListSessionMessages)
	protected.PUT("/sessions/:session_id", c.UpdateSession)
	protected.DELETE("/sessions/:session_id", c.DeleteSession)
	protected.GET("/sessions", c.ListSessions)
	protected.POST("/sessions", c.CreateSession)
	protected.GET("/stats", c.GetStats)
	protected.GET("/recent", c.GetRecentChats)
	protected.GET("/activity", c.GetActivity)

	// Chat groups: handlers in chat_group_controller.go (register /groups/stream before /groups/:id)
	protected.POST("/groups/stream", c.StreamGroupChat)
	protected.GET("/groups", c.ListGroups)
	protected.POST("/groups", c.CreateGroup)
	protected.GET("/groups/:id", c.GetGroup)
	protected.PUT("/groups/:id", c.UpdateGroup)
	protected.DELETE("/groups/:id", c.DeleteGroup)
}

func (c *ChatController) chatSessionsUnavailable(ctx context.Context, hc *app.RequestContext) {
	hc.JSON(http.StatusServiceUnavailable, schema.ErrorResponse("chat sessions require DATABASE_URL and user store"))
}

func (c *ChatController) checkAgentAccess(ctx context.Context, hc *app.RequestContext, agentID int64) bool {
	if agentID <= 0 {
		return true
	}
	user := auth.GetCurrentUser(hc)
	if user == nil {
		return false
	}
	agent, err := c.chatService.GetAgent(ctx, agentID)
	if err != nil || agent == nil {
		logger.Warn("agent not found for access check", "agent_id", agentID)
		return false
	}
	return c.rbacService.CheckAgentAccess(ctx, user.ID, agent.Name, user.IsAdmin)
}

// GetSession returns one session row (includes group_id for group-chat URL restore).
func (c *ChatController) GetSession(ctx context.Context, hc *app.RequestContext) {
	user := auth.GetCurrentUser(hc)
	if user == nil {
		hc.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	sid := hc.Param("session_id")
	if sid == "" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("session_id required"))
		return
	}
	sess, err := c.chatService.GetChatSession(ctx, sid)
	if err != nil {
		if errors.Is(err, storage.ErrSessionNotFound) {
			hc.JSON(http.StatusNotFound, schema.ErrorResponse("session not found"))
			return
		}
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	if sess.UserID != strconv.FormatInt(user.ID, 10) {
		hc.JSON(http.StatusForbidden, schema.ErrorResponse("forbidden"))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(sess))
}

// @Summary List messages in a session
// @Description Chronological history for your session (session must belong to the authenticated user).
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param session_id path string true "Session ID from POST /chat/sessions"
// @Param limit query int false "Max messages (default 100, max 500 without offset; max 2000 with offset)"
// @Param offset query int false "When set (including 0), returns chronological page from start; omit for most-recent messages only"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 401 {object} schema.APIResponse
// @Failure 403 {object} schema.APIResponse
// @Failure 404 {object} schema.APIResponse
// @Router /chat/sessions/{session_id}/messages [get]
func (c *ChatController) ListSessionMessages(ctx context.Context, hc *app.RequestContext) {
	user := auth.GetCurrentUser(hc)
	if user == nil {
		hc.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	sid := hc.Param("session_id")
	if sid == "" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("session_id required"))
		return
	}
	sess, sessErr := c.chatService.GetChatSession(ctx, sid)
	if sessErr != nil {
		if errors.Is(sessErr, storage.ErrSessionNotFound) {
			hc.JSON(http.StatusNotFound, schema.ErrorResponse("session not found"))
			return
		}
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(sessErr.Error()))
		return
	}
	if sess.UserID != strconv.FormatInt(user.ID, 10) {
		hc.JSON(http.StatusForbidden, schema.ErrorResponse("forbidden"))
		return
	}
	limit := parseIntQueryDefault(hc, "limit", 100)
	var msgs []schema.ChatHistoryMessage
	var err error
	if hc.Query("offset") == "" {
		msgs, err = c.chatService.ListRecentSessionMessages(ctx, sid, limit)
	} else {
		offset := parseIntQueryDefault(hc, "offset", 0)
		if offset < 0 {
			offset = 0
		}
		msgs, err = c.chatService.ListSessionMessagesPage(ctx, sid, offset, limit)
	}
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(msgs))
}

// UpdateSession sets session title (owner only).
func (c *ChatController) UpdateSession(ctx context.Context, hc *app.RequestContext) {
	user := auth.GetCurrentUser(hc)
	if user == nil {
		hc.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	sid := hc.Param("session_id")
	if sid == "" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("session_id required"))
		return
	}
	var req schema.UpdateChatSessionRequest
	if err := hc.BindJSON(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid JSON"))
		return
	}
	uid := strconv.FormatInt(user.ID, 10)
	err := c.chatService.UpdateChatSessionTitle(ctx, sid, uid, req.Title)
	if err != nil {
		if errors.Is(err, storage.ErrSessionNotFound) {
			hc.JSON(http.StatusNotFound, schema.ErrorResponse("session not found"))
			return
		}
		if errors.Is(err, storage.ErrSessionForbidden) {
			hc.JSON(http.StatusForbidden, schema.ErrorResponse("forbidden"))
			return
		}
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	sess, err := c.chatService.GetChatSession(ctx, sid)
	if err != nil {
		hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{"ok": true}))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(sess))
}

// DeleteSession removes a chat session and its stored messages (owner only).
func (c *ChatController) DeleteSession(ctx context.Context, hc *app.RequestContext) {
	user := auth.GetCurrentUser(hc)
	if user == nil {
		hc.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	sid := hc.Param("session_id")
	if sid == "" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("session_id required"))
		return
	}
	uid := strconv.FormatInt(user.ID, 10)
	err := c.chatService.DeleteChatSession(ctx, sid, uid)
	if err != nil {
		if errors.Is(err, storage.ErrSessionNotFound) {
			hc.JSON(http.StatusNotFound, schema.ErrorResponse("session not found"))
			return
		}
		if errors.Is(err, storage.ErrSessionForbidden) {
			hc.JSON(http.StatusForbidden, schema.ErrorResponse("forbidden"))
			return
		}
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]any{"deleted": true}))
}

// @Summary List chat sessions
// @Description Sessions for the given agent belonging to the authenticated user (newest activity first).
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param agent_id query int true "Agent ID"
// @Param limit query int false "Max sessions (default 50, max 500)"
// @Param offset query int false "Skip this many sessions (pagination, default 0)"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 401 {object} schema.APIResponse
// @Router /chat/sessions [get]
func (c *ChatController) ListSessions(ctx context.Context, hc *app.RequestContext) {
	user := auth.GetCurrentUser(hc)
	if user == nil {
		hc.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	agentID := parseInt64Query(hc, "agent_id")
	if agentID < 1 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("agent_id is required and must be >= 1"))
		return
	}
	limit := parseIntQueryDefault(hc, "limit", 50)
	offset := parseIntQueryDefault(hc, "offset", 0)
	if offset < 0 {
		offset = 0
	}
	uid := strconv.FormatInt(user.ID, 10)
	list, err := c.chatService.ListChatSessions(ctx, agentID, uid, limit, offset)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(list))
}

// @Summary Create chat session
// @Description Creates a session scoped to the JWT user and agent. User identity is taken from the token, not the request body.
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body schema.CreateChatSessionRequest true "Agent id"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 401 {object} schema.APIResponse
// @Failure 404 {object} schema.APIResponse
// @Router /chat/sessions [post]
func (c *ChatController) CreateSession(ctx context.Context, hc *app.RequestContext) {
	user := auth.GetCurrentUser(hc)
	if user == nil {
		hc.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	var req schema.CreateChatSessionRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	uid := strconv.FormatInt(user.ID, 10)
	sess, err := c.chatService.CreateChatSession(ctx, req.AgentID, uid, req.GroupID)
	if err != nil {
		if errors.Is(err, storage.ErrAgentNotFound) {
			hc.JSON(http.StatusNotFound, schema.ErrorResponse("agent not found"))
			return
		}
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(sess))
}

// @Summary Stop chat stream
// @Description Abort an ongoing streaming chat for the given session.
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body schema.StopChatRequest true "Session ID"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 401 {object} schema.APIResponse
// @Router /chat/stop [post]
func (c *ChatController) StopChat(ctx context.Context, hc *app.RequestContext) {
	user := auth.GetCurrentUser(hc)
	if user == nil {
		hc.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	var req schema.StopChatRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	if req.SessionID == "" {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("session_id is required"))
		return
	}
	c.chatService.StopStream(req.SessionID)
	hc.JSON(http.StatusOK, schema.SuccessResponse(nil))
}

// @Summary Chat with an agent or workflow (JSON)
// @Description Non-streaming reply. Use agent_id or workflow_id, not both. For token streaming use POST /chat/stream.
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param req body schema.ChatRequest true "Chat request"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 401 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /chat [post]
func (c *ChatController) Chat(ctx context.Context, hc *app.RequestContext) {
	var req schema.ChatRequest
	if err := hc.BindAndValidate(&req); err != nil {
		logger.Warn("chat request validation failed", "err", err)
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	beforeCT := req.ClientType
	ua := string(hc.GetHeader("User-Agent"))
	req.ClientType = NormalizeClientTypeFromUserAgent(req.ClientType, ua)
	if req.ClientType != beforeCT {
		logger.Info("client_type reconciled from User-Agent", "from", beforeCT, "to", req.ClientType, "ua_len", len(ua))
	}

	if c.rbacService != nil && !c.checkAgentAccess(ctx, hc, req.AgentID) {
		hc.JSON(http.StatusForbidden, schema.ErrorResponse("no permission to use this agent"))
		return
	}

	msgLen := len(strings.TrimSpace(req.Message))
	logger.Info("chat request",
		"agent_id", req.AgentID,
		"workflow_id", req.WorkflowID,
		"message_len", msgLen,
		"has_session_id", req.SessionID != "",
	)

	c.prepareChatSession(ctx, hc, &req)

	chatUserID := chatUserIDFromContext(hc, c.userStore != nil)
	if err := c.fillImagePartsFromUploadURLs(&req, chatUserID); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	if err := c.appendChatFileContentsToMessage(&req, chatUserID); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	resp, err := c.chatService.Chat(ctx, &req, chatUserID)
	if err != nil {
		logger.Error("chat handler failed", "agent_id", req.AgentID, "workflow_id", req.WorkflowID, "err", err)
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	logger.Info("chat completed", "agent_id", resp.AgentID)
	hc.JSON(http.StatusOK, schema.SuccessResponse(resp))
}

// @Summary Chat stream (SSE)
// @Description Server-Sent Events stream of assistant tokens. Same body as POST /chat. workflow_id streaming is not supported yet.
// @Tags chat
// @Accept json
// @Produce text/event-stream
// @Security BearerAuth
// @Param req body schema.ChatRequest true "Chat request"
// @Success 200 {string} string "SSE body"
// @Failure 400 {object} schema.APIResponse
// @Failure 401 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /chat/stream [post]
func (c *ChatController) StreamChat(ctx context.Context, hc *app.RequestContext) {
	var req schema.ChatRequest
	if err := hc.BindAndValidate(&req); err != nil {
		logger.Warn("chat stream request validation failed", "err", err)
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	beforeCT := req.ClientType
	ua := string(hc.GetHeader("User-Agent"))
	req.ClientType = NormalizeClientTypeFromUserAgent(req.ClientType, ua)
	if req.ClientType != beforeCT {
		logger.Info("client_type reconciled from User-Agent", "from", beforeCT, "to", req.ClientType, "ua_len", len(ua))
	}

	if c.rbacService != nil && !c.checkAgentAccess(ctx, hc, req.AgentID) {
		hc.JSON(http.StatusForbidden, schema.ErrorResponse("no permission to use this agent"))
		return
	}

	logger.Info("chat stream request",
		"agent_id", req.AgentID,
		"workflow_id", req.WorkflowID,
		"message_len", len(strings.TrimSpace(req.Message)),
		"has_session_id", req.SessionID != "",
	)

	c.prepareChatSession(ctx, hc, &req)
	chatUserID := chatUserIDFromContext(hc, c.userStore != nil)
	if err := c.fillImagePartsFromUploadURLs(&req, chatUserID); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	if err := c.appendChatFileContentsToMessage(&req, chatUserID); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	c.handleStream(ctx, hc, &req)
}

func chatUserIDFromContext(hc *app.RequestContext, requireUser bool) string {
	if !requireUser {
		return ""
	}
	u := auth.GetCurrentUser(hc)
	if u == nil {
		return ""
	}
	return strconv.FormatInt(u.ID, 10)
}

// prepareChatSession sets req.SessionID when the client omitted it but JWT user is present.
// fillImagePartsFromUploadURLs loads bytes from POST /chat/upload paths when the client sends image_urls only
// (no JSON base64). Paths must be /api/v1/chat/files/{uid}/{file} with uid matching the authenticated user.
func (c *ChatController) fillImagePartsFromUploadURLs(req *schema.ChatRequest, chatUserID string) error {
	if req == nil || len(req.ImageParts) > 0 || len(req.ImageURLs) == 0 {
		return nil
	}
	if chatUserID == "" {
		return fmt.Errorf("image_urls require authentication")
	}
	base := filepath.Clean(c.uploadDir)
	for _, raw := range req.ImageURLs {
		rel, err := chatUploadRelPath(raw, chatUserID)
		if err != nil {
			return err
		}
		full := filepath.Join(base, rel)
		fullClean := filepath.Clean(full)
		if !strings.HasPrefix(fullClean, base+string(os.PathSeparator)) && fullClean != base {
			return fmt.Errorf("invalid image path")
		}
		data, err := os.ReadFile(fullClean)
		if err != nil {
			return fmt.Errorf("read image: %w", err)
		}
		if len(data) == 0 {
			return fmt.Errorf("empty image file")
		}
		ext := strings.ToLower(filepath.Ext(rel))
		mime := mimeFromChatUploadExt(ext)
		req.ImageParts = append(req.ImageParts, schema.ChatImagePart{
			Base64: base64.StdEncoding.EncodeToString(data),
			Mime:   mime,
		})
	}
	return nil
}

func chatUploadRelPath(rawURL, chatUserID string) (string, error) {
	s := strings.TrimSpace(rawURL)
	const marker = "/chat/files/"
	i := strings.Index(s, marker)
	if i < 0 {
		return "", fmt.Errorf("invalid image url")
	}
	rel := strings.Trim(s[i+len(marker):], "/")
	parts := strings.Split(rel, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid image path")
	}
	if parts[0] != chatUserID {
		return "", fmt.Errorf("forbidden image path")
	}
	fn := parts[1]
	if fn == "" || strings.Contains(fn, "..") {
		return "", fmt.Errorf("invalid image path")
	}
	return filepath.Join(parts[0], fn), nil
}

func mimeFromChatUploadExt(ext string) string {
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

func (c *ChatController) prepareChatSession(ctx context.Context, hc *app.RequestContext, req *schema.ChatRequest) {
	var userID string
	if c.userStore != nil {
		jwtUser := auth.GetCurrentUser(hc)
		if jwtUser != nil {
			userID = strconv.FormatInt(jwtUser.ID, 10)
		}
	}
	if req.SessionID == "" && userID != "" && req.AgentID > 0 {
		sess, err := c.chatService.CreateChatSession(ctx, req.AgentID, userID, 0)
		if err == nil {
			req.SessionID = sess.SessionID
		} else {
			logger.Warn("chat auto-create session skipped", "agent_id", req.AgentID, "err", err)
		}
	}
}

func (c *ChatController) handleStream(ctx context.Context, hc *app.RequestContext, req *schema.ChatRequest) {
	start := time.Now()
	chatUserID := chatUserIDFromContext(hc, c.userStore != nil)
	reader, err := c.chatService.ChatStream(ctx, req, chatUserID)
	if err != nil {
		logger.Error("chat stream failed", "agent_id", req.AgentID, "err", err)
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	defer reader.Close()

	hc.SetStatusCode(http.StatusOK)
	hc.Response.Header.Set("Content-Type", "text/event-stream")
	hc.Response.Header.Set("Cache-Control", "no-cache")
	hc.Response.Header.Set("Connection", "keep-alive")
	hc.Response.Header.Set("X-Accel-Buffering", "no")
	if req.SessionID != "" {
		hc.Response.Header.Set("X-Session-ID", req.SessionID)
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
			logger.Info("chat stream response closed",
				"agent_id", req.AgentID,
				"session_id", req.SessionID,
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
				logger.Warn("chat stream read error", "agent_id", req.AgentID, "err", readErr)
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
	logger.Info("chat stream response closed",
		"agent_id", req.AgentID,
		"session_id", req.SessionID,
		"duration_ms", elapsed,
		"bytes_written", written,
		"end_reason", endReason,
	)
}

func (c *ChatController) GetStats(ctx context.Context, hc *app.RequestContext) {
	user := auth.GetCurrentUser(hc)
	if user == nil {
		hc.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	uid := strconv.FormatInt(user.ID, 10)
	stats, err := c.chatService.GetStats(ctx, uid, user.IsAdmin)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(stats))
}

func (c *ChatController) GetRecentChats(ctx context.Context, hc *app.RequestContext) {
	user := auth.GetCurrentUser(hc)
	if user == nil {
		hc.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	uid := strconv.FormatInt(user.ID, 10)
	chats, err := c.chatService.GetRecentChats(ctx, uid, 10)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(chats))
}

func (c *ChatController) GetActivity(ctx context.Context, hc *app.RequestContext) {
	user := auth.GetCurrentUser(hc)
	if user == nil {
		hc.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	uid := strconv.FormatInt(user.ID, 10)
	activity, err := c.chatService.GetChatActivity(ctx, uid, 7)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(activity))
}

func (c *ChatController) UploadFile(ctx context.Context, hc *app.RequestContext) {
	user := auth.GetCurrentUser(hc)
	if user == nil {
		hc.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}

	file, err := hc.FormFile("file")
	if err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("no file provided"))
		return
	}

	if file.Size > 10*1024*1024 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("file too large, max 10MB"))
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
		".pdf": true, ".txt": true, ".md": true, ".json": true,
	}
	if !allowedExts[ext] {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("unsupported file type"))
		return
	}

	uid := strconv.FormatInt(user.ID, 10)
	uploadDir := filepath.Join(c.uploadDir, uid)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse("failed to create upload directory"))
		return
	}

	filename := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), uuid.New().String()[:8], ext)
	uploadPath := filepath.Join(uploadDir, filename)

	src, err := file.Open()
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse("failed to open file"))
		return
	}
	defer src.Close()

	dst, err := os.Create(uploadPath)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse("failed to create file"))
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse("failed to save file"))
		return
	}

	// Always use URL path segments with '/' (never filepath separators) so clients and fillImagePartsFromUploadURLs parse reliably on Windows.
	fileURL := fmt.Sprintf("/api/v1/chat/files/%s/%s", uid, filename)
	hc.JSON(http.StatusOK, schema.SuccessResponse(schema.UploadResponse{
		URL:      fileURL,
		Filename: file.Filename,
		Size:     file.Size,
		MimeType: file.Header.Get("Content-Type"),
	}))
}

func (c *ChatController) SubmitToolResult(ctx context.Context, hc *app.RequestContext) {
	var req schema.SubmitToolResultRequest
	if err := hc.BindAndValidate(&req); err != nil {
		logger.Warn("tool result validation failed", "err", err)
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	ua := string(hc.UserAgent())
	req.ClientType = NormalizeClientTypeFromUserAgent(req.ClientType, ua)

	chatUserID := chatUserIDFromContext(hc, c.userStore != nil)
	resp, err := c.chatService.SubmitToolResult(ctx, &req, chatUserID)
	if err != nil {
		logger.Error("submit tool result failed", "session_id", req.SessionID, "call_id", logger.CallIDForLog(req.CallID), "err", err)
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(resp))
}

func (c *ChatController) SubmitToolResultStream(ctx context.Context, hc *app.RequestContext) {
	var req schema.SubmitToolResultRequest
	if err := hc.BindAndValidate(&req); err != nil {
		logger.Warn("tool result stream validation failed", "err", err)
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	ua := string(hc.UserAgent())
	req.ClientType = NormalizeClientTypeFromUserAgent(req.ClientType, ua)

	chatUserID := chatUserIDFromContext(hc, c.userStore != nil)
	reader, err := c.chatService.SubmitToolResultStream(ctx, &req, chatUserID)
	if err != nil {
		logger.Error("submit tool result stream failed", "session_id", req.SessionID, "call_id", logger.CallIDForLog(req.CallID), "err", err)
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	defer reader.Close()

	hc.SetStatusCode(http.StatusOK)
	hc.Response.Header.Set("Content-Type", "text/event-stream")
	hc.Response.Header.Set("Cache-Control", "no-cache")
	hc.Response.Header.Set("Connection", "keep-alive")
	hc.Response.Header.Set("X-Accel-Buffering", "no")

	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		n, readErr := reader.Read(buf)
		if n > 0 {
			flushSSEBytes(hc, buf[:n])
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				logger.Warn("tool result stream read error", "session_id", req.SessionID, "err", readErr)
			}
			break
		}
	}
	hc.Flush()
}
