package controller

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/service"
	"github.com/fisk086/sya/internal/storage"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

type WorkflowController struct {
	svc *service.WorkflowService
}

func NewWorkflowController(svc *service.WorkflowService) *WorkflowController {
	return &WorkflowController{svc: svc}
}

func (c *WorkflowController) RegisterRoutes(r *server.Hertz) {
	g := r.Group("/api/v1/workflows")
	g.GET("", c.List)
	g.GET("/:id", c.Get)
	g.POST("", c.Create)
	g.PUT("/:id", c.Update)
	g.DELETE("/:id", c.Delete)
}

// @Summary List agent workflows
// @Description Configurable multi-agent orchestration templates (for UI and POST /chat with workflow_id).
// @Tags workflows
// @Produce json
// @Success 200 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /workflows [get]
func (c *WorkflowController) List(ctx context.Context, hc *app.RequestContext) {
	list, err := c.svc.List(ctx)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(list))
}

// @Summary Get workflow by ID
// @Tags workflows
// @Produce json
// @Param id path int true "Workflow ID"
// @Success 200 {object} schema.APIResponse
// @Failure 404 {object} schema.APIResponse
// @Router /workflows/{id} [get]
func (c *WorkflowController) Get(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id < 1 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid workflow id"))
		return
	}
	wf, err := c.svc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrWorkflowNotFound) {
			hc.JSON(http.StatusNotFound, schema.ErrorResponse("workflow not found"))
			return
		}
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(wf))
}

// @Summary Create workflow
// @Description kind: single | sequential | parallel | supervisor | loop (runtime executes sequential today; others reserved for Eino ADK).
// @Tags workflows
// @Accept json
// @Produce json
// @Param request body schema.CreateWorkflowRequest true "Workflow"
// @Success 201 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Router /workflows [post]
func (c *WorkflowController) Create(ctx context.Context, hc *app.RequestContext) {
	var req schema.CreateWorkflowRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	wf, err := c.svc.Create(ctx, &req)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			hc.JSON(http.StatusConflict, schema.ErrorResponse(err.Error()))
			return
		}
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusCreated, schema.SuccessResponse(wf))
}

// @Summary Update workflow
// @Tags workflows
// @Accept json
// @Produce json
// @Param id path int true "Workflow ID"
// @Param request body schema.UpdateWorkflowRequest true "Fields"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Router /workflows/{id} [put]
func (c *WorkflowController) Update(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id < 1 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid workflow id"))
		return
	}
	var req schema.UpdateWorkflowRequest
	if err := hc.BindJSON(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	wf, err := c.svc.Update(ctx, id, &req)
	if err != nil {
		if errors.Is(err, storage.ErrWorkflowNotFound) {
			hc.JSON(http.StatusNotFound, schema.ErrorResponse("workflow not found"))
			return
		}
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(wf))
}

// @Summary Delete workflow
// @Tags workflows
// @Param id path int true "Workflow ID"
// @Success 200 {object} schema.APIResponse
// @Failure 404 {object} schema.APIResponse
// @Router /workflows/{id} [delete]
func (c *WorkflowController) Delete(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id < 1 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid workflow id"))
		return
	}
	if err := c.svc.Delete(ctx, id); err != nil {
		if errors.Is(err, storage.ErrWorkflowNotFound) {
			hc.JSON(http.StatusNotFound, schema.ErrorResponse("workflow not found"))
			return
		}
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]string{"status": "deleted"}))
}
