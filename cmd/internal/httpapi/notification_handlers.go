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
	api.DELETE("/notifications", func(c *gin.Context) { deleteAllNotifications(c, pool) })
	api.DELETE("/notifications/:id", func(c *gin.Context) { deleteNotification(c, pool) })
	api.POST("/notifications/read-all", func(c *gin.Context) { markAllNotificationsRead(c, pool) })
}

func listNotifications(c *gin.Context, pool *pgxpool.Pool) {
	userID := c.MustGet("userID").(string)

	rows, err := pool.Query(context.Background(), `
		SELECT id, user_id, type, title, message, is_read, link, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 20
	`, userID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to list notifications"})
		return
	}
	defer rows.Close()

	var notis []map[string]interface{}
	for rows.Next() {
		var n struct {
			ID        string    `json:"id"`
			UserID    string    `json:"user_id"`
			Type      string    `json:"type"`
			Title     string    `json:"title"`
			Message   string    `json:"message"`
			IsRead    bool      `json:"is_read"`
			Link      *string   `json:"link"`
			CreatedAt time.Time `json:"created_at"`
		}
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Message, &n.IsRead, &n.Link, &n.CreatedAt); err != nil {
			continue
		}
		notis = append(notis, map[string]interface{}{
			"id":         n.ID,
			"user_id":    n.UserID,
			"type":       n.Type,
			"title":      n.Title,
			"message":    n.Message,
			"is_read":    n.IsRead,
			"link":       n.Link,
			"created_at": n.CreatedAt,
		})
	}

	if notis == nil {
		notis = []map[string]interface{}{}
	}

	c.JSON(200, notis)
}

func markNotificationRead(c *gin.Context, pool *pgxpool.Pool) {
	id := c.Param("id")
	userID := c.MustGet("userID").(string)

	_, err := pool.Exec(context.Background(), `
		UPDATE notifications SET is_read = TRUE
		WHERE id = $1 AND user_id = $2
	`, id, userID)

	if err != nil {
		c.JSON(500, gin.H{"error": "failed to mark notification as read"})
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

func deleteNotification(c *gin.Context, pool *pgxpool.Pool) {
	id := c.Param("id")
	userID := c.MustGet("userID").(string)

	_, err := pool.Exec(context.Background(), `
		DELETE FROM notifications
		WHERE id = $1 AND user_id = $2
	`, id, userID)

	if err != nil {
		c.JSON(500, gin.H{"error": "failed to delete notification"})
		return
	}

	c.Status(204)
}

func deleteAllNotifications(c *gin.Context, pool *pgxpool.Pool) {
	userID := c.MustGet("userID").(string)

	_, err := pool.Exec(c.Request.Context(), `
		DELETE FROM notifications
		WHERE user_id = $1
	`, userID)

	if err != nil {
		c.JSON(500, gin.H{"error": "failed to delete all notifications"})
		return
	}

	c.Status(204)
}

// Helper to create notification (internal use)
func createNotification(ctx context.Context, db *pgxpool.Pool, userID, type_, title, message string, link *string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO notifications (user_id, type, title, message, link, is_read)
		VALUES ($1, $2, $3, $4, $5, FALSE)
	`, userID, type_, title, message, link)
	return err
}

// Helper to broadcast notification to all users or specific roles
func broadcastNotification(ctx context.Context, db *pgxpool.Pool, type_, title, message string, link *string) {
	// For this requirement: "Both High-ranking (admin) and Military Officer (user)" means EVERYONE.
	rows, _ := db.Query(ctx, "SELECT id FROM users")
	defer rows.Close()
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err == nil {
			createNotification(ctx, db, uid, type_, title, message, link)
		}
	}
}
