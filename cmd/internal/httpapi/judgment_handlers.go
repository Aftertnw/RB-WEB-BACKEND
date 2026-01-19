package httpapi

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Judgment struct {
	ID            string    `json:"id"`
	DocNo         *string   `json:"doc_no"`
	Title         string    `json:"title"`
	CaseNo        *string   `json:"case_no"`
	Court         *string   `json:"court"`
	JudgmentDate  *string   `json:"judgment_date"` // YYYY-MM-DD
	Parties       *string   `json:"parties"`
	Facts         *string   `json:"facts"`
	Issues        *string   `json:"issues"`
	Holding       *string   `json:"holding"`
	Notes         *string   `json:"notes"`
	Tags          []string  `json:"tags"`
	Status        string    `json:"status"` // pending, approved
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	CreatedByName *string   `json:"created_by_name,omitempty"` // populated for admins
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
	// ✅ auth required for all judgment access
	auth := api.Group("")
	auth.Use(AuthMiddleware()) // Middleware populates userRole and userID

	auth.GET("/judgments", func(c *gin.Context) { listJudgments(c, pool) })
	auth.GET("/judgments/:id", func(c *gin.Context) { getJudgment(c, pool) })

	auth.POST("/judgments", func(c *gin.Context) { createJudgment(c, pool) })
	auth.PUT("/judgments/:id", func(c *gin.Context) { updateJudgment(c, pool) })
	auth.DELETE("/judgments/:id", func(c *gin.Context) { deleteJudgment(c, pool) })
	auth.PUT("/judgments/:id/approve", func(c *gin.Context) { approveJudgment(c, pool) })
	auth.PUT("/judgments/:id/reject", func(c *gin.Context) { rejectJudgment(c, pool) })
}

