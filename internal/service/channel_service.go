package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/notify"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/storage"
)

type ChannelService struct {
	store storage.Storage
}

func NewChannelService(store storage.Storage) *ChannelService {
	return &ChannelService{store: store}
}

func ChannelToPublic(m *model.Channel) schema.Channel {
	ex := m.Extra
	if ex == nil {
		ex = map[string]string{}
	}
	return schema.Channel{
		ID:           m.ID,
		Name:         m.Name,
		Kind:         m.Kind,
		WebhookURL:   m.WebhookURL,
		AppID:        m.AppID,
		HasAppSecret: strings.TrimSpace(m.AppSecret) != "",
		Extra:        ex,
		IsActive:     m.IsActive,
		CreatedAt:    m.CreatedAt,
	}
}

func (s *ChannelService) List() ([]schema.Channel, error) {
	rows, err := s.store.ListChannels()
	if err != nil {
		return nil, err
	}
	out := make([]schema.Channel, 0, len(rows))
	for _, r := range rows {
		out = append(out, ChannelToPublic(r))
	}
	return out, nil
}

func (s *ChannelService) Get(id int64) (schema.Channel, error) {
	m, err := s.store.GetChannel(id)
	if err != nil {
		return schema.Channel{}, err
	}
	return ChannelToPublic(m), nil
}

func (s *ChannelService) Create(req *schema.CreateChannelRequest) (schema.Channel, error) {
	if err := validateChannelCreate(req); err != nil {
		return schema.Channel{}, err
	}
	m, err := s.store.CreateChannel(req)
	if err != nil {
		return schema.Channel{}, err
	}
	return ChannelToPublic(m), nil
}

func (s *ChannelService) Update(id int64, req *schema.UpdateChannelRequest) (schema.Channel, error) {
	m, err := s.store.GetChannel(id)
	if err != nil {
		return schema.Channel{}, err
	}
	if req.Name == "" && req.WebhookURL == "" && req.AppID == "" && req.AppSecret == "" && req.Extra == nil && req.IsActive == nil {
		return ChannelToPublic(m), nil
	}
	_, err = s.store.UpdateChannel(id, req)
	if err != nil {
		return schema.Channel{}, err
	}
	m2, err := s.store.GetChannel(id)
	if err != nil {
		return schema.Channel{}, err
	}
	return ChannelToPublic(m2), nil
}

func (s *ChannelService) Delete(id int64) error {
	return s.store.DeleteChannel(id)
}

func (s *ChannelService) SendTest(ctx context.Context, id int64, message string) error {
	m, err := s.store.GetChannel(id)
	if err != nil {
		return err
	}
	if !m.IsActive {
		return fmt.Errorf("channel is inactive")
	}
	return notify.SendText(ctx, m.Kind, m.WebhookURL, m.AppID, m.AppSecret, m.Extra, message)
}

func validateChannelCreate(req *schema.CreateChannelRequest) error {
	k := strings.ToLower(strings.TrimSpace(req.Kind))
	switch k {
	case "lark", "dingtalk", "wecom":
	default:
		return fmt.Errorf("kind must be lark, dingtalk, or wecom")
	}
	req.Kind = k
	hasHook := strings.TrimSpace(req.WebhookURL) != ""
	hasCreds := strings.TrimSpace(req.AppID) != "" && strings.TrimSpace(req.AppSecret) != ""
	if k == "lark" {
		if !hasHook && !hasCreds {
			return fmt.Errorf("lark: provide webhook_url or app_id+app_secret (and extra.receive_id for app credentials)")
		}
		if hasCreds {
			rid := ""
			if req.Extra != nil {
				rid = strings.TrimSpace(req.Extra["receive_id"])
			}
			if rid == "" {
				return fmt.Errorf("lark: extra.receive_id is required when using app_id and app_secret")
			}
		}
		return nil
	}
	if k == "dingtalk" || k == "wecom" {
		if !hasHook {
			return fmt.Errorf("%s: webhook_url is required", k)
		}
	}
	return nil
}
