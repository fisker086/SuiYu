package controller

import (
	"context"
	"strconv"

	"github.com/fisk086/sya/internal/auth"
	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/service"
	"github.com/fisk086/sya/internal/storage"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type RBACController struct {
	rbacService *service.RBACService
	userStore   storage.UserStore
	jwtCfg      auth.JWTConfig
}

func NewRBACController(rbacService *service.RBACService, userStore storage.UserStore, jwtCfg auth.JWTConfig) *RBACController {
	return &RBACController{
		rbacService: rbacService,
		userStore:   userStore,
		jwtCfg:      jwtCfg,
	}
}

func (ctrl *RBACController) RegisterRoutes(r *server.Hertz) {
	rbacGroup := r.Group("/api/v1/rbac")
	rbacGroup.Use(auth.JWTMiddleware(ctrl.jwtCfg, ctrl.getUserForMiddleware))

	rbacGroup.GET("/roles", ctrl.ListRoles)
	rbacGroup.GET("/roles/:id", ctrl.GetRole)
	rbacGroup.POST("/roles", ctrl.CreateRole)
	rbacGroup.PUT("/roles/:id", ctrl.UpdateRole)
	rbacGroup.DELETE("/roles/:id", ctrl.DeleteRole)
	rbacGroup.GET("/roles/:id/agent-permissions", ctrl.GetRoleAgentPermissions)
	rbacGroup.POST("/roles/:id/agent-permissions", ctrl.SetRoleAgentPermissions)

	rbacGroup.GET("/users/:user_id/roles", ctrl.ListUserRoles)
	rbacGroup.POST("/users/:user_id/roles", ctrl.AssignRole)
	rbacGroup.DELETE("/users/:user_id/roles/:role_id", ctrl.RevokeRole)
}

func (ctrl *RBACController) getUserForMiddleware(userID int64) (*auth.User, error) {
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

func (ctrl *RBACController) ListRoles(ctx context.Context, c *app.RequestContext) {
	roles, err := ctrl.rbacService.ListRoles()
	if err != nil {
		c.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(consts.StatusOK, schema.SuccessResponse(roles))
}

func (ctrl *RBACController) GetRole(ctx context.Context, c *app.RequestContext) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(consts.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}
	role, err := ctrl.rbacService.GetRole(id)
	if err != nil {
		c.JSON(consts.StatusNotFound, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(consts.StatusOK, schema.SuccessResponse(role))
}

func (ctrl *RBACController) CreateRole(ctx context.Context, c *app.RequestContext) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		IsActive    bool   `json:"is_active"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	role, err := ctrl.rbacService.CreateRole(&model.Role{
		Name:        req.Name,
		Description: req.Description,
		IsActive:    req.IsActive,
	})
	if err != nil {
		c.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(consts.StatusOK, schema.SuccessResponse(role))
}

func (ctrl *RBACController) UpdateRole(ctx context.Context, c *app.RequestContext) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(consts.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		IsActive    bool   `json:"is_active"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	role, err := ctrl.rbacService.UpdateRole(id, &model.Role{
		Name:        req.Name,
		Description: req.Description,
		IsActive:    req.IsActive,
	})
	if err != nil {
		c.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(consts.StatusOK, schema.SuccessResponse(role))
}

func (ctrl *RBACController) DeleteRole(ctx context.Context, c *app.RequestContext) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(consts.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}
	if err := ctrl.rbacService.DeleteRole(id); err != nil {
		c.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(consts.StatusOK, schema.SuccessResponse(nil))
}

func (ctrl *RBACController) ListUserRoles(ctx context.Context, c *app.RequestContext) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil {
		c.JSON(consts.StatusBadRequest, schema.ErrorResponse("invalid user_id"))
		return
	}
	roles, err := ctrl.rbacService.ListUserRoles(userID)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(consts.StatusOK, schema.SuccessResponse(roles))
}

func (ctrl *RBACController) AssignRole(ctx context.Context, c *app.RequestContext) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil {
		c.JSON(consts.StatusBadRequest, schema.ErrorResponse("invalid user_id"))
		return
	}
	var req struct {
		RoleID int64 `json:"role_id"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	if err := ctrl.rbacService.AssignRole(userID, req.RoleID); err != nil {
		c.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(consts.StatusOK, schema.SuccessResponse(nil))
}

func (ctrl *RBACController) RevokeRole(ctx context.Context, c *app.RequestContext) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil {
		c.JSON(consts.StatusBadRequest, schema.ErrorResponse("invalid user_id"))
		return
	}
	roleID, err := strconv.ParseInt(c.Param("role_id"), 10, 64)
	if err != nil {
		c.JSON(consts.StatusBadRequest, schema.ErrorResponse("invalid role_id"))
		return
	}
	if err := ctrl.rbacService.RevokeRole(userID, roleID); err != nil {
		c.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(consts.StatusOK, schema.SuccessResponse(nil))
}

func (ctrl *RBACController) GetRoleAgentPermissions(ctx context.Context, c *app.RequestContext) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(consts.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}
	perms, err := ctrl.rbacService.GetRoleAgentPermissions(ctx, id)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(consts.StatusOK, schema.SuccessResponse(perms))
}

func (ctrl *RBACController) SetRoleAgentPermissions(ctx context.Context, c *app.RequestContext) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(consts.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}
	var req struct {
		AgentIDs []int64 `json:"agent_ids"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	if err := ctrl.rbacService.SetRoleAgentPermissions(ctx, id, req.AgentIDs); err != nil {
		c.JSON(consts.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(consts.StatusOK, schema.SuccessResponse(nil))
}
