package service

import (
	"context"

	"github.com/fisk086/sya/internal/gateway"
	"github.com/fisk086/sya/internal/schema"
)

type MessageService struct {
	router *gateway.MessageRouter
}

func NewMessageService(router *gateway.MessageRouter) *MessageService {
	return &MessageService{router: router}
}

func (s *MessageService) SendMessage(ctx context.Context, req *schema.SendMessageRequest) (*schema.MessageSendResponse, error) {
	return s.router.SendMessage(ctx, req)
}

func (s *MessageService) SendSpan(ctx context.Context, req *schema.MessageSpanRequest) (*schema.MessageSpanResponse, error) {
	return s.router.SendSpan(ctx, req)
}

func (s *MessageService) ListMessages(req *schema.ListMessagesRequest) ([]*schema.AgentMessage, int64, error) {
	return s.router.ListMessages(req)
}

func (s *MessageService) CreateChannel(req *schema.CreateMessageChannelRequest) (*schema.MessageChannel, error) {
	return s.router.CreateChannel(req)
}

func (s *MessageService) ListChannels(agentID int64) ([]*schema.MessageChannel, error) {
	return s.router.ListChannels(agentID)
}

func (s *MessageService) UpdateChannel(id int64, req *schema.UpdateMessageChannelRequest) (*schema.MessageChannel, error) {
	return s.router.UpdateChannel(id, req)
}

func (s *MessageService) DeleteChannel(id int64) error {
	return s.router.DeleteChannel(id)
}

func (s *MessageService) CreateA2ACard(req *schema.CreateA2ACardRequest) (*schema.A2ACard, error) {
	return s.router.CreateA2ACard(req)
}

func (s *MessageService) ListA2ACards(agentID int64) ([]*schema.A2ACard, error) {
	return s.router.ListA2ACards(agentID)
}

func (s *MessageService) GetA2ACard(id int64) (*schema.A2ACard, error) {
	return s.router.GetA2ACard(id)
}

func (s *MessageService) DeleteA2ACard(id int64) error {
	return s.router.DeleteA2ACard(id)
}
