package storage

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/fisk086/sya/internal/model"
	"github.com/jackc/pgx/v5"
)

func nullableFKID(id int64) interface{} {
	if id <= 0 {
		return nil
	}
	return id
}

func scanScheduleRow(
	sch *model.Schedule,
	agentID, wfID, chID sql.NullInt64,
	codeLang sql.NullString,
) {
	if agentID.Valid {
		sch.AgentID = agentID.Int64
	} else {
		sch.AgentID = 0
	}
	if wfID.Valid {
		sch.WorkflowID = wfID.Int64
	} else {
		sch.WorkflowID = 0
	}
	if codeLang.Valid {
		sch.CodeLanguage = codeLang.String
	} else {
		sch.CodeLanguage = ""
	}
	if chID.Valid {
		v := chID.Int64
		sch.ChannelID = &v
	} else {
		sch.ChannelID = nil
	}
}

func (s *PostgresStorage) ListSchedules() ([]*model.Schedule, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT id, name, description, agent_id, workflow_id, channel_id, COALESCE(owner_user_id, ''), COALESCE(chat_session_id, ''), schedule_kind, cron_expr, at_time, every_ms, timezone, wake_mode, session_target, prompt, COALESCE(code_language, ''), stagger_ms, enabled, created_at, updated_at
		 FROM schedules ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []*model.Schedule
	for rows.Next() {
		sch := &model.Schedule{}
		var agentID, wfID, chID sql.NullInt64
		var codeLang sql.NullString
		if err := rows.Scan(&sch.ID, &sch.Name, &sch.Description, &agentID, &wfID, &chID, &sch.OwnerUserID, &sch.ChatSessionID, &sch.ScheduleKind, &sch.CronExpr, &sch.At, &sch.EveryMs, &sch.Timezone, &sch.WakeMode, &sch.SessionTarget, &sch.Prompt, &codeLang, &sch.StaggerMs, &sch.Enabled, &sch.CreatedAt, &sch.UpdatedAt); err != nil {
			return nil, err
		}
		scanScheduleRow(sch, agentID, wfID, chID, codeLang)
		schedules = append(schedules, sch)
	}
	return schedules, nil
}

func (s *PostgresStorage) GetSchedule(id int64) (*model.Schedule, error) {
	sch := &model.Schedule{}
	var agentID, wfID, chID sql.NullInt64
	var codeLang sql.NullString
	err := s.pool.QueryRow(context.Background(),
		`SELECT id, name, description, agent_id, workflow_id, channel_id, COALESCE(owner_user_id, ''), COALESCE(chat_session_id, ''), schedule_kind, cron_expr, at_time, every_ms, timezone, wake_mode, session_target, prompt, COALESCE(code_language, ''), stagger_ms, enabled, created_at, updated_at
		 FROM schedules WHERE id = $1`, id).
		Scan(&sch.ID, &sch.Name, &sch.Description, &agentID, &wfID, &chID, &sch.OwnerUserID, &sch.ChatSessionID, &sch.ScheduleKind, &sch.CronExpr, &sch.At, &sch.EveryMs, &sch.Timezone, &sch.WakeMode, &sch.SessionTarget, &sch.Prompt, &codeLang, &sch.StaggerMs, &sch.Enabled, &sch.CreatedAt, &sch.UpdatedAt)
	if err != nil {
		return nil, err
	}
	scanScheduleRow(sch, agentID, wfID, chID, codeLang)
	return sch, nil
}

