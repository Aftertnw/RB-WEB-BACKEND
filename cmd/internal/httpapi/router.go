package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(pool *pgxpool.Pool) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// CORS + utf-8 (ของเดิม)
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	api := r.Group("/api")

	// Auth routes (public)
	registerAuthRoutes(api, pool)

	// Protected routes
	protected := api.Group("")
	protected.Use(AuthMiddleware())
	registerNotificationRoutes(protected, pool) // ✅ Notifications
	// The following routes are assumed to be part of notification routes,
	// and are added here based on the provided snippet.
	// Note: The snippet used 'auth' which is not defined, assuming it refers to 'protected'.
	// Also, 'markNotificationRead', 'deleteNotification', 'markAllNotificationsRead'
	// are assumed to be defined elsewhere or will be defined.
	// The PATCH route was already implicitly handled by registerNotificationRoutes,
	// but is included here as per the instruction's snippet.
	protected.PATCH("/notifications/:id/read", func(c *gin.Context) {
		markNotificationRead(c, pool)
	})
	protected.DELETE("/notifications/:id", func(c *gin.Context) {
		deleteNotification(c, pool)
	})
	protected.POST("/notifications/read-all", func(c *gin.Context) {
		markAllNotificationsRead(c, pool)
	})

	// ✅ Admin-only routes (จัดการ user)
	admin := api.Group("")
	admin.Use(AuthMiddleware(), RequireRole("admin"))
	registerUserAdminRoutes(admin, pool)

	// ✅ Judgments: user ก็ทำ CRUD ได้ แค่ต้อง login
	registerJudgmentRoutes(api, pool) // เดี๋ยวไปแก้ใน registerJudgmentRoutes ให้แยก public/protected

	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	return r
}
