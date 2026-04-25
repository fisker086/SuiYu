package service

import (
	"fmt"
	"time"

	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/storage"
)

type ApprovalService struct {
	store  ApprovalStore
	agents AgentInfoProvider
}

// AgentInfoProvider loads agent + runtime (approvers) for enrichment and permission checks.
type AgentInfoProvider interface {
	GetAgent(id int64) (*schema.AgentWithRuntime, error)
}

type ApprovalStore interface {
	CreateApprovalRequest(req *model.ApprovalRequest) (*model.ApprovalRequest, error)
	ListApprovalRequests(filter *storage.ApprovalRequestFilter) ([]*model.ApprovalRequest, int64, error)
	GetApprovalRequest(id int64) (*model.ApprovalRequest, error)
	UpdateApprovalRequest(id int64, status, approverID, comment string) error
}

// ApprovalViewer is used to compute can_approve on list/detail responses. Pass nil to skip.
type ApprovalViewer struct {
	Username        string
	HasApproverRole bool
}

func NewApprovalService(store ApprovalStore, agents AgentInfoProvider) *ApprovalService {
	return &ApprovalService{store: store, agents: agents}
}

func (s *ApprovalService) CreateRequest(req *model.ApprovalRequest) (*schema.ApprovalRequest, error) {
	r, err := s.store.CreateApprovalRequest(req)
	if err != nil {
		return nil, err
	}
	return toSchemaApproval(r), nil
}

func (s *ApprovalService) ListRequests(filter *storage.ApprovalRequestFilter, viewer *ApprovalViewer) ([]*schema.ApprovalRequest, int64, error) {
	reqs, total, err := s.store.ListApprovalRequests(filter)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*schema.ApprovalRequest, 0, len(reqs))
	for _, r := range reqs {
		if r.Status == "pending" && r.ExpiresAt != nil && r.ExpiresAt.Before(time.Now()) {
			r.Status = "expired"
		}
		sch := toSchemaApproval(r)
		s.enrichOne(sch, viewer)
		result = append(result, sch)
	}
	return result, total, nil
}

func (s *ApprovalService) GetRequest(id int64, viewer *ApprovalViewer) (*schema.ApprovalRequest, error) {
	r, err := s.store.GetApprovalRequest(id)
	if err != nil {
		return nil, err
	}
	if r.Status == "pending" && r.ExpiresAt != nil && r.ExpiresAt.Before(time.Now()) {
		r.Status = "expired"
	}
	sch := toSchemaApproval(r)
	s.enrichOne(sch, viewer)
	return sch, nil
}

func (s *ApprovalService) Approve(id int64, approverID, comment string) error {
	return s.store.UpdateApprovalRequest(id, "approved", approverID, comment)
}

func (s *ApprovalService) Reject(id int64, approverID, comment string) error {
	return s.store.UpdateApprovalRequest(id, "rejected", approverID, comment)
}

func (s *ApprovalService) enrichOne(req *schema.ApprovalRequest, viewer *ApprovalViewer) {
	if s.agents == nil {
		return
	}
	ag, err := s.agents.GetAgent(req.AgentID)
	if err != nil || ag == nil {
		return
	}
	if req.AgentName == "" {
		req.AgentName = ag.Agent.Name
	}
	if ag.RuntimeProfile != nil {
		req.DesignatedApprovers = append([]string(nil), ag.RuntimeProfile.Approvers...)
	}
	if viewer != nil {
		req.CanApprove = computeCanApprove(req, viewer.Username, viewer.HasApproverRole, ag)
	}
}

// computeCanApprove: designated approvers on the agent gate approval when non-empty; otherwise any user with
// approver role. Requesters cannot approve their own pending requests.
func computeCanApprove(req *schema.ApprovalRequest, username string, hasApproverRole bool, ag *schema.AgentWithRuntime) bool {
	if req.Status != "pending" {
		return false
	}
	if username == "" {
		return false
	}
	if req.UserID != "" && username == req.UserID {
		return false
	}
	var designated []string
	if ag != nil && ag.RuntimeProfile != nil {
		designated = ag.RuntimeProfile.Approvers
	}
	if len(designated) > 0 {
		for _, u := range designated {
			if u == username {
				return true
			}
		}
		return false
	}
	return hasApproverRole
}

// CanUserApproveRequest enforces the same rules as computeCanApprove for HTTP approve/reject.
func (s *ApprovalService) CanUserApproveRequest(approvalID int64, username string, hasApproverRole bool) (bool, error) {
	r, err := s.store.GetApprovalRequest(approvalID)
	if err != nil {
		return false, err
	}
	sch := toSchemaApproval(r)
	if sch.Status == "pending" && sch.ExpiresAt != nil && sch.ExpiresAt.Before(time.Now()) {
		sch.Status = "expired"
	}
	if s.agents == nil {
		return hasApproverRole && username != "" && sch.UserID != username && sch.Status == "pending", nil
	}
	ag, err := s.agents.GetAgent(sch.AgentID)
	if err != nil || ag == nil {
		return hasApproverRole && username != "" && sch.UserID != username && sch.Status == "pending", nil
	}
	return computeCanApprove(sch, username, hasApproverRole, ag), nil
}

// ErrNotApprover may be returned by ValidateApprovePermission for a stable API message.
var ErrNotApprover = fmt.Errorf("not authorized to approve or reject this request")

func (s *ApprovalService) ValidateApprovePermission(approvalID int64, username string, hasApproverRole bool) error {
	ok, err := s.CanUserApproveRequest(approvalID, username, hasApproverRole)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotApprover
	}
	return nil
}

func toSchemaApproval(m *model.ApprovalRequest) *schema.ApprovalRequest {
	return &schema.ApprovalRequest{
		ID:           m.ID,
		AgentID:      m.AgentID,
		AgentName:    m.AgentName,
		SessionID:    m.SessionID,
		UserID:       m.UserID,
		ToolName:     m.ToolName,
		RiskLevel:    m.RiskLevel,
		Input:        m.Input,
		Status:       m.Status,
		ApproverID:   m.ApproverID,
		Comment:      m.Comment,
		ApprovedAt:   m.ApprovedAt,
		CreatedAt:    m.CreatedAt,
		ApprovalType: m.ApprovalType,
		ExternalID:   m.ExternalID,
		ExpiresAt:    m.ExpiresAt,
	}
}
