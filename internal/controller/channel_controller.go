package controller

import (
	"context"
	"net/http"
	"strconv"

	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/service"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

type ChannelController struct {
	svc *service.ChannelService
}

func NewChannelController(svc *service.ChannelService) *ChannelController {
	return &ChannelController{svc: svc}
}

func (c *ChannelController) RegisterRoutes(r *server.Hertz) {
	g := r.Group("/api/v1/channels")
	g.GET("", c.List)
	g.POST("", c.Create)
	g.GET("/:id", c.Get)
	g.PUT("/:id", c.Update)
	g.DELETE("/:id", c.Delete)
	g.POST("/:id/test", c.Test)
}

func (c *ChannelController) List(ctx context.Context, hc *app.RequestContext) {
	list, err := c.svc.List()
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(list))
}

func (c *ChannelController) Get(ctx context.Context, hc *app.RequestContext) {
	id, err := strconv.ParseInt(hc.Param("id"), 10, 64)
	if err != nil || id < 1 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}
	ch, err := c.svc.Get(id)
	if err != nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(ch))
}

func (c *ChannelController) Create(ctx context.Context, hc *app.RequestContext) {
	var req schema.CreateChannelRequest
	if err := hc.BindJSON(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	ch, err := c.svc.Create(&req)
	if err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusCreated, schema.SuccessResponse(ch))
}

func (c *ChannelController) Update(ctx context.Context, hc *app.RequestContext) {
	id, err := strconv.ParseInt(hc.Param("id"), 10, 64)
	if err != nil || id < 1 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}
	var req schema.UpdateChannelRequest
	if err := hc.BindJSON(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	ch, err := c.svc.Update(id, &req)
	if err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(ch))
}

func (c *ChannelController) Delete(ctx context.Context, hc *app.RequestContext) {
	id, err := strconv.ParseInt(hc.Param("id"), 10, 64)
	if err != nil || id < 1 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}
	if err := c.svc.Delete(id); err != nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]string{"ok": "true"}))
}

func (c *ChannelController) Test(ctx context.Context, hc *app.RequestContext) {
	id, err := strconv.ParseInt(hc.Param("id"), 10, 64)
	if err != nil || id < 1 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}
	var req schema.TestChannelRequest
	if err := hc.BindJSON(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	msg := req.Message
	if msg == "" {
		msg = "channel test"
	}
	if err := c.svc.SendTest(ctx, id, msg); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]string{"status": "sent"}))
}
