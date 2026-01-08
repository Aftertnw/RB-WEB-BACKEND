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
