package httpapi

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type AdminCreateUserPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Role     string `json:"role"` // admin/user
}

type AdminUpdateUserPayload struct {
	Email    *string `json:"email"` // ✅ เพิ่ม
	Name     *string `json:"name"`
	Role     *string `json:"role"`     // admin/user
	Password *string `json:"password"` // optional reset
}

type AdminUser struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	AvatarURL *string   `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
}

func registerUserAdminRoutes(api *gin.RouterGroup, pool *pgxpool.Pool) {
	api.GET("/users", func(c *gin.Context) { adminListUsers(c, pool) })
	api.GET("/users/:id", func(c *gin.Context) { adminGetUser(c, pool) })
	api.POST("/users", func(c *gin.Context) { adminCreateUser(c, pool) })
	api.PATCH("/users/:id", func(c *gin.Context) { adminUpdateUser(c, pool) })
	api.DELETE("/users/:id", func(c *gin.Context) { adminDeleteUser(c, pool) })
}

func normalizeRole(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
func isValidRole(r string) bool {
	r = normalizeRole(r)
	return r == "admin" || r == "user"
}

func adminListUsers(c *gin.Context, pool *pgxpool.Pool) {
	rows, err := pool.Query(c, `
		SELECT id, email, name, role, avatar_url, created_at
		FROM users
		ORDER BY created_at DESC
	`)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	out := make([]AdminUser, 0)
	for rows.Next() {
		var u AdminUser
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.AvatarURL, &u.CreatedAt); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		out = append(out, u)
	}
	c.JSON(200, out)
}

func adminGetUser(c *gin.Context, pool *pgxpool.Pool) {
	id := c.Param("id")

	var u AdminUser
	err := pool.QueryRow(c, `
		SELECT id, email, name, role, avatar_url, created_at
		FROM users
		WHERE id=$1
	`, id).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.AvatarURL, &u.CreatedAt)

	if err != nil {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}
	c.JSON(200, u)
}

func adminCreateUser(c *gin.Context, pool *pgxpool.Pool) {
	var in AdminCreateUserPayload
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": "invalid payload"})
		return
	}

	email := strings.ToLower(strings.TrimSpace(in.Email))
	name := strings.TrimSpace(in.Name)
	role := normalizeRole(in.Role)

	if email == "" || name == "" || in.Password == "" {
		c.JSON(400, gin.H{"error": "email, password, and name are required"})
		return
	}
	if len(in.Password) < 6 {
		c.JSON(400, gin.H{"error": "password must be at least 6 characters"})
		return
	}
	if role == "" {
		role = "user"
	}
	if !isValidRole(role) {
		c.JSON(400, gin.H{"error": "invalid role"})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to hash password"})
		return
	}

	var u AdminUser
	err = pool.QueryRow(c, `
		INSERT INTO users (email, password_hash, name, role)
		VALUES ($1,$2,$3,$4)
		RETURNING id, email, name, role, avatar_url, created_at
	`, email, string(hashed), name, role).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.AvatarURL, &u.CreatedAt)

	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			c.JSON(409, gin.H{"error": "email already exists"})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, u)
}

func adminUpdateUser(c *gin.Context, pool *pgxpool.Pool) {
	id := c.Param("id")
	var in AdminUpdateUserPayload
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": "invalid payload"})
		return
	}

	selfID := c.GetString("userID")

	setParts := []string{}
	args := []any{}
	argN := 1

	// ✅ email
	if in.Email != nil {
		email := strings.ToLower(strings.TrimSpace(*in.Email))
		if email == "" || !strings.Contains(email, "@") {
			c.JSON(400, gin.H{"error": "invalid email"})
			return
		}
		setParts = append(setParts, "email=$"+itoa(argN))
		args = append(args, email)
		argN++
	}

	// ✅ name
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			c.JSON(400, gin.H{"error": "name cannot be empty"})
			return
		}
		setParts = append(setParts, "name=$"+itoa(argN))
		args = append(args, name)
		argN++
	}

	// ✅ role
	if in.Role != nil {
		role := normalizeRole(*in.Role)
		if !isValidRole(role) {
			c.JSON(400, gin.H{"error": "invalid role"})
			return
		}
		if id == selfID && role != "admin" {
			c.JSON(400, gin.H{"error": "cannot downgrade your own role"})
			return
		}
		setParts = append(setParts, "role=$"+itoa(argN))
		args = append(args, role)
		argN++
	}

	// ✅ password reset
	if in.Password != nil {
		if len(*in.Password) < 6 {
			c.JSON(400, gin.H{"error": "password must be at least 6 characters"})
			return
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(*in.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to hash password"})
			return
		}
		setParts = append(setParts, "password_hash=$"+itoa(argN))
		args = append(args, string(hashed))
		argN++
	}

	if len(setParts) == 0 {
		c.Status(204)
		return
	}

	q := `UPDATE users SET ` + strings.Join(setParts, ", ") + ` WHERE id=$` + itoa(argN)
	args = append(args, id)

	ct, err := pool.Exec(c, q, args...)
	if err != nil {
		// ✅ ถ้าอีเมลซ้ำ (unique) จะเข้ามาตรงนี้
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			c.JSON(409, gin.H{"error": "email already exists"})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if ct.RowsAffected() == 0 {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}

	c.Status(204)
}

func adminDeleteUser(c *gin.Context, pool *pgxpool.Pool) {
	id := c.Param("id")
	selfID := c.GetString("userID")
	if id == selfID {
		c.JSON(400, gin.H{"error": "cannot delete your own account"})
		return
	}

	ct, err := pool.Exec(c, `DELETE FROM users WHERE id=$1`, id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if ct.RowsAffected() == 0 {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}
	c.Status(204)
}
