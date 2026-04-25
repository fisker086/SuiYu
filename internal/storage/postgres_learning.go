package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/fisk086/sya/internal/schema"
)

func (s *PostgresStorage) CreateLearning(req *schema.CreateLearningRequest) (*schema.Learning, error) {
	var id int64
	var createdAt, updatedAt time.Time
	err := s.pool.QueryRow(context.Background(),
		`INSERT INTO learnings (user_id, error_type, context, root_cause, fix, lesson) 
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, error_type) DO UPDATE SET times = learnings.times + 1, lesson = EXCLUDED.lesson, fix = EXCLUDED.fix, root_cause = EXCLUDED.root_cause, context = EXCLUDED.context, updated_at = NOW()
		RETURNING id, created_at, updated_at`,
		req.UserID, req.ErrorType, req.Context, req.RootCause, req.Fix, req.Lesson).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	return &schema.Learning{
		ID:        id,
		UserID:    req.UserID,
		ErrorType: req.ErrorType,
		Context:   req.Context,
		RootCause: req.RootCause,
		Fix:       req.Fix,
		Lesson:    req.Lesson,
		Times:     1,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func (s *PostgresStorage) ListLearnings(userID *int64) ([]*schema.Learning, error) {
	query := `SELECT id, user_id, error_type, context, root_cause, fix, lesson, times, created_at, updated_at FROM learnings`
	var args []interface{}

	if userID == nil {
		query += " WHERE user_id IS NULL ORDER BY times DESC, updated_at DESC"
	} else {
		query += " WHERE user_id = $1 OR user_id IS NULL ORDER BY times DESC, updated_at DESC"
		args = append(args, *userID)
	}

	rows, err := s.pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var learnings []*schema.Learning
	for rows.Next() {
		var l schema.Learning
		var userID *int64
		if err := rows.Scan(&l.ID, &userID, &l.ErrorType, &l.Context, &l.RootCause, &l.Fix, &l.Lesson, &l.Times, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, err
		}
		l.UserID = userID
		learnings = append(learnings, &l)
	}
	return learnings, nil
}

func (s *PostgresStorage) GetLearning(userID *int64, errorType string) (*schema.Learning, error) {
	var l schema.Learning
	var uid *int64
	err := s.pool.QueryRow(context.Background(),
		`SELECT id, user_id, error_type, context, root_cause, fix, lesson, times, created_at, updated_at 
		FROM learnings WHERE (user_id = $1 OR user_id IS NULL) AND error_type = $2`,
		userID, errorType).Scan(&l.ID, &uid, &l.ErrorType, &l.Context, &l.RootCause, &l.Fix, &l.Lesson, &l.Times, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("learning not found: %v", err)
	}
	l.UserID = uid
	return &l, nil
}

func (s *PostgresStorage) DeleteLearning(id int64) error {
	_, err := s.pool.Exec(context.Background(), `DELETE FROM learnings WHERE id = $1`, id)
	return err
}
