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

type ContributorStat struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type DashboardStats struct {
	TotalJudgments int64  `json:"total_judgments"`
	TotalUsers     *int64 `json:"total_users,omitempty"`
	ActiveUsers    *int64 `json:"active_users,omitempty"`
	PendingUsers   *int64 `json:"pending_users,omitempty"`

	PendingJudgments *int64 `json:"pending_judgments,omitempty"`

	YearlyStats      []YearlyStat      `json:"yearly_stats"`
	ContributorStats []ContributorStat `json:"contributor_stats,omitempty"`
	CourtStats       []CourtStat       `json:"court_stats,omitempty"`
}

func registerStatsRoutes(api *gin.RouterGroup, pool *pgxpool.Pool) {
	api.GET("/dashboard/stats", AuthMiddleware(pool), func(c *gin.Context) { getDashboardStats(c, pool) })
}

func getDashboardStats(c *gin.Context, pool *pgxpool.Pool) {
	role := c.GetString("userRole")
	userID := c.GetString("userID")

	stats := DashboardStats{
		YearlyStats: []YearlyStat{}, // Ensure non-null for JSON
	}

	// Helper to build queries
	var filterClause string
	var args []any
	if role != "admin" && role != "owner" {
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
		for rows.Next() {
			var y string
			var count int
			if err := rows.Scan(&y, &count); err == nil {
				if _, ok := yearMap[y]; ok {
					yearMap[y] = count
				}
			}
		}
		rows.Close() // Explicit close to free connection
	}

	// Convert map to sorted slice
	for _, y := range years {
		stats.YearlyStats = append(stats.YearlyStats, YearlyStat{
			Year:  y,
			Count: yearMap[y],
		})
	}

	// 3. Conditional Stats
	if role == "admin" || role == "owner" {
		// Admin: Top 5 Contributors (Current Year)
		stats.ContributorStats = []ContributorStat{}

		contribArgs := make([]any, 0)
		// For admins, no base filterClause on created_by, so we query all.
		// Filter by current year judgment_date (as decided previously).

		contribQ := `
			SELECT u.name, COUNT(j.id)
			FROM judgments j
			JOIN users u ON j.created_by = u.id
			WHERE to_char(j.judgment_date, 'YYYY') = $1
			GROUP BY u.name
			ORDER BY count DESC
			LIMIT 5
		`
		contribArgs = append(contribArgs, strconv.Itoa(currentYear))

		rows2, err := pool.Query(c, contribQ, contribArgs...)
		if err == nil {
			defer rows2.Close()
			for rows2.Next() {
				var s ContributorStat
				if err := rows2.Scan(&s.Name, &s.Count); err == nil {
					stats.ContributorStats = append(stats.ContributorStats, s)
				}
			}
		}
	} else {
		// User: Top Offenders (Parties) - based on their judgments
		stats.CourtStats = []CourtStat{}

		// Handle empty string as Unknown, Alias count
		// Use explicit aliases to avoid AMBIGUOUS column reference in ORDER BY
		partiesQ := `
			SELECT COALESCE(NULLIF(parties, ''), 'Unknown') as party, COUNT(*) as cnt
			FROM judgments
			WHERE 1=1 ` + filterClause + `
			GROUP BY party ORDER BY cnt DESC LIMIT 5
		`
		// reuse 'args' which already has userID if needed
		rows2, err := pool.Query(c, partiesQ, args...)
		if err == nil {
			defer rows2.Close()
			for rows2.Next() {
				var s CourtStat
				if err := rows2.Scan(&s.Court, &s.Count); err == nil {
					stats.CourtStats = append(stats.CourtStats, s)
				}
			}
		} else {
			// Log error for debugging if needed (or just ignore as per pattern)
			// c.Error(err)
		}
	}

	// 4. Admin Stats
	if role == "admin" || role == "owner" {
		var total, active, pending int64
		// Total
		if err := pool.QueryRow(c, "SELECT COUNT(*) FROM users").Scan(&total); err == nil {
			stats.TotalUsers = &total
		}
		// Active
		if err := pool.QueryRow(c, "SELECT COUNT(*) FROM users WHERE is_approved = true").Scan(&active); err == nil {
			stats.ActiveUsers = &active
		}
		// Pending Users
		if err := pool.QueryRow(c, "SELECT COUNT(*) FROM users WHERE is_approved = false").Scan(&pending); err == nil {
			stats.PendingUsers = &pending
		}
		// Pending Judgments (pending + request_delete)
		var pendingJudgments int64
		if err := pool.QueryRow(c, "SELECT COUNT(*) FROM judgments WHERE status IN ('pending', 'request_delete')").Scan(&pendingJudgments); err == nil {
			stats.PendingJudgments = &pendingJudgments
		}
	}

	c.JSON(200, stats)
}
