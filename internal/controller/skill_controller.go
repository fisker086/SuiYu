package controller

import (
	"context"
	"net/http"

	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/service"
	"github.com/fisk086/sya/internal/skills"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

type SkillController struct {
	skillService *service.SkillService
	registry     *skills.Registry
	skillsDir    string
}

func NewSkillController(skillService *service.SkillService, registry *skills.Registry, skillsDir string) *SkillController {
	return &SkillController{skillService: skillService, registry: registry, skillsDir: skillsDir}
}

func (c *SkillController) RegisterRoutes(r *server.Hertz) {
	skills := r.Group("/api/v1/skills")
	skills.GET("", c.ListSkills)
	skills.POST("/sync-builtins", c.SyncBuiltinSkills)
	skills.POST("", c.CreateSkill)
	skills.GET("/:id", c.GetSkillByID)
	skills.PUT("/:id", c.UpdateSkill)
	skills.DELETE("/:id", c.DeleteSkill)
}

// @Summary List all skills
// @Description Get a list of all available skills
// @Tags skills
// @Accept json
// @Produce json
// @Success 200 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /skills [get]
func (c *SkillController) ListSkills(ctx context.Context, hc *app.RequestContext) {
	skills, err := c.skillService.ListSkills()
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(skills))
}

// @Summary Get skill by ID
// @Description Returns one skill including content; empty DB content may be filled from skills/*/SKILL.md for built-ins.
// @Tags skills
// @Produce json
// @Param id path int true "Skill ID"
// @Success 200 {object} schema.APIResponse
// @Router /skills/{id} [get]
func (c *SkillController) GetSkillByID(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid skill id"))
		return
	}
	skill, err := c.skillService.GetSkill(id, c.skillsDir)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	if skill == nil {
		hc.JSON(http.StatusNotFound, schema.ErrorResponse("skill not found"))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(skill))
}

// @Summary Create a new skill
// @Description Create a new skill with the specified configuration
// @Tags skills
// @Accept json
// @Produce json
// @Param skill body schema.CreateSkillRequest true "Skill data"
// @Success 201 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /skills [post]
func (c *SkillController) CreateSkill(ctx context.Context, hc *app.RequestContext) {
	var req schema.CreateSkillRequest
	if err := hc.BindAndValidate(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}

	skill, err := c.skillService.CreateSkill(&req)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	hc.JSON(http.StatusCreated, schema.SuccessResponse(skill))
}

// @Summary Sync built-in skills from skills directory into the database
// @Description Inserts missing skills from the server registry (skills/*/SKILL.md). Does not overwrite existing rows.
// @Tags skills
// @Produce json
// @Success 200 {object} schema.APIResponse
// @Router /skills/sync-builtins [post]
func (c *SkillController) SyncBuiltinSkills(ctx context.Context, hc *app.RequestContext) {
	if c.registry == nil {
		hc.JSON(http.StatusServiceUnavailable, schema.ErrorResponse("builtin skill registry not available"))
		return
	}
	n := c.skillService.SyncBuiltinSkills(c.registry, c.skillsDir)
	hc.JSON(http.StatusOK, schema.SuccessResponse(map[string]int{"created": n}))
}

// @Summary Update a skill
// @Description Partial update: only send fields to change (risk_level, execution_mode, prompt_hint, etc.)
// @Tags skills
// @Accept json
// @Produce json
// @Param id path int true "Skill ID"
// @Param body body schema.UpdateSkillRequest true "Fields to update"
// @Success 200 {object} schema.APIResponse
// @Router /skills/{id} [put]
func (c *SkillController) UpdateSkill(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid skill id"))
		return
	}
	var req schema.UpdateSkillRequest
	if err := hc.BindJSON(&req); err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	skill, err := c.skillService.UpdateSkill(id, &req)
	if err != nil {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	hc.JSON(http.StatusOK, schema.SuccessResponse(skill))
}

// @Summary Delete a skill
// @Description Delete a skill by its ID
// @Tags skills
// @Accept json
// @Produce json
// @Param id path int true "Skill ID"
// @Success 200 {object} schema.APIResponse
// @Failure 400 {object} schema.APIResponse
// @Failure 500 {object} schema.APIResponse
// @Router /skills/{id} [delete]
func (c *SkillController) DeleteSkill(ctx context.Context, hc *app.RequestContext) {
	id := parseInt64Param(hc, "id")
	if id == 0 {
		hc.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid skill id"))
		return
	}

	if err := c.skillService.DeleteSkill(id); err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(nil))
}