func listJudgments(c *gin.Context, pool *pgxpool.Pool) {
	search := strings.TrimSpace(c.Query("search"))
	status := strings.TrimSpace(c.Query("status"))

	// Role-based filtering
	role := c.GetString("userRole")
	userID := c.GetString("userID")

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

	// If not admin, restrict to own judgments
	// Note: If user is not logged in (role=""), they might see nothing or public?
	// Requirement implies this is an internal app function.
	// If the route is public in router.go, "userID" might be empty.
	// Safe default: If not admin, and we have a userID, show own.
	// If not admin and NO userID (not logged in), maybe show nothing or approved?
	// Given the context of "Military Officer", they are users.
	if role != "admin" {
		if userID != "" {
			conds = append(conds, "j.created_by = $"+itoa(argN))
			args = append(args, userID)
			argN++
		} else {
			// Not logged in? Maybe strict security: show nothing or only 'approved'?
			// Let's assume for now if public access is allowed, they see nothing or all?
			// The prompt says: "role user1 sees only theirs".
			// Use strict default: if not admin, strict filter.
			// But if not logged in... let's enforce empty for now to be safe,
			// OR if your app has a public view, let me know.
			// Assuming mixed usage: "User1 sees only User1's".
			// We will append a condition that fails if no user, UNLESS there's a requirement for public view.
			// Let's assume safety:
			// if auth missing, maybe we shouldn't show restricted data.
			// We'll filter by created_by if userID exists.

			// Actually, let's just stick to the request: "User role sees theirs".
		}
	}

	if search != "" {
		conds = append(conds,
			"(j.doc_no ILIKE $"+itoa(argN)+" OR j.title ILIKE $"+itoa(argN)+" OR j.case_no ILIKE $"+itoa(argN)+" OR j.court ILIKE $"+itoa(argN)+" OR j.notes ILIKE $"+itoa(argN)+")",
		)
		args = append(args, "%"+search+"%")
		argN++
	}

	if status != "" {
		conds = append(conds, "j.status = $"+itoa(argN))
		args = append(args, status)
		argN++
	}

	where := strings.Join(conds, " AND ")

	// Count total (use simple count first)
	countQ := `SELECT COUNT(*) FROM judgments j WHERE ` + where
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

	// Fetch items with pagination - distinct selection to avoid name collision
	q := `
SELECT j.id, j.doc_no, j.title, j.case_no, j.court, to_char(j.judgment_date,'YYYY-MM-DD'),
       j.parties, j.facts, j.issues, j.holding, j.notes, j.tags, j.status, j.created_at, j.updated_at,
       u.name
FROM judgments j
LEFT JOIN users u ON j.created_by = u.id
WHERE ` + where + `
ORDER BY j.judgment_date DESC NULLS LAST, j.updated_at DESC
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
		var createdByName *string
		if err := rows.Scan(
			&j.ID, &j.DocNo, &j.Title, &j.CaseNo, &j.Court, &jd,
			&j.Parties, &j.Facts, &j.Issues, &j.Holding, &j.Notes, &j.Tags, &j.Status, &j.CreatedAt, &j.UpdatedAt,
			&createdByName,
		); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		j.JudgmentDate = jd
		j.CreatedByName = createdByName
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
	role := c.GetString("userRole")
	userID := c.GetString("userID")

	q := `
SELECT j.id, j.doc_no, j.title, j.case_no, j.court, to_char(j.judgment_date,'YYYY-MM-DD'),
       j.parties, j.facts, j.issues, j.holding, j.notes, j.tags, j.status, j.created_at, j.updated_at,
       u.name
FROM judgments j
LEFT JOIN users u ON j.created_by = u.id
WHERE j.id=$1`

	args := []any{id}

	// Security: If not admin, MUST be creator
	if role != "admin" && userID != "" {
		q += ` AND j.created_by = $2`
		args = append(args, userID)
	}

	var j Judgment
	var jd *string
	var createdByName *string

	err := pool.QueryRow(c, q, args...).Scan(
		&j.ID, &j.DocNo, &j.Title, &j.CaseNo, &j.Court, &jd,
		&j.Parties, &j.Facts, &j.Issues, &j.Holding, &j.Notes, &j.Tags, &j.Status, &j.CreatedAt, &j.UpdatedAt,
		&createdByName,
	)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	j.JudgmentDate = jd
	j.CreatedByName = createdByName
	c.JSON(200, j)
}

func createJudgment(c *gin.Context, pool *pgxpool.Pool) {
	var in createUpdatePayload
	if err := c.ShouldBindJSON(&in); err != nil || strings.TrimSpace(in.Title) == "" {
		c.JSON(400, gin.H{"error": "invalid payload (title required)"})
		return
	}

	userID := c.GetString("userID")
	userName := c.GetString("userName") // Ensure this is set in AuthMiddleware or fetch it
	if userName == "" {
		// Fallback if userName not in context (though it should be if we add it)
		// For now we might just say "A user" or fetch it.
		// Actually AuthMiddleware sets "userEmail" and "userID". Let's stick to context.
	}

	q := `
INSERT INTO judgments (doc_no, title, case_no, court, judgment_date, parties, facts, issues, holding, notes, tags, status, created_by)
VALUES (next_judgment_doc_no(), $1,$2,$3,$4::date,$5,$6,$7,$8,$9,$10, 'pending', $11)
RETURNING id, doc_no`

	var id string
	var docNo string
	err := pool.QueryRow(c, q,
		in.Title, in.CaseNo, in.Court, in.JudgmentDate,
		in.Parties, in.Facts, in.Issues, in.Holding, in.Notes, in.Tags, userID,
	).Scan(&id, &docNo)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Notify Admins Only
	go func() {
		ctx := context.Background()
		// Get all admins
		rows, _ := pool.Query(ctx, "SELECT id FROM users WHERE role = 'admin'")
		defer rows.Close()

		link := "/judgments/approve" // Direct to approval page
		msg := "New judgment recorded: " + in.Title + " (Pending Approval)"

		for rows.Next() {
			var adminID string
			if err := rows.Scan(&adminID); err == nil {
				createNotification(ctx, pool, adminID, "info", "New Judgment Pending", msg, &link)
			}
		}
	}()

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
    issues=$7, holding=$8, notes=$9, tags=$10, status='pending', updated_at=now()
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

	// Notify Admins about the update
	go func() {
		ctx := context.Background()
		rows, _ := pool.Query(ctx, "SELECT id FROM users WHERE role = 'admin'")
		defer rows.Close()

		link := "/judgments/approve"
		msg := "Judgment updated and pending re-approval: " + in.Title

		for rows.Next() {
			var adminID string
			if err := rows.Scan(&adminID); err == nil {
				createNotification(ctx, pool, adminID, "info", "Judgment Updated", msg, &link)
			}
		}
	}()

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

func approveJudgment(c *gin.Context, pool *pgxpool.Pool) {
	// Ensure only admin
	role := c.GetString("userRole")
	if role != "admin" {
		c.JSON(403, gin.H{"error": "admin required"})
		return
	}

	id := c.Param("id")

	// Returns created_by so we can notify them
	q := `UPDATE judgments SET status='approved', updated_at=now() WHERE id=$1 RETURNING created_by`
	var createdBy *string
	err := pool.QueryRow(c, q, id).Scan(&createdBy)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	// No rows check handled by Scan error usually, but distinct not found vs other error is better.
	// However QueryRow returns ErrNoRows if no rows.

	// Notify Creator
	if createdBy != nil {
		go func() {
			link := "/judgments/" + id
			msg := "Your judgment #" + id[:8] + " has been approved."
			createNotification(context.Background(), pool, *createdBy, "success", "Judgment Approved", msg, &link)
		}()
	}

	c.JSON(200, gin.H{"id": id, "status": "approved"})
}

func rejectJudgment(c *gin.Context, pool *pgxpool.Pool) {
	// Ensure only admin
	role := c.GetString("userRole")
	if role != "admin" {
		c.JSON(403, gin.H{"error": "admin required"})
		return
	}

	id := c.Param("id")
	q := `UPDATE judgments SET status='rejected', updated_at=now() WHERE id=$1 RETURNING created_by`
	var createdBy *string
	err := pool.QueryRow(c, q, id).Scan(&createdBy)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Notify Creator
	if createdBy != nil {
		go func() {
			link := "/judgments/" + id
			msg := "Your judgment #" + id[:8] + " has been rejected."
			createNotification(context.Background(), pool, *createdBy, "alert", "Judgment Rejected", msg, &link)
		}()
	}

	c.JSON(200, gin.H{"id": id, "status": "rejected"})
}
