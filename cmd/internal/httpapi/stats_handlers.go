package httpapi

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type YearlyStat struct {
	Year  string `json:"year"`
	Count int    `json:"count"`
}

type CourtStat struct {
	Court string `json:"court"`
	Count int    `json:"count"`
}

type DashboardStats struct {
	TotalJudgments int64  `json:"total_judgments"`
	TotalUsers     *int64 `json:"total_users,omitempty"`
	ActiveUsers    *int64 `json:"active_users,omitempty"`
	PendingUsers   *int64 `json:"pending_users,omitempty"`

	YearlyStats []YearlyStat `json:"yearly_stats"`
	CourtStats  []CourtStat  `json:"court_stats"`
}

func registerStatsRoutes(api *gin.RouterGroup, pool *pgxpool.Pool) {
	api.GET("/dashboard/stats", AuthMiddleware(), func(c *gin.Context) { getDashboardStats(c, pool) })
}

func getDashboardStats(c *gin.Context, pool *pgxpool.Pool) {
	role := c.GetString("userRole")
	userID := c.GetString("userID")

	stats := DashboardStats{
		YearlyStats: []YearlyStat{}, // Ensure non-null for JSON
		CourtStats:  []CourtStat{},
	}

	// Helper to build queries
	var filterClause string
	var args []any
	if role != "admin" {
		filterClause = " AND created_by = $1"
		args = append(args, userID)
	}

	// 1. Count Judgments
	countQ := "SELECT COUNT(*) FROM judgments WHERE 1=1" + filterClause
	err := pool.QueryRow(c, countQ, args...).Scan(&stats.TotalJudgments)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to count judgments"})
		return
	}

	// 2. 5-Year Trend (Sequence Generation)
	// Create map for last 5 years
	yearMap := make(map[string]int)
	currentYear := time.Now().Year()
	years := make([]string, 0)
	for i := 4; i >= 0; i-- {
		y := strconv.Itoa(currentYear - i)
		yearMap[y] = 0
		years = append(years, y)
	}

	// Query actual data
	trendQ := `
		SELECT to_char(judgment_date, 'YYYY') as year, COUNT(*)
		FROM judgments
		WHERE judgment_date >= (CURRENT_DATE - INTERVAL '5 years')
	` + filterClause + `
		GROUP BY year
	`
	rows, err := pool.Query(c, trendQ, args...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var y string
			var count int
			if err := rows.Scan(&y, &count); err == nil {
				if _, ok := yearMap[y]; ok {
					yearMap[y] = count
				}
			}
		}
	}

	// Convert map to sorted slice
	for _, y := range years {
		stats.YearlyStats = append(stats.YearlyStats, YearlyStat{
			Year:  y,
			Count: yearMap[y],
		})
	}

	// 3. Offenders (Parties) Distribution
	// Using 'parties' column, grouped by exact string match
	partiesQ := `
		SELECT COALESCE(parties, 'Unknown'), COUNT(*)
		FROM judgments
		WHERE 1=1 ` + filterClause + `
		GROUP BY parties ORDER BY count DESC LIMIT 5
	`
	rows2, err := pool.Query(c, partiesQ, args...)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var s CourtStat // Reusing CourtStat struct for simplicity/compatibility
			if err := rows2.Scan(&s.Court, &s.Count); err == nil {
				stats.CourtStats = append(stats.CourtStats, s)
			}
		}
	}

	// 4. Admin Stats
	if role == "admin" {
		var total, active, pending int64
		// Total
		if err := pool.QueryRow(c, "SELECT COUNT(*) FROM users").Scan(&total); err == nil {
			stats.TotalUsers = &total
		}
		// Active
		if err := pool.QueryRow(c, "SELECT COUNT(*) FROM users WHERE is_approved = true").Scan(&active); err == nil {
			stats.ActiveUsers = &active
		}
		// Pending
		if err := pool.QueryRow(c, "SELECT COUNT(*) FROM users WHERE is_approved = false").Scan(&pending); err == nil {
			stats.PendingUsers = &pending
		}
	}

	c.JSON(200, stats)
}
