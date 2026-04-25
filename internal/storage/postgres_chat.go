package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fisk086/sya/internal/model"
	"github.com/fisk086/sya/internal/schema"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *PostgresStorage) CreateChatSession(ctx context.Context, agentID int64, userID string, groupID int64) (*schema.ChatSession, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM agents WHERE id = $1`, agentID).Scan(&n)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrAgentNotFound
	}
	id := uuid.NewString()
	var uid any
	if userID == "" {
		uid = nil
	} else {
		uid = userID
	}
	var gid any
	if groupID > 0 {
		gid = groupID
	} else {
		gid = nil
	}
	var created, updated time.Time
	err = s.pool.QueryRow(ctx,
		`INSERT INTO chat_sessions (id, agent_id, user_id, group_id) VALUES ($1, $2, $3, $4) RETURNING created_at, updated_at`,
		id, agentID, uid, gid,
	).Scan(&created, &updated)
	if err != nil {
		return nil, err
	}
	out := &schema.ChatSession{
		SessionID: id,
		AgentID:   agentID,
		UserID:    userID,
		Title:     "",
		CreatedAt: created,
		UpdatedAt: updated,
	}
	if groupID > 0 {
		out.GroupID = groupID
	}
	return out, nil
}

func (s *PostgresStorage) ListChatSessions(ctx context.Context, agentID int64, userID string, limit, offset int) ([]schema.ChatSession, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var q string
	var args []any
	if userID == "" {
		q = `SELECT id, agent_id, COALESCE(user_id, ''), COALESCE(title, ''), created_at, updated_at, group_id FROM chat_sessions
			WHERE agent_id = $1 ORDER BY updated_at DESC LIMIT $2 OFFSET $3`
		args = []any{agentID, limit, offset}
	} else {
		q = `SELECT id, agent_id, COALESCE(user_id, ''), COALESCE(title, ''), created_at, updated_at, group_id FROM chat_sessions
			WHERE agent_id = $1 AND user_id = $2 ORDER BY updated_at DESC LIMIT $3 OFFSET $4`
		args = []any{agentID, userID, limit, offset}
	}
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []schema.ChatSession
	for rows.Next() {
		var sess schema.ChatSession
		var gID sql.NullInt64
		if err := rows.Scan(&sess.SessionID, &sess.AgentID, &sess.UserID, &sess.Title, &sess.CreatedAt, &sess.UpdatedAt, &gID); err != nil {
			return nil, err
		}
		if gID.Valid {
			sess.GroupID = gID.Int64
		}
		list = append(list, sess)
	}
	return list, rows.Err()
}

func (s *PostgresStorage) GetChatSession(ctx context.Context, sessionID string) (*schema.ChatSession, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, agent_id, COALESCE(user_id, ''), COALESCE(title, ''), created_at, updated_at, group_id FROM chat_sessions WHERE id = $1`,
		sessionID,
	)
	var sess schema.ChatSession
	var gID sql.NullInt64
	err := row.Scan(&sess.SessionID, &sess.AgentID, &sess.UserID, &sess.Title, &sess.CreatedAt, &sess.UpdatedAt, &gID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	if gID.Valid {
		sess.GroupID = gID.Int64
	}
	return &sess, nil
}

func (s *PostgresStorage) UpdateChatSessionTitle(ctx context.Context, sessionID, userID, title string) error {
	title = strings.TrimSpace(title)
	if len([]rune(title)) > 512 {
		title = string([]rune(title)[:512])
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE chat_sessions SET title = $1, updated_at = NOW() WHERE id = $2 AND user_id = $3`,
		title, sessionID, userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var n int64
		_ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM chat_sessions WHERE id = $1`, sessionID).Scan(&n)
		if n == 0 {
			return ErrSessionNotFound
		}
		return ErrSessionForbidden
	}
	return nil
}

func (s *PostgresStorage) DeleteChatSession(ctx context.Context, sessionID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM agent_memory WHERE session_id = $1`, sessionID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM chat_sessions WHERE id = $1`, sessionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrSessionNotFound
	}
	return tx.Commit(ctx)
}

