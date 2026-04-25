package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/fisk086/sya/internal/model"
)

func (s *PostgresStorage) buildApprovalRequestWhere(filter *ApprovalRequestFilter) string {
	where := "1=1"
	if filter.AgentID != 0 {
		where += fmt.Sprintf(" AND agent_id = %d", filter.AgentID)
	}
	if filter.SessionID != "" {
		where += fmt.Sprintf(" AND session_id = '%s'", filter.SessionID)
	}
	if filter.Status != "" {
		where += fmt.Sprintf(" AND status = '%s'", filter.Status)
	}
	if filter.ExternalID != "" {
		where += fmt.Sprintf(" AND external_id = '%s'", filter.ExternalID)
	}
	if filter.UserID != "" {
		where += fmt.Sprintf(" AND user_id = '%s'", filter.UserID)
	}
	return where
}

func (s *PostgresStorage) CreateApprovalRequest(req *model.ApprovalRequest) (*model.ApprovalRequest, error) {
	createdAt := req.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	var id int64
	err := s.pool.QueryRow(context.Background(),
		`INSERT INTO approval_requests (agent_id, session_id, user_id, tool_name, risk_level, input, status, approver_id, comment, created_at, approval_type, external_id, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		 RETURNING id`,
		req.AgentID, req.SessionID, req.UserID, req.ToolName, req.RiskLevel, req.Input,
		req.Status, req.ApproverID, req.Comment, createdAt, req.ApprovalType, req.ExternalID, req.ExpiresAt,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	req.ID = id
	req.CreatedAt = createdAt
	return req, nil
}

func (s *PostgresStorage) ListApprovalRequests(filter *ApprovalRequestFilter) ([]*model.ApprovalRequest, int64, error) {
	where := s.buildApprovalRequestWhere(filter)

	var total int64
	err := s.pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM approval_requests WHERE "+where,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	page := 1
	pageSize := 50
	if filter != nil {
		if filter.Page > 0 {
			page = filter.Page
		}
		if filter.PageSize > 0 {
			pageSize = filter.PageSize
		}
	}
	offset := (page - 1) * pageSize

	rows, err := s.pool.Query(context.Background(),
		fmt.Sprintf(`SELECT id, agent_id, session_id, user_id, tool_name, risk_level, input, status, approver_id, comment, approved_at, created_at, approval_type, external_id, expires_at
			FROM approval_requests WHERE %s ORDER BY created_at DESC LIMIT %d OFFSET %d`, where, pageSize, offset),
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var requests []*model.ApprovalRequest
	for rows.Next() {
		var req model.ApprovalRequest
		var approvedAt *time.Time
		var expiresAt *time.Time
		var externalID sql.NullString
		err := rows.Scan(
			&req.ID, &req.AgentID, &req.SessionID, &req.UserID, &req.ToolName, &req.RiskLevel,
			&req.Input, &req.Status, &req.ApproverID, &req.Comment, &approvedAt, &req.CreatedAt,
			&req.ApprovalType, &externalID, &expiresAt,
		)
		if err != nil {
			return nil, 0, err
		}
		if externalID.Valid {
			req.ExternalID = externalID.String
		}
		req.ApprovedAt = approvedAt
		req.ExpiresAt = expiresAt
		requests = append(requests, &req)
	}
	return requests, total, nil
}

func (s *PostgresStorage) GetApprovalRequest(id int64) (*model.ApprovalRequest, error) {
	var req model.ApprovalRequest
	var approvedAt *time.Time
	var expiresAt *time.Time
	var externalID sql.NullString
	err := s.pool.QueryRow(context.Background(),
		`SELECT id, agent_id, session_id, user_id, tool_name, risk_level, input, status, approver_id, comment, approved_at, created_at, approval_type, external_id, expires_at
		 FROM approval_requests WHERE id = $1`,
		id,
	).Scan(
		&req.ID, &req.AgentID, &req.SessionID, &req.UserID, &req.ToolName, &req.RiskLevel,
		&req.Input, &req.Status, &req.ApproverID, &req.Comment, &approvedAt, &req.CreatedAt,
		&req.ApprovalType, &externalID, &expiresAt,
	)
	if err != nil {
		return nil, err
	}
	if externalID.Valid {
		req.ExternalID = externalID.String
	}
	req.ApprovedAt = approvedAt
	req.ExpiresAt = expiresAt
	return &req, nil
}

func (s *PostgresStorage) UpdateApprovalRequest(id int64, status, approverID, comment string) error {
	_, err := s.pool.Exec(context.Background(),
		`UPDATE approval_requests SET status = $1, approver_id = $2, comment = $3, approved_at = NOW() WHERE id = $4`,
		status, approverID, comment, id,
	)
	return err
}