func (s *PostgresStorage) CreateSchedule(schedule *model.Schedule) (*model.Schedule, error) {
	now := time.Now()
	var chID interface{}
	if schedule.ChannelID != nil {
		chID = *schedule.ChannelID
	}
	var ouid any
	if strings.TrimSpace(schedule.OwnerUserID) != "" {
		ouid = schedule.OwnerUserID
	}
	agentID := nullableFKID(schedule.AgentID)
	wfID := nullableFKID(schedule.WorkflowID)
	var codeLang interface{}
	if cl := strings.TrimSpace(schedule.CodeLanguage); cl != "" {
		codeLang = cl
	}
	err := s.pool.QueryRow(context.Background(),
		`INSERT INTO schedules (name, description, agent_id, workflow_id, channel_id, owner_user_id, schedule_kind, cron_expr, at_time, every_ms, timezone, wake_mode, session_target, prompt, code_language, stagger_ms, enabled, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		 RETURNING id, created_at, updated_at`,
		schedule.Name, schedule.Description, agentID, wfID, chID, ouid, schedule.ScheduleKind, schedule.CronExpr, schedule.At, schedule.EveryMs, schedule.Timezone, schedule.WakeMode, schedule.SessionTarget, schedule.Prompt, codeLang, schedule.StaggerMs, schedule.Enabled, now, now).
		Scan(&schedule.ID, &schedule.CreatedAt, &schedule.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return schedule, nil
}

func (s *PostgresStorage) UpdateScheduleChatSessionID(id int64, chatSessionID string) error {
	_, err := s.pool.Exec(context.Background(),
		`UPDATE schedules SET chat_session_id = $1, updated_at = NOW() WHERE id = $2`,
		chatSessionID, id)
	return err
}

func (s *PostgresStorage) UpdateSchedule(id int64, schedule *model.Schedule) (*model.Schedule, error) {
	now := time.Now()
	var chID interface{}
	if schedule.ChannelID != nil {
		chID = *schedule.ChannelID
	}
	agentID := nullableFKID(schedule.AgentID)
	wfID := nullableFKID(schedule.WorkflowID)
	var codeLang interface{}
	if cl := strings.TrimSpace(schedule.CodeLanguage); cl != "" {
		codeLang = cl
	}
	var agentOut, wfOut sql.NullInt64
	var chIDOut sql.NullInt64
	var codeOut sql.NullString
	err := s.pool.QueryRow(context.Background(),
		`UPDATE schedules SET
			name = COALESCE($1, name),
			description = COALESCE($2, description),
			agent_id = $3,
			workflow_id = $4,
			code_language = $5,
			schedule_kind = COALESCE($6, schedule_kind),
			cron_expr = COALESCE($7, cron_expr),
			at_time = COALESCE($8, at_time),
			every_ms = COALESCE($9, every_ms),
			timezone = COALESCE($10, timezone),
			wake_mode = COALESCE($11, wake_mode),
			session_target = COALESCE(NULLIF($12, ''), session_target),
			prompt = COALESCE($13, prompt),
			stagger_ms = COALESCE($14, stagger_ms),
			enabled = COALESCE($15, enabled),
			channel_id = $16,
			updated_at = $17,
			owner_user_id = COALESCE(NULLIF($18, ''), owner_user_id)
		 WHERE id = $19
		 RETURNING id, name, description, agent_id, workflow_id, channel_id, COALESCE(owner_user_id, ''), COALESCE(chat_session_id, ''), schedule_kind, cron_expr, at_time, every_ms, timezone, wake_mode, session_target, prompt, COALESCE(code_language, ''), stagger_ms, enabled, created_at, updated_at`,
		schedule.Name, schedule.Description, agentID, wfID, codeLang, schedule.ScheduleKind, schedule.CronExpr, schedule.At, schedule.EveryMs, schedule.Timezone, schedule.WakeMode, schedule.SessionTarget, schedule.Prompt, schedule.StaggerMs, schedule.Enabled, chID, now, schedule.OwnerUserID, id).
		Scan(&schedule.ID, &schedule.Name, &schedule.Description, &agentOut, &wfOut, &chIDOut, &schedule.OwnerUserID, &schedule.ChatSessionID, &schedule.ScheduleKind, &schedule.CronExpr, &schedule.At, &schedule.EveryMs, &schedule.Timezone, &schedule.WakeMode, &schedule.SessionTarget, &schedule.Prompt, &codeOut, &schedule.StaggerMs, &schedule.Enabled, &schedule.CreatedAt, &schedule.UpdatedAt)
	if err != nil {
		return nil, err
	}
	scanScheduleRow(schedule, agentOut, wfOut, chIDOut, codeOut)
	return schedule, nil
}

func (s *PostgresStorage) DeleteSchedule(id int64) error {
	_, err := s.pool.Exec(context.Background(), `DELETE FROM schedules WHERE id = $1`, id)
	return err
}

func (s *PostgresStorage) ListScheduleExecutions(scheduleID int64, limit int) ([]*model.ScheduleExecution, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var rows pgx.Rows
	var err error
	if scheduleID > 0 {
		rows, err = s.pool.Query(context.Background(),
			`SELECT id, schedule_id, status, result, error, duration_ms, started_at, finished_at
			 FROM schedule_executions WHERE schedule_id = $1 ORDER BY id DESC LIMIT $2`, scheduleID, limit)
	} else {
		rows, err = s.pool.Query(context.Background(),
			`SELECT id, schedule_id, status, result, error, duration_ms, started_at, finished_at
			 FROM schedule_executions ORDER BY id DESC LIMIT $1`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var execs []*model.ScheduleExecution
	for rows.Next() {
		exec := &model.ScheduleExecution{}
		if err := rows.Scan(&exec.ID, &exec.ScheduleID, &exec.Status, &exec.Result, &exec.Error, &exec.DurationMs, &exec.StartedAt, &exec.FinishedAt); err != nil {
			return nil, err
		}
		execs = append(execs, exec)
	}
	return execs, nil
}

func (s *PostgresStorage) CreateScheduleExecution(exec *model.ScheduleExecution) (*model.ScheduleExecution, error) {
	err := s.pool.QueryRow(context.Background(),
		`INSERT INTO schedule_executions (schedule_id, status, started_at)
		 VALUES ($1, $2, NOW())
		 RETURNING id, started_at`,
		exec.ScheduleID, exec.Status).
		Scan(&exec.ID, &exec.StartedAt)
	if err != nil {
		return nil, err
	}
	return exec, nil
}

func (s *PostgresStorage) UpdateScheduleExecution(id int64, status string, result, execErr string, durationMs int64) error {
	_, err := s.pool.Exec(context.Background(),
		`UPDATE schedule_executions SET status = $1, result = $2, error = $3, duration_ms = $4, finished_at = NOW() WHERE id = $5`,
		status, result, execErr, durationMs, id)
	return err
}
