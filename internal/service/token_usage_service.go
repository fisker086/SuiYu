package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/fisk086/sya/internal/logger"
	"github.com/fisk086/sya/internal/model"
	"gorm.io/gorm"
)

type TokenUsageService struct {
	db                  *gorm.DB
	defaultModelDisplay string // replace stored "default" / empty in API responses with effective OPENAI_MODEL / ARK_MODEL
}

func NewTokenUsageService(db *gorm.DB, defaultModelDisplay string) *TokenUsageService {
	return &TokenUsageService{db: db, defaultModelDisplay: strings.TrimSpace(defaultModelDisplay)}
}

type UsageStat struct {
	Date         string  `json:"date"`
	UserID       string  `json:"user_id"`
	UserName     string  `json:"user_name"`
	AgentID      int64   `json:"agent_id"`
	AgentName    string  `json:"agent_name"`
	Model        string  `json:"model"`
	PromptTokens int64   `json:"prompt_tokens"`
	Completion   int64   `json:"completion_tokens" gorm:"column:completion_tokens"` // SELECT aliases SUM(completion) AS completion_tokens
	TotalTokens  int64   `json:"total_tokens"`
	Cost         float64 `json:"cost"`
}

type UsageSummary struct {
	TotalTokens     int64       `json:"total_tokens"`
	TotalCost       float64     `json:"total_cost"`
	TotalPrompt     int64       `json:"total_prompt"`
	TotalCompletion int64       `json:"total_completion"`
	RecordCount     int64       `json:"record_count"`
	Breakdown       []UsageStat `json:"breakdown"`
}

func (s *TokenUsageService) GetUsageStats(ctx context.Context, startDate, endDate string, groupBy string) ([]UsageStat, error) {
	var results []UsageStat

	query := s.db.Model(&model.TokenUsage{})

	if startDate != "" {
		query = query.Where("date >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("date <= ?", endDate)
	}

	// Group by user and date (day level). Column for completion is `completion` (GORM default for field Completion), not `completion_tokens`.
	query = query.Select(`
		date,
		user_id,
		user_name,
		agent_id,
		agent_name,
		model,
		SUM(prompt_tokens) as prompt_tokens,
		SUM(completion) as completion_tokens,
		SUM(total_tokens) as total_tokens,
		SUM(cost) as cost
	`).Group("date, user_id, user_name, agent_id, agent_name, model").Order("date DESC, user_name, agent_name")

	if err := query.Find(&results).Error; err != nil {
		return nil, err
	}

	s.enrichDefaultModelDisplay(results)
	s.enrichUserNames(ctx, results)
	return results, nil
}

func (s *TokenUsageService) enrichDefaultModelDisplay(stats []UsageStat) {
	if len(stats) == 0 || s.defaultModelDisplay == "" {
		return
	}
	for i := range stats {
		m := strings.TrimSpace(stats[i].Model)
		if m == "" || m == "default" {
			stats[i].Model = s.defaultModelDisplay
		}
	}
}

// enrichUserNames fills user_name from users.username when user_id is numeric (用量落库时常为空).
func (s *TokenUsageService) enrichUserNames(ctx context.Context, stats []UsageStat) {
	if len(stats) == 0 || s.db == nil {
		return
	}
	seen := make(map[int64]struct{})
	var ids []int64
	for _, st := range stats {
		id, err := strconv.ParseInt(strings.TrimSpace(st.UserID), 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return
	}
	var users []model.User
	if err := s.db.WithContext(ctx).Select("id", "username").Where("id IN ?", ids).Find(&users).Error; err != nil {
		logger.Warn("usage stats: resolve usernames failed", "err", err)
		return
	}
	byID := make(map[int64]string, len(users))
	for i := range users {
		if strings.TrimSpace(users[i].Username) != "" {
			byID[users[i].ID] = users[i].Username
		}
	}
	for i := range stats {
		id, err := strconv.ParseInt(strings.TrimSpace(stats[i].UserID), 10, 64)
		if err != nil {
			continue
		}
		if name, ok := byID[id]; ok {
			stats[i].UserName = name
		}
	}
}

func (s *TokenUsageService) GetUserUsageSummary(ctx context.Context, userID string, startDate, endDate string) (*UsageSummary, error) {
	var summary UsageSummary

	query := s.db.Model(&model.TokenUsage{}).Where("user_id = ?", userID)

	if startDate != "" {
		query = query.Where("date >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("date <= ?", endDate)
	}

	err := query.Select(`
		COALESCE(SUM(total_tokens), 0) as total_tokens,
		COALESCE(SUM(cost), 0) as total_cost,
		COALESCE(SUM(prompt_tokens), 0) as total_prompt,
		COALESCE(SUM(completion), 0) as total_completion,
		COUNT(*) as record_count
	`).Scan(&summary).Error

	if err != nil {
		return nil, err
	}

	summary.Breakdown, _ = s.GetUsageStats(ctx, startDate, endDate, "day")
	// Filter by user
	var filtered []UsageStat
	for _, b := range summary.Breakdown {
		if b.UserID == userID {
			filtered = append(filtered, b)
		}
	}
	summary.Breakdown = filtered

	return &summary, nil
}

func (s *TokenUsageService) GetAllUsersUsageSummary(ctx context.Context, startDate, endDate string) ([]UsageSummary, error) {
	results, err := s.GetUsageStats(ctx, startDate, endDate, "day")
	if err != nil {
		return nil, err
	}

	// Group by user
	userMap := make(map[string]*UsageSummary)
	for _, r := range results {
		if _, ok := userMap[r.UserID]; !ok {
			userMap[r.UserID] = &UsageSummary{}
		}
		userMap[r.UserID].TotalTokens += r.TotalTokens
		userMap[r.UserID].TotalCost += r.Cost
		userMap[r.UserID].TotalPrompt += r.PromptTokens
		userMap[r.UserID].TotalCompletion += r.Completion
		userMap[r.UserID].RecordCount++
		userMap[r.UserID].Breakdown = append(userMap[r.UserID].Breakdown, r)
	}

	var summaries []UsageSummary
	for _, v := range userMap {
		summaries = append(summaries, *v)
	}

	return summaries, nil
}

func (s *TokenUsageService) RecordUsage(usage *model.TokenUsage) error {
	return s.db.Create(usage).Error
}

func (s *TokenUsageService) RecordUsageAsync(usage *model.TokenUsage) {
	go func() {
		if err := s.db.Create(usage).Error; err != nil {
			logger.Error("failed to record token usage", "err", err)
		}
	}()
}

// GetStartDateByPeriod returns start date based on period (day, week, month)
func GetStartDateByPeriod(period string) string {
	now := time.Now()
	switch period {
	case "week":
		return now.AddDate(0, 0, -7).Format("2006-01-02")
	case "month":
		return now.AddDate(0, -1, 0).Format("2006-01-02")
	default: // day
		return now.Format("2006-01-02")
	}
}
