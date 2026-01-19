package httpapi

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Notification struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	IsRead    bool      `json:"is_read"`
	Link      *string   `json:"link"`
	CreatedAt time.Time `json:"created_at"`
}

func registerNotificationRoutes(api *gin.RouterGroup, pool *pgxpool.Pool) {
	// All routes here should be protected by AuthMiddleware (handled in router.go)
	api.GET("/notifications", func(c *gin.Context) { listNotifications(c, pool) })
	api.PATCH("/notifications/:id/read", func(c *gin.Context) { markNotificationRead(c, pool) })
	api.POST("/notifications/read-all", func(c *gin.Context) { markAllNotificationsRead(c, pool) })
}

func listNotifications(c *gin.Context, pool *pgxpool.Pool) {
	userID := c.GetString("userID")

	rows, err := pool.Query(c, `
		SELECT id, user_id, type, title, message, is_read, link, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 50
	`, userID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	out := make([]Notification, 0)
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Message, &n.IsRead, &n.Link, &n.CreatedAt); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		out = append(out, n)
	}
	c.JSON(200, out)
}

func markNotificationRead(c *gin.Context, pool *pgxpool.Pool) {
	userID := c.GetString("userID")
	id := c.Param("id")

	ct, err := pool.Exec(c, `
		UPDATE notifications
		SET is_read = TRUE
		WHERE id = $1 AND user_id = $2
	`, id, userID)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if ct.RowsAffected() == 0 {
		c.JSON(404, gin.H{"error": "notification not found"})
		return
	}

	c.Status(204)
}

func markAllNotificationsRead(c *gin.Context, pool *pgxpool.Pool) {
	userID := c.GetString("userID")

	_, err := pool.Exec(c, `
		UPDATE notifications
		SET is_read = TRUE
		WHERE user_id = $1 AND is_read = FALSE
	`, userID)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.Status(204)
}

// Helper to create notification (internal use)
func createNotification(ctx context.Context, pool *pgxpool.Pool, userID, nType, title, message string, link *string) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO notifications (user_id, type, title, message, link)
		VALUES ($1, $2, $3, $4, $5)
	`, userID, nType, title, message, link)
	return err
}
