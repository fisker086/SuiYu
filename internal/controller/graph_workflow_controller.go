package controller

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/fisk086/sya/internal/auth"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/service"
)

type GraphWorkflowController struct {
	graphService *service.GraphWorkflowService
}

func NewGraphWorkflowController(graphService *service.GraphWorkflowService) *GraphWorkflowController {
	return &GraphWorkflowController{graphService: graphService}
}

func (ctrl *GraphWorkflowController) RegisterRoutes(r *server.Hertz) {
	v1 := r.Group("/api/v1")

	v1.GET("/workflows/graph", ctrl.ListDefinitions)
	v1.GET("/workflows/graph/:id", ctrl.GetDefinition)
	v1.GET("/workflows/graph/key/:key", ctrl.GetDefinitionByKey)
	v1.POST("/workflows/graph", ctrl.CreateDefinition)
	v1.PUT("/workflows/graph/:id", ctrl.UpdateDefinition)
	v1.DELETE("/workflows/graph/:id", ctrl.DeleteDefinition)
	v1.POST("/workflows/graph/:id/execute", ctrl.Execute)
	v1.GET("/workflows/graph/:id/executions", ctrl.ListExecutions)
	v1.GET("/workflows/graph/executions/:execId", ctrl.GetExecution)
}

func (ctrl *GraphWorkflowController) ListDefinitions(c context.Context, ctx *app.RequestContext) {
	defs, err := ctrl.graphService.ListDefinitions(c)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	ctx.JSON(consts.StatusOK, schema.SuccessResponse(defs))
}

func (ctrl *GraphWorkflowController) GetDefinition(c context.Context, ctx *app.RequestContext) {
	id := parseInt64Param(ctx, "id")

	def, err := ctrl.graphService.GetDefinition(c, id)
	if err != nil {
		ctx.JSON(consts.StatusNotFound, schema.ErrorResponse("workflow not found"))
		return
	}
	ctx.JSON(consts.StatusOK, schema.SuccessResponse(def))
}

func (ctrl *GraphWorkflowController) GetDefinitionByKey(c context.Context, ctx *app.RequestContext) {
	key := ctx.Param("key")

	def, err := ctrl.graphService.GetDefinitionByKey(c, key)
	if err != nil {
		ctx.JSON(consts.StatusNotFound, schema.ErrorResponse("workflow not found"))
		return
	}
	ctx.JSON(consts.StatusOK, schema.SuccessResponse(def))
}

func (ctrl *GraphWorkflowController) CreateDefinition(c context.Context, ctx *app.RequestContext) {
	var req schema.CreateWorkflowDefinitionRequest
	if err := ctx.BindAndValidate(&req); err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	def, err := ctrl.graphService.CreateDefinition(c, &req)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	ctx.JSON(consts.StatusOK, schema.SuccessResponse(def))
}

func (ctrl *GraphWorkflowController) UpdateDefinition(c context.Context, ctx *app.RequestContext) {
	id := parseInt64Param(ctx, "id")

	var req schema.UpdateWorkflowDefinitionRequest
	if err := ctx.BindAndValidate(&req); err != nil {
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	def, err := ctrl.graphService.UpdateDefinition(c, id, &req)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	ctx.JSON(consts.StatusOK, schema.SuccessResponse(def))
}

func (ctrl *GraphWorkflowController) DeleteDefinition(c context.Context, ctx *app.RequestContext) {
	id := parseInt64Param(ctx, "id")

	if err := ctrl.graphService.DeleteDefinition(c, id); err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	ctx.JSON(consts.StatusOK, schema.SuccessResponse(nil))
}

func (ctrl *GraphWorkflowController) Execute(c context.Context, ctx *app.RequestContext) {
	id := parseInt64Param(ctx, "id")
	slog.Info("executing workflow", "workflowID", id)

	var req schema.ExecuteWorkflowRequest
	if err := ctx.BindAndValidate(&req); err != nil {
		slog.Error("failed to bind request", "workflowID", id, "error", err)
		ctx.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	slog.Debug("execute request", "workflowID", id, "message", req.Message, "variables", req.Variables)

	req.WorkflowID = id

	user := auth.GetCurrentUser(ctx)
	if user != nil {
		req.UserID = strconv.FormatInt(user.ID, 10)
	}

	result, err := ctrl.graphService.Execute(c, &req)
	if err != nil {
		slog.Error("workflow execution failed", "workflowID", id, "error", err)
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	slog.Info("workflow executed successfully", "workflowID", id, "durationMS", result.DurationMS)
	ctx.JSON(consts.StatusOK, schema.SuccessResponse(result))
}

func (ctrl *GraphWorkflowController) ListExecutions(c context.Context, ctx *app.RequestContext) {
	id := parseInt64Param(ctx, "id")

	execs, err := ctrl.graphService.ListExecutions(c, id, 50)
	if err != nil {
		ctx.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	ctx.JSON(consts.StatusOK, schema.SuccessResponse(execs))
}

func (ctrl *GraphWorkflowController) GetExecution(c context.Context, ctx *app.RequestContext) {
	execID := parseInt64Param(ctx, "execId")

	exec, err := ctrl.graphService.GetExecution(c, execID)
	if err != nil {
		ctx.JSON(consts.StatusNotFound, schema.ErrorResponse("execution not found"))
		return
	}
	ctx.JSON(consts.StatusOK, schema.SuccessResponse(exec))
}
