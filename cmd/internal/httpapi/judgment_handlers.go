package httpapi

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Judgment struct {
	ID           string    `json:"id"`
	DocNo        *string   `json:"doc_no"`
	Title        string    `json:"title"`
	CaseNo       *string   `json:"case_no"`
	Court        *string   `json:"court"`
	JudgmentDate *string   `json:"judgment_date"` // YYYY-MM-DD
	Parties      *string   `json:"	parties"`
	Facts        *string   `json:"facts"`
	Issues       *string   `json:"issues"`
	Holding      *string   `json:"holding"`
	Notes        *string   `json:"notes"`
	Tags         []string  `json:"tags"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type createUpdatePayload struct {
	Title        string   `json:"title"`
	CaseNo       *string  `json:"case_no"`
	Court        *string  `json:"court"`
	JudgmentDate *string  `json:"judgment_date"`
	Parties      *string  `json:"parties"`
	Facts        *string  `json:"facts"`
	Issues       *string  `json:"issues"`
	Holding      *string  `json:"holding"`
	Notes        *string  `json:"notes"`
	Tags         []string `json:"tags"`
}

// Paginated response
type PaginatedResponse struct {
	Items      []Judgment `json:"items"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	Limit      int        `json:"limit"`
	TotalPages int        `json:"totalPages"`
}

func registerJudgmentRoutes(api *gin.RouterGroup, pool *pgxpool.Pool) {
	// public read
	api.GET("/judgments", func(c *gin.Context) { listJudgments(c, pool) })
	api.GET("/judgments/:id", func(c *gin.Context) { getJudgment(c, pool) })

	// ✅ auth write (user ก็ทำ CRUD ได้ แค่ต้อง login)
	auth := api.Group("")
	auth.Use(AuthMiddleware())
	auth.POST("/judgments", func(c *gin.Context) { createJudgment(c, pool) })
	auth.PUT("/judgments/:id", func(c *gin.Context) { updateJudgment(c, pool) })
	auth.DELETE("/judgments/:id", func(c *gin.Context) { deleteJudgment(c, pool) })
}

func listJudgments(c *gin.Context, pool *pgxpool.Pool) {
	search := strings.TrimSpace(c.Query("search"))

	// Pagination params
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	// Validate
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100 // max limit
	}

	offset := (page - 1) * limit

	// Build WHERE clause
	conds := []string{"1=1"}
	args := []any{}
	argN := 1

	if search != "" {
		conds = append(conds,
			"(doc_no ILIKE $"+itoa(argN)+" OR title ILIKE $"+itoa(argN)+" OR case_no ILIKE $"+itoa(argN)+" OR court ILIKE $"+itoa(argN)+" OR notes ILIKE $"+itoa(argN)+")",
		)
		args = append(args, "%"+search+"%")
		argN++
	}

	where := strings.Join(conds, " AND ")

	// Count total
	countQ := `SELECT COUNT(*) FROM judgments WHERE ` + where
	var total int
	if err := pool.QueryRow(c, countQ, args...).Scan(&total); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	// Fetch items with pagination
	q := `
SELECT id, doc_no, title, case_no, court, to_char(judgment_date,'YYYY-MM-DD'),
       parties, facts, issues, holding, notes, tags, created_at, updated_at
FROM judgments
WHERE ` + where + `
ORDER BY judgment_date DESC NULLS LAST, updated_at DESC
LIMIT $` + itoa(argN) + ` OFFSET $` + itoa(argN+1)

	args = append(args, limit, offset)

	rows, err := pool.Query(c, q, args...)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	items := make([]Judgment, 0)
	for rows.Next() {
		var j Judgment
		var jd *string
		if err := rows.Scan(
			&j.ID, &j.DocNo, &j.Title, &j.CaseNo, &j.Court, &jd,
			&j.Parties, &j.Facts, &j.Issues, &j.Holding, &j.Notes, &j.Tags, &j.CreatedAt, &j.UpdatedAt,
		); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		j.JudgmentDate = jd
		items = append(items, j)
	}

	c.JSON(200, PaginatedResponse{
		Items:      items,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	})
}

func getJudgment(c *gin.Context, pool *pgxpool.Pool) {
	id := c.Param("id")

	q := `
SELECT id, doc_no, title, case_no, court, to_char(judgment_date,'YYYY-MM-DD'),
       parties, facts, issues, holding, notes, tags, created_at, updated_at
FROM judgments
WHERE id=$1`

	var j Judgment
	var jd *string
	err := pool.QueryRow(c, q, id).Scan(
		&j.ID, &j.DocNo, &j.Title, &j.CaseNo, &j.Court, &jd,
		&j.Parties, &j.Facts, &j.Issues, &j.Holding, &j.Notes, &j.Tags, &j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	j.JudgmentDate = jd
	c.JSON(200, j)
}

func createJudgment(c *gin.Context, pool *pgxpool.Pool) {
	var in createUpdatePayload
	if err := c.ShouldBindJSON(&in); err != nil || strings.TrimSpace(in.Title) == "" {
		c.JSON(400, gin.H{"error": "invalid payload (title required)"})
		return
	}

	q := `
INSERT INTO judgments (doc_no, title, case_no, court, judgment_date, parties, facts, issues, holding, notes, tags)
VALUES (next_judgment_doc_no(), $1,$2,$3,$4::date,$5,$6,$7,$8,$9,$10)
RETURNING id, doc_no`

	var id string
	var docNo string
	err := pool.QueryRow(c, q,
		in.Title, in.CaseNo, in.Court, in.JudgmentDate,
		in.Parties, in.Facts, in.Issues, in.Holding, in.Notes, in.Tags,
	).Scan(&id, &docNo)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, gin.H{"id": id, "doc_no": docNo})
}

func updateJudgment(c *gin.Context, pool *pgxpool.Pool) {
	id := c.Param("id")

	var in createUpdatePayload
	if err := c.ShouldBindJSON(&in); err != nil || strings.TrimSpace(in.Title) == "" {
		c.JSON(400, gin.H{"error": "invalid payload (title required)"})
		return
	}

	q := `
UPDATE judgments
SET title=$1, case_no=$2, court=$3, judgment_date=$4::date, parties=$5, facts=$6,
    issues=$7, holding=$8, notes=$9, tags=$10, updated_at=now()
WHERE id=$11`

	ct, err := pool.Exec(c, q,
		in.Title, in.CaseNo, in.Court, in.JudgmentDate,
		in.Parties, in.Facts, in.Issues, in.Holding, in.Notes, in.Tags, id,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if ct.RowsAffected() == 0 {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	c.Status(204)
}

func deleteJudgment(c *gin.Context, pool *pgxpool.Pool) {
	id := c.Param("id")

	ct, err := pool.Exec(c, `DELETE FROM judgments WHERE id=$1`, id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if ct.RowsAffected() == 0 {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}

	c.Status(204)
}