func (s *PostgresStorage) ListRecentSessionMessages(ctx context.Context, sessionID string, limit int) ([]schema.ChatHistoryMessage, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	// Newest-first subquery, then ASC so UI / retrieveHistory see chronological order.
	rows, err := s.pool.Query(ctx,
		`SELECT id, agent_id, role, content, COALESCE(extra, '{}'::jsonb), created_at FROM (
			SELECT id, agent_id, role, content, extra, created_at FROM agent_memory
			WHERE session_id = $1 ORDER BY created_at DESC LIMIT $2
		) AS recent ORDER BY created_at ASC`,
		sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []schema.ChatHistoryMessage
	for rows.Next() {
		var m schema.ChatHistoryMessage
		var extraBytes []byte
		if err := rows.Scan(&m.ID, &m.AgentID, &m.Role, &m.Content, &extraBytes, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.ImageURLs, m.FileURLs = ChatHistoryAttachmentsFromExtraJSONB(extraBytes)
		m.ReactSteps = ChatHistoryReactStepsFromExtraJSONB(extraBytes)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		return []schema.ChatHistoryMessage{}, nil
	}
	return out, nil
}

func (s *PostgresStorage) ListSessionMessagesPage(ctx context.Context, sessionID string, offset, limit int) ([]schema.ChatHistoryMessage, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, agent_id, role, content, COALESCE(extra, '{}'::jsonb), created_at FROM agent_memory
		 WHERE session_id = $1 ORDER BY created_at ASC OFFSET $2 LIMIT $3`,
		sessionID, offset, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []schema.ChatHistoryMessage
	for rows.Next() {
		var m schema.ChatHistoryMessage
		var extraBytes []byte
		if err := rows.Scan(&m.ID, &m.AgentID, &m.Role, &m.Content, &extraBytes, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.ImageURLs, m.FileURLs = ChatHistoryAttachmentsFromExtraJSONB(extraBytes)
		m.ReactSteps = ChatHistoryReactStepsFromExtraJSONB(extraBytes)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		return []schema.ChatHistoryMessage{}, nil
	}
	return out, nil
}

func (s *PostgresStorage) GetChatStats(ctx context.Context, userID string, isAdmin bool) (map[string]int64, error) {
	var totalChats, totalSessions, totalMessages int64

	if isAdmin {
		err := s.pool.QueryRow(ctx, `SELECT COUNT(DISTINCT agent_id), COUNT(*), (SELECT COUNT(*) FROM agent_memory) FROM chat_sessions`).Scan(&totalChats, &totalSessions, &totalMessages)
		if err != nil {
			return nil, err
		}
	} else {
		err := s.pool.QueryRow(ctx, `SELECT COUNT(DISTINCT agent_id), COUNT(*), (SELECT COUNT(*) FROM agent_memory am JOIN chat_sessions cs ON am.session_id = cs.id WHERE cs.user_id = $1) FROM chat_sessions WHERE user_id = $1`, userID).Scan(&totalChats, &totalSessions, &totalMessages)
		if err != nil {
			return nil, err
		}
	}

	var totalAgents int64
	if isAdmin {
		err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM agents`).Scan(&totalAgents)
		if err != nil {
			return nil, err
		}
	} else {
		err := s.pool.QueryRow(ctx, `SELECT COUNT(DISTINCT agent_id) FROM user_roles ur JOIN rbac_role_agent_permissions rap ON ur.role_id = rap.role_id WHERE ur.user_id = $1 AND ur.is_active = true`, userID).Scan(&totalAgents)
		if err != nil {
			return nil, err
		}
	}

	return map[string]int64{
		"total_chats":    totalChats,
		"total_sessions": totalSessions,
		"total_messages": totalMessages,
		"total_agents":   totalAgents,
	}, nil
}

func (s *PostgresStorage) GetRecentChats(ctx context.Context, userID string, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	rows, err := s.pool.Query(ctx,
		`SELECT cs.id, cs.agent_id, a.public_id, a.name, cs.updated_at, cs.title
		 FROM chat_sessions cs
		 JOIN agents a ON cs.agent_id = a.id
		 WHERE cs.user_id = $1
		 ORDER BY cs.updated_at DESC
		 LIMIT $2`,
		userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var sessionID, title string
		var agentID int64
		var agentPublicID, agentName string
		var updatedAt time.Time
		if err := rows.Scan(&sessionID, &agentID, &agentPublicID, &agentName, &updatedAt, &title); err != nil {
			return nil, err
		}
		results = append(results, map[string]any{
			"session_id":      sessionID,
			"agent_id":        agentID,
			"agent_public_id": agentPublicID,
			"agent_name":      agentName,
			"updated_at":      updatedAt.Format("2006-01-02 15:04:05"),
			"title":           title,
			"is_active":       true,
		})
	}
	return results, nil
}

