package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/fisk086/sya/internal/model"
	"github.com/jackc/pgx/v5"
)

func (s *PostgresStorage) ListRoles() ([]model.Role, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT r.id, r.name, r.description, r.is_system, r.is_active, r.created_at, r.updated_at,
		       COALESCE(cnt.user_count, 0) as user_count,
		       COALESCE(ac.agent_count, 0) as agent_count
		FROM rbac_roles r
		LEFT JOIN (SELECT role_id, COUNT(*) as user_count FROM user_roles WHERE is_active = TRUE GROUP BY role_id) cnt ON r.id = cnt.role_id
		LEFT JOIN (SELECT role_id, COUNT(*) as agent_count FROM rbac_role_agent_permissions GROUP BY role_id) ac ON r.id = ac.role_id
		ORDER BY r.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []model.Role
	for rows.Next() {
		var r model.Role
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.IsSystem, &r.IsActive, &r.CreatedAt, &r.UpdatedAt, &r.UserCount, &r.AgentCount); err != nil {
			return nil, err
		}
		roles = append(roles, r)
	}
	return roles, nil
}

func (s *PostgresStorage) GetRole(id int64) (*model.Role, error) {
	var r model.Role
	err := s.pool.QueryRow(context.Background(),
		`SELECT id, name, description, is_system, is_active, created_at, updated_at FROM rbac_roles WHERE id = $1`, id).
		Scan(&r.ID, &r.Name, &r.Description, &r.IsSystem, &r.IsActive, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("role not found")
		}
		return nil, err
	}
	return &r, nil
}

func (s *PostgresStorage) CreateRole(role *model.Role) (*model.Role, error) {
	var id int64
	err := s.pool.QueryRow(context.Background(),
		`INSERT INTO rbac_roles (name, description, is_system, is_active) VALUES ($1, $2, $3, $4) RETURNING id`,
		role.Name, role.Description, role.IsSystem, role.IsActive).Scan(&id)
	if err != nil {
		return nil, err
	}
	role.ID = id
	return role, nil
}

func (s *PostgresStorage) UpdateRole(id int64, role *model.Role) (*model.Role, error) {
	_, err := s.pool.Exec(context.Background(),
		`UPDATE rbac_roles SET name = $1, description = $2, is_active = $3, updated_at = NOW() WHERE id = $4`,
		role.Name, role.Description, role.IsActive, id)
	if err != nil {
		return nil, err
	}
	return s.GetRole(id)
}

func (s *PostgresStorage) DeleteRole(id int64) error {
	tag, err := s.pool.Exec(context.Background(), `DELETE FROM rbac_roles WHERE id = $1 AND is_system = FALSE`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("role not found or is system role")
	}
	return nil
}

func (s *PostgresStorage) GetRoleAgentPermissions(ctx context.Context, roleID int64) (map[int64]bool, error) {
	rows, err := s.pool.Query(ctx, `SELECT agent_id FROM rbac_role_agent_permissions WHERE role_id = $1`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	perms := make(map[int64]bool)
	for rows.Next() {
		var agentID int64
		if err := rows.Scan(&agentID); err != nil {
			return nil, err
		}
		perms[agentID] = true
	}
	return perms, nil
}

func (s *PostgresStorage) SetRoleAgentPermissions(ctx context.Context, roleID int64, agentIDs []int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM rbac_role_agent_permissions WHERE role_id = $1`, roleID)
	if err != nil {
		return err
	}

	for _, aid := range agentIDs {
		_, err = tx.Exec(ctx, `INSERT INTO rbac_role_agent_permissions (role_id, agent_id) VALUES ($1, $2)`, roleID, aid)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *PostgresStorage) GetUserAgentPermissions(ctx context.Context, userID int64) (map[int64]bool, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT rap.agent_id 
		 FROM rbac_role_agent_permissions rap
		 JOIN user_roles ur ON ur.role_id = rap.role_id
		 WHERE ur.user_id = $1 AND ur.is_active = TRUE AND (ur.expires_at IS NULL OR ur.expires_at > NOW())`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]bool)
	for rows.Next() {
		var agentID int64
		if err := rows.Scan(&agentID); err != nil {
			return nil, err
		}
		result[agentID] = true
	}
	return result, nil
}

func (s *PostgresStorage) ListUserRoles(userID int64) ([]model.UserRole, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT ur.id, ur.user_id, ur.role_id, ur.is_active, ur.expires_at, ur.created_at, r.name, r.description 
		 FROM user_roles ur JOIN rbac_roles r ON ur.role_id = r.id 
		 WHERE ur.user_id = $1 AND (ur.expires_at IS NULL OR ur.expires_at > NOW())`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var urs []model.UserRole
	for rows.Next() {
		var ur model.UserRole
		var roleName, roleDesc string
		if err := rows.Scan(&ur.ID, &ur.UserID, &ur.RoleID, &ur.IsActive, &ur.ExpiresAt, &ur.CreatedAt, &roleName, &roleDesc); err != nil {
			return nil, err
		}
		ur.Role = &model.Role{ID: ur.RoleID, Name: roleName, Description: roleDesc}
		urs = append(urs, ur)
	}
	return urs, nil
}

func (s *PostgresStorage) GetUserRolesByRoleID(ctx context.Context, roleID int64) ([]model.UserRole, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, role_id, is_active, expires_at, created_at FROM user_roles WHERE role_id = $1 AND is_active = TRUE AND (expires_at IS NULL OR expires_at > NOW())`,
		roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var urs []model.UserRole
	for rows.Next() {
		var ur model.UserRole
		if err := rows.Scan(&ur.ID, &ur.UserID, &ur.RoleID, &ur.IsActive, &ur.ExpiresAt, &ur.CreatedAt); err != nil {
			return nil, err
		}
		urs = append(urs, ur)
	}
	return urs, nil
}

func (s *PostgresStorage) AssignRole(userID int64, roleID int64) error {
	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, roleID)
	return err
}

func (s *PostgresStorage) RevokeRole(userID int64, roleID int64) error {
	tag, err := s.pool.Exec(context.Background(), `DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2`, userID, roleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user role not found")
	}
	return nil
}

func (s *PostgresStorage) GetUserPermissions(ctx context.Context, userID int64) ([]model.Permission, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT p.id, p.resource_type, p.resource_name, p.actions, p.description, p.is_system, p.created_at 
		 FROM rbac_permissions p 
		 JOIN rbac_role_permissions rp ON p.id = rp.permission_id 
		 JOIN user_roles ur ON ur.role_id = rp.role_id 
		 WHERE ur.user_id = $1 AND ur.is_active = TRUE AND (ur.expires_at IS NULL OR ur.expires_at > NOW())`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []model.Permission
	for rows.Next() {
		var p model.Permission
		if err := rows.Scan(&p.ID, &p.ResourceType, &p.ResourceName, &p.Actions, &p.Description, &p.IsSystem, &p.CreatedAt); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, nil
}
