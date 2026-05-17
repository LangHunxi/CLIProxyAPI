package management

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/usagerecord"
)

// DashboardStats represents the unified dashboard statistics response.
type DashboardStats struct {
	// Overview stats (for stat cards)
	Overview OverviewStats `json:"overview"`
	// System health metrics
	SystemHealth SystemHealthStats `json:"system_health"`
	// Daily statistics for the last N days
	DailyStats []DailyStatsItem `json:"daily_stats"`
	// Request trend for charts
	RequestTrend []TrendPoint `json:"request_trend"`
	// Model usage counts (aggregated for the period)
	ModelCounts []ModelCount `json:"model_counts"`
}

// OverviewStats contains the main stat card data.
type OverviewStats struct {
	TotalRequests   int64 `json:"total_requests"`
	SuccessRequests int64 `json:"success_requests"`
	FailureRequests int64 `json:"failure_requests"`
	TotalTokens     int64 `json:"total_tokens"`
	InputTokens     int64 `json:"input_tokens"`
	OutputTokens    int64 `json:"output_tokens"`
}

// SystemHealthStats contains system health metrics.
type SystemHealthStats struct {
	AvgResponseTime float64 `json:"avg_response_time"` // in seconds
	ErrorRate       float64 `json:"error_rate"`        // percentage
	UniqueModels    int64   `json:"unique_models"`
	UniqueProviders int64   `json:"unique_providers"`
}

// DailyStatsItem represents daily statistics for the table.
type DailyStatsItem struct {
	Date            string  `json:"date"`
	Requests        int64   `json:"requests"`
	Tokens          int64   `json:"tokens"`
	AvgResponseTime float64 `json:"avg_response_time"` // in seconds
	UniqueModels    int64   `json:"unique_models"`
}

// TrendPoint represents a single point in the request trend chart.
type TrendPoint struct {
	Date     string `json:"date"`
	Requests int64  `json:"requests"`
}

// ModelCount represents usage count for a single model.
type ModelCount struct {
	Model    string `json:"model"`
	Requests int64  `json:"requests"`
}

// GetDashboardStats returns unified dashboard statistics.
// GET /management/dashboard/stats?days=7
func (h *Handler) GetDashboardStats(c *gin.Context) {
	store := usagerecord.DefaultStore()
	if store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "usage records not available"})
		return
	}

	// Default to 7 days
	days := 7
	if daysStr := c.Query("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 && d <= 30 {
			days = d
		}
	}

	now := time.Now()
	startTime := now.AddDate(0, 0, -days).Format("2006-01-02") + " 00:00:00"
	endTime := now.Format("2006-01-02") + " 23:59:59"

	// Get usage summary for overview stats
	summary, err := store.GetUsageSummary(c.Request.Context(), startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get model stats for model counts
	modelStats, err := store.GetModelStats(c.Request.Context(), startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get activity heatmap for daily stats
	heatmap, err := store.GetActivityHeatmap(c.Request.Context(), days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Build overview stats
	overview := OverviewStats{
		TotalRequests:   summary.TotalRequests,
		SuccessRequests: summary.SuccessRequests,
		FailureRequests: summary.FailureRequests,
		TotalTokens:     summary.TotalTokens,
		InputTokens:     summary.InputTokens,
		OutputTokens:    summary.OutputTokens,
	}

	// Build system health stats
	errorRate := 0.0
	if summary.TotalRequests > 0 {
		errorRate = float64(summary.FailureRequests) / float64(summary.TotalRequests) * 100
	}
	systemHealth := SystemHealthStats{
		AvgResponseTime: summary.AvgDuration / 1000, // convert ms to seconds
		ErrorRate:       errorRate,
		UniqueModels:    summary.UniqueModels,
		UniqueProviders: summary.UniqueProviders,
	}

	// Build daily stats from heatmap (now includes per-day avg_duration and unique_models)
	dailyStats := make([]DailyStatsItem, 0, len(heatmap.Days))
	requestTrend := make([]TrendPoint, 0, len(heatmap.Days))

	for _, day := range heatmap.Days {
		dailyStats = append(dailyStats, DailyStatsItem{
			Date:            day.Date,
			Requests:        day.Requests,
			Tokens:          day.TotalTokens,
			AvgResponseTime: day.AvgDuration / 1000, // convert ms to seconds
			UniqueModels:    day.UniqueModels,
		})
		requestTrend = append(requestTrend, TrendPoint{
			Date:     day.Date,
			Requests: day.Requests,
		})
	}

	// Build model counts from model stats
	// Aggregate by model name (model stats groups by model+provider, so we need to sum)
	modelCountMap := make(map[string]int64)
	if modelStats != nil {
		for _, m := range modelStats.Models {
			modelCountMap[m.Model] += m.RequestCount
		}
	}
	modelCounts := make([]ModelCount, 0, len(modelCountMap))
	for model, requests := range modelCountMap {
		modelCounts = append(modelCounts, ModelCount{
			Model:    model,
			Requests: requests,
		})
	}

	response := DashboardStats{
		Overview:     overview,
		SystemHealth: systemHealth,
		DailyStats:   dailyStats,
		RequestTrend: requestTrend,
		ModelCounts:  modelCounts,
	}

	c.JSON(http.StatusOK, response)
}
