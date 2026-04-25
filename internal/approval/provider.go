package approval

import (
	"context"
	"time"

	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/schema"
)

type ExternalApprovalProvider interface {
	SubmitApproval(ctx context.Context, req *ExternalApprovalRequest) (string, error)
	GetApprovalStatus(ctx context.Context, externalID string) (string, error)
	CancelApproval(ctx context.Context, externalID string) error
}

type ExternalApprovalRequest struct {
	AgentID       int64
	SessionID     string
	UserID        string
	ToolName      string
	RiskLevel     string
	Input         string
	Approvers     []string
	Title         string
	Timeout       time.Duration
	NotifyWebhook string
}

type ApprovalCallback struct {
	ExternalID string
	Status     string
	ApproverID string
	Comment    string
	ApprovedAt time.Time
}

type LarkProvider struct{}

func (p *LarkProvider) SubmitApproval(ctx context.Context, req *ExternalApprovalRequest) (string, error) {
	return "", nil
}

func (p *LarkProvider) GetApprovalStatus(ctx context.Context, externalID string) (string, error) {
	return "", nil
}

func (p *LarkProvider) CancelApproval(ctx context.Context, externalID string) error {
	return nil
}

type DingTalkProvider struct{}

func (p *DingTalkProvider) SubmitApproval(ctx context.Context, req *ExternalApprovalRequest) (string, error) {
	return "", nil
}

func (p *DingTalkProvider) GetApprovalStatus(ctx context.Context, externalID string) (string, error) {
	return "", nil
}

func (p *DingTalkProvider) CancelApproval(ctx context.Context, externalID string) error {
	return nil
}

type ProviderFactory struct{}

func (f *ProviderFactory) GetProvider(approvalType string) ExternalApprovalProvider {
	switch approvalType {
	case string(schema.ApprovalTypeLark):
		return &LarkProvider{}
	case string(schema.ApprovalTypeDingTalk):
		return &DingTalkProvider{}
	default:
		return nil
	}
}

type ApprovalNotifier interface {
	NotifyApprovalRequest(ctx context.Context, req *model.ApprovalRequest, approvers []string) error
	NotifyApprovalResult(ctx context.Context, req *model.ApprovalRequest) error
}
