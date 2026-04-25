package controller

import (
	"context"
	"strconv"

	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/service"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type MessageController struct {
	messageService *service.MessageService
}

func NewMessageController(messageService *service.MessageService) *MessageController {
	return &MessageController{messageService: messageService}
}

func (ctrl *MessageController) RegisterRoutes(h *server.Hertz) {
	v1 := h.Group("/api/v1")

	v1.POST("/messages/send", ctrl.SendMessage)
	v1.POST("/messages/span", ctrl.SendSpan)
	v1.GET("/messages", ctrl.ListMessages)

	v1.POST("/message-channels", ctrl.CreateChannel)
	v1.GET("/message-channels", ctrl.ListChannels)
	v1.GET("/message-channels/:id", ctrl.GetChannel)
	v1.PUT("/message-channels/:id", ctrl.UpdateChannel)
	v1.DELETE("/message-channels/:id", ctrl.DeleteChannel)

	v1.POST("/a2a-cards", ctrl.CreateA2ACard)
	v1.GET("/a2a-cards", ctrl.ListA2ACards)
	v1.GET("/a2a-cards/:id", ctrl.GetA2ACard)
	v1.DELETE("/a2a-cards/:id", ctrl.DeleteA2ACard)
}

func (ctrl *MessageController) SendMessage(c context.Context, ctx *app.RequestContext) {
	var req schema.SendMessageRequest
	if err := ctx.BindAndValidate(&req); err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	resp, err := ctrl.messageService.SendMessage(c, &req)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(resp))
}

func (ctrl *MessageController) SendSpan(c context.Context, ctx *app.RequestContext) {
	var req schema.MessageSpanRequest
	if err := ctx.BindAndValidate(&req); err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	resp, err := ctrl.messageService.SendSpan(c, &req)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(resp))
}

func (ctrl *MessageController) ListMessages(c context.Context, ctx *app.RequestContext) {
	req := schema.ListMessagesRequest{
		Limit:  parseIntQueryDefault(ctx, "limit", 50),
		Offset: parseIntQueryDefault(ctx, "offset", 0),
	}

	if ch := ctx.Query("channel_id"); ch != "" {
		if id, err := strconv.ParseInt(ch, 10, 64); err == nil {
			req.ChannelID = id
		}
	}
	if ag := ctx.Query("agent_id"); ag != "" {
		if id, err := strconv.ParseInt(ag, 10, 64); err == nil {
			req.AgentID = id
		}
	}
	if sid := ctx.Query("session_id"); sid != "" {
		req.SessionID = sid
	}
	if st := ctx.Query("status"); st != "" {
		req.Status = st
	}

	messages, total, err := ctrl.messageService.ListMessages(&req)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(map[string]any{
		"messages": messages,
		"total":    total,
		"limit":    req.Limit,
		"offset":   req.Offset,
	}))
}

func (ctrl *MessageController) CreateChannel(c context.Context, ctx *app.RequestContext) {
	var req schema.CreateMessageChannelRequest
	if err := ctx.BindAndValidate(&req); err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	ch, err := ctrl.messageService.CreateChannel(&req)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(ch))
}

func (ctrl *MessageController) ListChannels(c context.Context, ctx *app.RequestContext) {
	agentID := parseInt64Query(ctx, "agent_id")

	channels, err := ctrl.messageService.ListChannels(agentID)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(channels))
}

func (ctrl *MessageController) GetChannel(c context.Context, ctx *app.RequestContext) {
	id := parseInt64Param(ctx, "id")

	channels, err := ctrl.messageService.ListChannels(0)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	for _, ch := range channels {
		if ch.ID == id {
			ctx.JSON(consts.StatusOK, schema.SuccessResponse(ch))
			return
		}
	}

	ctx.JSON(consts.StatusNotFound, schema.ErrorResponse("channel not found"))
}

func (ctrl *MessageController) UpdateChannel(c context.Context, ctx *app.RequestContext) {
	id := parseInt64Param(ctx, "id")

	var req schema.UpdateMessageChannelRequest
	if err := ctx.BindAndValidate(&req); err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	ch, err := ctrl.messageService.UpdateChannel(id, &req)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(ch))
}

func (ctrl *MessageController) DeleteChannel(c context.Context, ctx *app.RequestContext) {
	id := parseInt64Param(ctx, "id")

	if err := ctrl.messageService.DeleteChannel(id); err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(nil))
}

func (ctrl *MessageController) CreateA2ACard(c context.Context, ctx *app.RequestContext) {
	var req schema.CreateA2ACardRequest
	if err := ctx.BindAndValidate(&req); err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	card, err := ctrl.messageService.CreateA2ACard(&req)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(card))
}

func (ctrl *MessageController) ListA2ACards(c context.Context, ctx *app.RequestContext) {
	agentID := parseInt64Query(ctx, "agent_id")

	cards, err := ctrl.messageService.ListA2ACards(agentID)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(cards))
}

func (ctrl *MessageController) GetA2ACard(c context.Context, ctx *app.RequestContext) {
	id := parseInt64Param(ctx, "id")

	card, err := ctrl.messageService.GetA2ACard(id)
	if err != nil {
		ctx.JSON(consts.StatusNotFound, schema.ErrorResponse("card not found"))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(card))
}

func (ctrl *MessageController) DeleteA2ACard(c context.Context, ctx *app.RequestContext) {
	id := parseInt64Param(ctx, "id")

	if err := ctrl.messageService.DeleteA2ACard(id); err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(nil))
}
