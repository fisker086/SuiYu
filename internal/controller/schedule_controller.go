package controller

import (
	"context"
	"errors"
	"strconv"

	"github.com/fisk086/sya/internal/auth"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/scheduler"
	"github.com/fisk086/sya/internal/service"
	"github.com/fisk086/sya/internal/storage"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type ScheduleController struct {
	scheduleService *service.ScheduleService
	jwtCfg          auth.JWTConfig
	userStore       storage.UserStore
}

func NewScheduleController(scheduleService *service.ScheduleService, jwtCfg auth.JWTConfig, userStore storage.UserStore) *ScheduleController {
	return &ScheduleController{scheduleService: scheduleService, jwtCfg: jwtCfg, userStore: userStore}
}

func (ctrl *ScheduleController) getUserForMiddleware(userID int64) (*auth.User, error) {
	user, err := ctrl.userStore.GetUserByID(userID)
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

func (ctrl *ScheduleController) RegisterRoutes(h *server.Hertz) {
	v1 := h.Group("/api/v1")
	sched := v1.Group("/schedules")

	if ctrl.userStore == nil {
		sched.GET("", ctrl.ListSchedules)
		sched.POST("", ctrl.CreateSchedule)
		sched.GET("/:id", ctrl.GetSchedule)
		sched.PUT("/:id", ctrl.UpdateSchedule)
		sched.DELETE("/:id", ctrl.DeleteSchedule)
		sched.POST("/:id/trigger", ctrl.TriggerSchedule)
		sched.GET("/:id/executions", ctrl.ListExecutions)
		return
	}

	protected := sched.Group("", auth.JWTMiddleware(ctrl.jwtCfg, ctrl.getUserForMiddleware))
	protected.GET("", ctrl.ListSchedules)
	protected.POST("", ctrl.CreateSchedule)
	protected.GET("/:id", ctrl.GetSchedule)
	protected.PUT("/:id", ctrl.UpdateSchedule)
	protected.DELETE("/:id", ctrl.DeleteSchedule)
	protected.POST("/:id/trigger", ctrl.TriggerSchedule)
	protected.GET("/:id/executions", ctrl.ListExecutions)
}

func (ctrl *ScheduleController) ListSchedules(c context.Context, ctx *app.RequestContext) {
	schedules, err := ctrl.scheduleService.ListSchedules()
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(schedules))
}

func (ctrl *ScheduleController) GetSchedule(c context.Context, ctx *app.RequestContext) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse("invalid schedule id"))
		return
	}

	schedule, err := ctrl.scheduleService.GetSchedule(id)
	if err != nil {
		ctx.JSON(consts.StatusNotFound, schema.ErrorResponse("schedule not found"))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(schedule))
}

func (ctrl *ScheduleController) CreateSchedule(c context.Context, ctx *app.RequestContext) {
	var req schema.CreateScheduleRequest
	if err := ctx.BindAndValidate(&req); err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	ownerID := ""
	if u := auth.GetCurrentUser(ctx); u != nil {
		ownerID = strconv.FormatInt(u.ID, 10)
	}

	schedule, err := ctrl.scheduleService.CreateSchedule(&req, ownerID)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(schedule))
}

func (ctrl *ScheduleController) UpdateSchedule(c context.Context, ctx *app.RequestContext) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse("invalid schedule id"))
		return
	}

	var req schema.UpdateScheduleRequest
	if err := ctx.BindAndValidate(&req); err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	ownerID := ""
	if u := auth.GetCurrentUser(ctx); u != nil {
		ownerID = strconv.FormatInt(u.ID, 10)
	}

	schedule, err := ctrl.scheduleService.UpdateSchedule(id, &req, ownerID)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(schedule))
}

func (ctrl *ScheduleController) DeleteSchedule(c context.Context, ctx *app.RequestContext) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse("invalid schedule id"))
		return
	}

	if err := ctrl.scheduleService.DeleteSchedule(id); err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(nil))
}

func (ctrl *ScheduleController) TriggerSchedule(c context.Context, ctx *app.RequestContext) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse("invalid schedule id"))
		return
	}

	triggerUserID := ""
	if u := auth.GetCurrentUser(ctx); u != nil {
		triggerUserID = strconv.FormatInt(u.ID, 10)
	}

	if err := ctrl.scheduleService.TriggerSchedule(c, id, triggerUserID); err != nil {
		if errors.Is(err, scheduler.ErrScheduleDisabled) {
			ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
			return
		}
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(map[string]string{"status": "triggered"}))
}

func (ctrl *ScheduleController) ListExecutions(c context.Context, ctx *app.RequestContext) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse("invalid schedule id"))
		return
	}

	limit := parseIntQueryDefault(ctx, "limit", 50)

	executions, err := ctrl.scheduleService.ListExecutions(id, limit)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	ctx.JSON(consts.StatusOK, schema.SuccessResponse(executions))
}
