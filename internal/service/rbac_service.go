package service

import (
	"context"

	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/storage"
)

type RBACService struct {
	store storage.Storage
}

func NewRBACService(store storage.Storage) *RBACService {
	return &RBACService{store: store}
}

// CheckAgentAccess returns true if the user is a platform admin, or has access to this agent via role.
func (s *RBACService) CheckAgentAccess(ctx context.Context, userID int64, agentName string, isAdmin bool) bool {
	if isAdmin {
		return true
	}

	agentID, err := s.store.GetAgentIDByName(ctx, agentName)
	if err != nil {
		return false
	}

	perms, err := s.store.GetUserAgentPermissions(ctx, userID)
	if err != nil {
		logger.Error("get user agent permissions failed", "user_id", userID, "err", err)
		return false
	}

	return perms[agentID]
}

func (s *RBACService) ListRoles() ([]model.Role, error) {
	return s.store.ListRoles()
}

func (s *RBACService) GetRole(id int64) (*model.Role, error) {
	return s.store.GetRole(id)
}

func (s *RBACService) CreateRole(role *model.Role) (*model.Role, error) {
	return s.store.CreateRole(role)
}

func (s *RBACService) UpdateRole(id int64, role *model.Role) (*model.Role, error) {
	return s.store.UpdateRole(id, role)
}

func (s *RBACService) DeleteRole(id int64) error {
	return s.store.DeleteRole(id)
}

func (s *RBACService) ListUserRoles(userID int64) ([]model.UserRole, error) {
	return s.store.ListUserRoles(userID)
}

func (s *RBACService) AssignRole(userID int64, roleID int64) error {
	return s.store.AssignRole(userID, roleID)
}

func (s *RBACService) RevokeRole(userID int64, roleID int64) error {
	return s.store.RevokeRole(userID, roleID)
}

func (s *RBACService) GetUserRolesByRoleID(ctx context.Context, roleID int64) ([]model.UserRole, error) {
	return s.store.GetUserRolesByRoleID(ctx, roleID)
}

func (s *RBACService) GetRoleAgentPermissions(ctx context.Context, roleID int64) (map[int64]bool, error) {
	return s.store.GetRoleAgentPermissions(ctx, roleID)
}

func (s *RBACService) SetRoleAgentPermissions(ctx context.Context, roleID int64, agentIDs []int64) error {
	return s.store.SetRoleAgentPermissions(ctx, roleID, agentIDs)
}
