package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/schema"
	"github.com/jackc/pgx/v5"
)

func (s *PostgresStorage) ListChannels() ([]*model.Channel, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT id, name, kind, COALESCE(webhook_url,''), COALESCE(app_id,''), COALESCE(app_secret,''), extra, is_active, created_at FROM channels ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*model.Channel
	for rows.Next() {
		ch, err := scanChannelRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

func scanChannelRow(scanner interface {
	Scan(dest ...any) error
}) (*model.Channel, error) {
	var ch model.Channel
	var extraJSON []byte
	if err := scanner.Scan(&ch.ID, &ch.Name, &ch.Kind, &ch.WebhookURL, &ch.AppID, &ch.AppSecret, &extraJSON, &ch.IsActive, &ch.CreatedAt); err != nil {
		return nil, err
	}
	if len(extraJSON) > 0 {
		var raw map[string]interface{}
		if err := json.Unmarshal(extraJSON, &raw); err == nil && raw != nil {
			ch.Extra = make(map[string]string, len(raw))
			for k, v := range raw {
				ch.Extra[k] = fmt.Sprint(v)
			}
		}
	}
	if ch.Extra == nil {
		ch.Extra = map[string]string{}
	}
	return &ch, nil
}

func (s *PostgresStorage) GetChannel(id int64) (*model.Channel, error) {
	row := s.pool.QueryRow(context.Background(),
		`SELECT id, name, kind, COALESCE(webhook_url,''), COALESCE(app_id,''), COALESCE(app_secret,''), extra, is_active, created_at FROM channels WHERE id = $1`, id)
	ch, err := scanChannelRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrChannelNotFound
		}
		return nil, err
	}
	return ch, nil
}

func (s *PostgresStorage) CreateChannel(req *schema.CreateChannelRequest) (*model.Channel, error) {
	extraJSON, err := json.Marshal(req.Extra)
	if err != nil {
		return nil, fmt.Errorf("extra json: %w", err)
	}
	if len(extraJSON) == 0 || string(extraJSON) == "null" {
		extraJSON = []byte("{}")
	}
	var id int64
	var createdAt time.Time
	err = s.pool.QueryRow(context.Background(),
		`INSERT INTO channels (name, kind, webhook_url, app_id, app_secret, extra, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at`,
		req.Name, req.Kind, req.WebhookURL, req.AppID, req.AppSecret, extraJSON, req.IsActive,
	).Scan(&id, &createdAt)
	if err != nil {
		return nil, err
	}
	return s.GetChannel(id)
}

func (s *PostgresStorage) UpdateChannel(id int64, req *schema.UpdateChannelRequest) (*model.Channel, error) {
	cur, err := s.GetChannel(id)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		cur.Name = req.Name
	}
	if req.WebhookURL != "" {
		cur.WebhookURL = req.WebhookURL
	}
	if req.AppID != "" {
		cur.AppID = req.AppID
	}
	if req.AppSecret != "" {
		cur.AppSecret = req.AppSecret
	}
	if req.Extra != nil {
		cur.Extra = req.Extra
	}
	if req.IsActive != nil {
		cur.IsActive = *req.IsActive
	}
	extraJSON, err := json.Marshal(cur.Extra)
	if err != nil {
		return nil, err
	}
	if len(extraJSON) == 0 || string(extraJSON) == "null" {
		extraJSON = []byte("{}")
	}
	_, err = s.pool.Exec(context.Background(),
		`UPDATE channels SET name = $1, webhook_url = $2, app_id = $3, app_secret = $4, extra = $5, is_active = $6 WHERE id = $7`,
		cur.Name, cur.WebhookURL, cur.AppID, cur.AppSecret, extraJSON, cur.IsActive, id)
	if err != nil {
		return nil, err
	}
	return s.GetChannel(id)
}

func (s *PostgresStorage) DeleteChannel(id int64) error {
	tag, err := s.pool.Exec(context.Background(), `DELETE FROM channels WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrChannelNotFound
	}
	return nil
}