func (s *PostgresStorage) GetChatActivity(ctx context.Context, userID string, days int) ([]map[string]any, error) {
	if days <= 0 || days > 30 {
		days = 7
	}

	// 必须用 ($n * INTERVAL '1 day')；写成 INTERVAL '$2 days' 时引号内是字面量，参数不会代入，查询会失败，仪表盘「会话活动」恒为空。
	rows, err := s.pool.Query(ctx,
		`SELECT TO_CHAR(created_at::date, 'MM-DD') AS date, COUNT(*)::bigint AS count
		 FROM chat_sessions
		 WHERE user_id = $1 AND created_at >= NOW() - ($2::int * INTERVAL '1 day')
		 GROUP BY created_at::date
		 ORDER BY created_at::date`,
		userID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var date string
		var count int64
		if err := rows.Scan(&date, &count); err != nil {
			return nil, err
		}
		results = append(results, map[string]any{
			"date":  date,
			"count": count,
		})
	}
	return results, nil
}

func (s *PostgresStorage) CreateMessage(ctx context.Context, msg *model.AgentMessage) (*model.AgentMessage, error) {
	var id int64
	var deliveredAt *time.Time
	err := s.pool.QueryRow(ctx,
		`INSERT INTO agent_messages (from_agent_id, to_agent_id, channel_id, session_id, kind, content, metadata, status, priority)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, delivered_at`,
		msg.FromAgentID, msg.ToAgentID, msg.ChannelID, msg.SessionID, msg.Kind, msg.Content, msg.Metadata, msg.Status, msg.Priority,
	).Scan(&id, &deliveredAt)
	if err != nil {
		return nil, err
	}
	msg.ID = id
	msg.DeliveredAt = deliveredAt
	return msg, nil
}

