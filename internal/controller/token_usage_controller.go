package controller

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/fisk086/sya/internal/auth"
	"github.com/fisk086/sya/internal/schema"
	"github.com/fisk086/sya/internal/service"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

type TokenUsageController struct {
	usageService *service.TokenUsageService
	jwtCfg       auth.JWTConfig
	getUser      func(userID int64) (*auth.User, error)
}

func NewTokenUsageController(usageService *service.TokenUsageService, jwtCfg auth.JWTConfig, getUser func(userID int64) (*auth.User, error)) *TokenUsageController {
	return &TokenUsageController{
		usageService: usageService,
		jwtCfg:       jwtCfg,
		getUser:      getUser,
	}
}

type UsageStatsResponse struct {
	StartDate string                          `json:"start_date"`
	EndDate   string                          `json:"end_date"`
	Period    string                          `json:"period"`
	Stats     []service.UsageStat             `json:"stats"`
	Summary   map[string]service.UsageSummary `json:"summary"`
}

func (c *TokenUsageController) GetUsageStats(ctx context.Context, hc *app.RequestContext) {
	user := auth.GetCurrentUser(hc)
	if user == nil {
		hc.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}

	// Get query params
	period := hc.DefaultQuery("period", "day")
	startDate := hc.DefaultQuery("start_date", service.GetStartDateByPeriod(period))
	endDate := hc.DefaultQuery("end_date", time.Now().Format("2006-01-02"))

	// Get all usage stats (admin sees all)
	stats, err := c.usageService.GetUsageStats(ctx, startDate, endDate, period)
	if err != nil {
		hc.JSON(http.StatusInternalServerError, schema.ErrorResponse("failed to get usage stats"))
		return
	}

	// Group by user for summary
	summaryMap := make(map[string]service.UsageSummary)
	for _, s := range stats {
		if _, ok := summaryMap[s.UserID]; !ok {
			summaryMap[s.UserID] = service.UsageSummary{
				TotalTokens: 0,
				TotalCost:   0,
				Breakdown:   []service.UsageStat{},
			}
		}
		sum := summaryMap[s.UserID]
		sum.TotalTokens += s.TotalTokens
		sum.TotalCost += s.Cost
		sum.TotalPrompt += s.PromptTokens
		sum.TotalCompletion += s.Completion
		sum.Breakdown = append(sum.Breakdown, s)
		summaryMap[s.UserID] = sum
	}

	// If not admin, filter to only show their own data
	if !user.IsAdmin {
		var filteredStats []service.UsageStat
		userIDStr := strconv.FormatInt(user.ID, 10)
		for _, s := range stats {
			if s.UserID == userIDStr {
				filteredStats = append(filteredStats, s)
			}
		}
		stats = filteredStats
		if summary, ok := summaryMap[userIDStr]; ok {
			summaryMap = map[string]service.UsageSummary{
				userIDStr: summary,
			}
		} else {
			summaryMap = nil
		}
	}

	hc.JSON(http.StatusOK, schema.SuccessResponse(UsageStatsResponse{
		StartDate: startDate,
		EndDate:   endDate,
		Period:    period,
		Stats:     stats,
		Summary:   summaryMap,
	}))
}

func (c *TokenUsageController) RegisterRoutes(r *server.Hertz) {
	protected := r.Group("/api/v1/usage", auth.JWTMiddleware(c.jwtCfg, c.getUser))
	protected.GET("", c.GetUsageStats)
}