func (s *PostgresStorage) ListMessages(ctx context.Context, req *schema.ListMessagesRequest) ([]*model.AgentMessage, int64, error) {
	where := "1=1"
	if req.ChannelID > 0 {
		where += fmt.Sprintf(" AND channel_id = %d", req.ChannelID)
	}
	if req.AgentID > 0 {
		where += fmt.Sprintf(" AND (from_agent_id = %d OR to_agent_id = %d)", req.AgentID, req.AgentID)
	}
	if req.SessionID != "" {
		where += fmt.Sprintf(" AND session_id = '%s'", req.SessionID)
	}
	if req.Status != "" {
		where += fmt.Sprintf(" AND status = '%s'", req.Status)
	}

	var total int64
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM agent_messages WHERE "+where).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := req.Offset
	if offset < 0 {
		offset = 0
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.pool.Query(ctx,
		fmt.Sprintf(`SELECT id, from_agent_id, to_agent_id, channel_id, session_id, kind, content, metadata, status, priority, created_at, delivered_at
			FROM agent_messages WHERE %s ORDER BY created_at DESC LIMIT %d OFFSET %d`, where, limit, offset),
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var msgs []*model.AgentMessage
	for rows.Next() {
		var msg model.AgentMessage
		var deliveredAt *time.Time
		if err := rows.Scan(&msg.ID, &msg.FromAgentID, &msg.ToAgentID, &msg.ChannelID, &msg.SessionID, &msg.Kind, &msg.Content, &msg.Metadata, &msg.Status, &msg.Priority, &msg.CreatedAt, &deliveredAt); err != nil {
			return nil, 0, err
		}
		msg.DeliveredAt = deliveredAt
		msgs = append(msgs, &msg)
	}
	return msgs, total, nil
}

func (s *PostgresStorage) UpdateMessageStatus(ctx context.Context, id int64, status string) error {
	var deliveredAt *time.Time
	if status == "delivered" {
		now := time.Now()
		deliveredAt = &now
	}
	_, err := s.pool.Exec(ctx,
		"UPDATE agent_messages SET status = $1, delivered_at = $2 WHERE id = $3",
		status, deliveredAt, id,
	)
	return err
}

func (s *PostgresStorage) CreateMessageChannel(ctx context.Context, req *schema.CreateMessageChannelRequest) (*model.MessageChannel, error) {
	var id int64
	var createdAt, updatedAt time.Time
	err := s.pool.QueryRow(ctx,
		`INSERT INTO message_channels (name, agent_id, kind, description, is_public, metadata, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at, updated_at`,
		req.Name, req.AgentID, req.Kind, req.Description, req.IsPublic, req.Metadata, req.IsActive,
	).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	return &model.MessageChannel{
		ID:          id,
		Name:        req.Name,
		AgentID:     req.AgentID,
		Kind:        req.Kind,
		Description: req.Description,
		IsPublic:    req.IsPublic,
		Metadata:    req.Metadata,
		IsActive:    req.IsActive,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func (s *PostgresStorage) GetMessageChannel(ctx context.Context, id int64) (*model.MessageChannel, error) {
	var ch model.MessageChannel
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, agent_id, kind, description, is_public, metadata, is_active, created_at, updated_at FROM message_channels WHERE id = $1`,
		id,
	).Scan(&ch.ID, &ch.Name, &ch.AgentID, &ch.Kind, &ch.Description, &ch.IsPublic, &ch.Metadata, &ch.IsActive, &ch.CreatedAt, &ch.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

func (s *PostgresStorage) ListMessageChannels(ctx context.Context, agentID int64) ([]*model.MessageChannel, error) {
	var q string
	var args []any
	if agentID > 0 {
		q = "SELECT id, name, agent_id, kind, description, is_public, metadata, is_active, created_at, updated_at FROM message_channels WHERE agent_id = $1"
		args = []any{agentID}
	} else {
		q = "SELECT id, name, agent_id, kind, description, is_public, metadata, is_active, created_at, updated_at FROM message_channels"
		args = []any{}
	}

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*model.MessageChannel
	for rows.Next() {
		var ch model.MessageChannel
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.AgentID, &ch.Kind, &ch.Description, &ch.IsPublic, &ch.Metadata, &ch.IsActive, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, &ch)
	}
	return result, nil
}

func (s *PostgresStorage) UpdateMessageChannel(ctx context.Context, id int64, req *schema.UpdateMessageChannelRequest) (*model.MessageChannel, error) {
	var name, description string
	var isPublic, isActive bool
	var metadata []byte

	err := s.pool.QueryRow(ctx,
		`UPDATE message_channels SET 
			name = COALESCE($1, name),
			description = COALESCE($2, description),
			is_public = COALESCE($3, is_public),
			metadata = COALESCE($4, metadata),
			is_active = COALESCE($5, is_active),
			updated_at = NOW()
		 WHERE id = $6
		 RETURNING id, name, agent_id, kind, description, is_public, metadata, is_active, created_at, updated_at`,
		req.Name, req.Description, req.IsPublic, req.Metadata, req.IsActive, id,
	).Scan(&id, &name, &metadata, &description, &isPublic, &metadata, &isActive, &metadata, &metadata, &metadata)
	if err != nil {
		return nil, err
	}
	return &model.MessageChannel{ID: id}, nil
}

func (s *PostgresStorage) DeleteMessageChannel(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM message_channels WHERE id = $1", id)
	return err
}

func (s *PostgresStorage) CreateA2ACard(ctx context.Context, req *schema.CreateA2ACardRequest) (*model.A2ACard, error) {
	var id int64
	var createdAt time.Time
	err := s.pool.QueryRow(ctx,
		`INSERT INTO a2a_cards (agent_id, name, description, url, version, capabilities, is_active)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at`,
		req.AgentID, req.Name, req.Description, req.URL, req.Version, req.Capabilities, req.IsActive,
	).Scan(&id, &createdAt)
	if err != nil {
		return nil, err
	}
	return &model.A2ACard{
		ID:           id,
		AgentID:      req.AgentID,
		Name:         req.Name,
		Description:  req.Description,
		URL:          req.URL,
		Version:      req.Version,
		Capabilities: req.Capabilities,
		IsActive:     req.IsActive,
		CreatedAt:    createdAt,
	}, nil
}

func (s *PostgresStorage) ListA2ACards(ctx context.Context, agentID int64) ([]*model.A2ACard, error) {
	var q string
	var args []any
	if agentID > 0 {
		q = "SELECT id, agent_id, name, description, url, version, capabilities, is_active, created_at FROM a2a_cards WHERE agent_id = $1"
		args = []any{agentID}
	} else {
		q = "SELECT id, agent_id, name, description, url, version, capabilities, is_active, created_at FROM a2a_cards"
		args = []any{}
	}

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*model.A2ACard
	for rows.Next() {
		var card model.A2ACard
		if err := rows.Scan(&card.ID, &card.AgentID, &card.Name, &card.Description, &card.URL, &card.Version, &card.Capabilities, &card.IsActive, &card.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, &card)
	}
	return result, nil
}

func (s *PostgresStorage) GetA2ACard(ctx context.Context, id int64) (*model.A2ACard, error) {
	var card model.A2ACard
	err := s.pool.QueryRow(ctx,
		`SELECT id, agent_id, name, description, url, version, capabilities, is_active, created_at FROM a2a_cards WHERE id = $1`,
		id,
	).Scan(&card.ID, &card.AgentID, &card.Name, &card.Description, &card.URL, &card.Version, &card.Capabilities, &card.IsActive, &card.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &card, nil
}

func (s *PostgresStorage) DeleteA2ACard(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM a2a_cards WHERE id = $1", id)
	return err
}

// Chat Group operations

func (s *PostgresStorage) CreateChatGroup(ctx context.Context, req *schema.CreateGroupRequest, userID string) (*model.ChatGroup, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var groupID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO chat_groups (name, created_by) VALUES ($1, $2) RETURNING id`,
		req.Name, userID,
	).Scan(&groupID)
	if err != nil {
		return nil, err
	}

	// Insert members
	for _, agentID := range req.AgentIDs {
		_, err = tx.Exec(ctx,
			`INSERT INTO chat_group_members (group_id, agent_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			groupID, agentID,
		)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.GetChatGroup(ctx, groupID)
}

func (s *PostgresStorage) GetChatGroup(ctx context.Context, id int64) (*model.ChatGroup, error) {
	var group model.ChatGroup
	err := s.pool.QueryRow(ctx,
		`SELECT g.id, g.name, COALESCE(g.created_by, ''), g.created_at,
			COALESCE((SELECT MAX(cs.updated_at) FROM chat_sessions cs WHERE cs.group_id = g.id), g.created_at)
		 FROM chat_groups g WHERE g.id = $1`,
		id,
	).Scan(&group.ID, &group.Name, &group.CreatedBy, &group.CreatedAt, &group.UpdatedAt)
	if err != nil {
		return nil, err
	}

	// Get members
	rows, err := s.pool.Query(ctx,
		`SELECT m.agent_id, COALESCE(a.name, '') 
		 FROM chat_group_members m 
		 LEFT JOIN agents a ON m.agent_id = a.id 
		 WHERE m.group_id = $1`,
		id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var member model.ChatGroupMember
		var agentName string
		if err := rows.Scan(&member.AgentID, &agentName); err != nil {
			return nil, err
		}
		member.GroupID = id
		member.AgentName = agentName
		group.Members = append(group.Members, member)
	}

	return &group, nil
}

func (s *PostgresStorage) ListChatGroups(ctx context.Context, userID string) ([]*model.ChatGroup, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT g.id, g.name, COALESCE(g.created_by, ''), g.created_at,
			COALESCE(MAX(cs.updated_at), g.created_at) AS updated_at
		 FROM chat_groups g
		 LEFT JOIN chat_sessions cs ON cs.group_id = g.id
		 GROUP BY g.id, g.name, g.created_by, g.created_at
		 ORDER BY COALESCE(MAX(cs.updated_at), g.created_at) DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*model.ChatGroup
	for rows.Next() {
		var group model.ChatGroup
		if err := rows.Scan(&group.ID, &group.Name, &group.CreatedBy, &group.CreatedAt, &group.UpdatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, &group)
	}

	// Get members for each group
	for _, group := range groups {
		memberRows, err := s.pool.Query(ctx,
			`SELECT m.agent_id, COALESCE(a.name, '') 
			 FROM chat_group_members m 
			 LEFT JOIN agents a ON m.agent_id = a.id 
			 WHERE m.group_id = $1`,
			group.ID,
		)
		if err != nil {
			return nil, err
		}
		for memberRows.Next() {
			var member model.ChatGroupMember
			var agentName string
			if err := memberRows.Scan(&member.AgentID, &agentName); err != nil {
				memberRows.Close()
				return nil, err
			}
			member.GroupID = group.ID
			member.AgentName = agentName
			group.Members = append(group.Members, member)
		}
		memberRows.Close()
	}

	return groups, nil
}

func (s *PostgresStorage) UpdateChatGroup(ctx context.Context, id int64, req *schema.UpdateGroupRequest) (*model.ChatGroup, error) {
	if req.Name != nil {
		_, err := s.pool.Exec(ctx, `UPDATE chat_groups SET name = $1 WHERE id = $2`, *req.Name, id)
		if err != nil {
			return nil, err
		}
	}

	if len(req.AgentIDs) > 0 {
		// Replace all members
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx, `DELETE FROM chat_group_members WHERE group_id = $1`, id)
		if err != nil {
			return nil, err
		}

		for _, agentID := range req.AgentIDs {
			_, err = tx.Exec(ctx,
				`INSERT INTO chat_group_members (group_id, agent_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				id, agentID,
			)
			if err != nil {
				return nil, err
			}
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
	}

	return s.GetChatGroup(ctx, id)
}

func (s *PostgresStorage) DeleteChatGroup(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM chat_groups WHERE id = $1`, id)
	return err
}
