package httpapi

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	AvatarURL *string   `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
}

type loginPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

var jwtSecret = []byte(getEnv("JWT_SECRET", "your-secret-key-change-in-production"))

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func registerAuthRoutes(api *gin.RouterGroup, pool *pgxpool.Pool) {
	api.POST("/auth/login", func(c *gin.Context) { login(c, pool) })
	api.POST("/auth/register", func(c *gin.Context) { register(c, pool) })
	api.GET("/auth/me", AuthMiddleware(), func(c *gin.Context) { getMe(c, pool) })
	api.POST("/auth/logout", func(c *gin.Context) { logout(c) })
}

func login(c *gin.Context, pool *pgxpool.Pool) {
	var in loginPayload
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": "invalid payload"})
		return
	}

	// Find user
	var user User
	var passwordHash string
	err := pool.QueryRow(c, `
		SELECT id, email, name, role, avatar_url, created_at, password_hash 
		FROM users WHERE email = $1
	`, strings.ToLower(strings.TrimSpace(in.Email))).Scan(
		&user.ID, &user.Email, &user.Name, &user.Role, &user.AvatarURL, &user.CreatedAt, &passwordHash,
	)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid email or password"})
		return
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(in.Password)); err != nil {
		c.JSON(401, gin.H{"error": "invalid email or password"})
		return
	}

	// Generate JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"name":  user.Name,
		"role":  user.Role,
		"exp":   time.Now().Add(24 * 7 * time.Hour).Unix(), // 7 days
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(200, gin.H{
		"token": tokenString,
		"user":  user,
	})
}

func register(c *gin.Context, pool *pgxpool.Pool) {
	var in registerPayload
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": "invalid payload"})
		return
	}

	email := strings.ToLower(strings.TrimSpace(in.Email))
	name := strings.TrimSpace(in.Name)

	if email == "" || in.Password == "" || name == "" {
		c.JSON(400, gin.H{"error": "email, password, and name are required"})
		return
	}

	if len(in.Password) < 6 {
		c.JSON(400, gin.H{"error": "password must be at least 6 characters"})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to hash password"})
		return
	}

	// Insert user
	var user User
	err = pool.QueryRow(c, `
    INSERT INTO users (email, password_hash, name, role)
    VALUES ($1, $2, $3, 'user')
    RETURNING id, email, name, role, avatar_url, created_at
`, email, string(hashedPassword), name).Scan(
		&user.ID, &user.Email, &user.Name, &user.Role, &user.AvatarURL, &user.CreatedAt,
	)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			c.JSON(409, gin.H{"error": "email already exists"})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Generate JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"name":  user.Name,
		"role":  user.Role,
		"exp":   time.Now().Add(24 * 7 * time.Hour).Unix(),
	})

	tokenString, _ := token.SignedString(jwtSecret)

	c.JSON(201, gin.H{
		"token": tokenString,
		"user":  user,
	})
}

func getMe(c *gin.Context, pool *pgxpool.Pool) {
	userID := c.GetString("userID")

	var user User
	err := pool.QueryRow(c, `
		SELECT id, email, name, role, avatar_url, created_at 
		FROM users WHERE id = $1
	`, userID).Scan(
		&user.ID, &user.Email, &user.Name, &user.Role, &user.AvatarURL, &user.CreatedAt,
	)
	if err != nil {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}

	c.JSON(200, user)
}

func logout(c *gin.Context) {
	// JWT เป็น stateless ฝั่ง client ลบ token เอง
	c.JSON(200, gin.H{"message": "logged out"})
}

// Auth Middleware
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		tokenString = strings.TrimSpace(tokenString)
		if tokenString == "" {
			c.JSON(401, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// ✅ กันโจมตีเปลี่ยน algorithm
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			c.JSON(401, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(401, gin.H{"error": "invalid token claims"})
			c.Abort()
			return
		}

		// ✅ แปลงเป็น string ให้ชัวร์
		userID := ""
		if v, ok := claims["sub"]; ok {
			userID = strings.TrimSpace(fmt.Sprint(v))
		}
		userEmail := ""
		if v, ok := claims["email"]; ok {
			userEmail = strings.TrimSpace(fmt.Sprint(v))
		}
		userRole := ""
		if v, ok := claims["role"]; ok {
			userRole = strings.TrimSpace(fmt.Sprint(v))
		}

		if userID == "" {
			c.JSON(401, gin.H{"error": "invalid token (no sub)"})
			c.Abort()
			return
		}

		c.Set("userID", userID)
		c.Set("userEmail", userEmail)
		c.Set("userRole", userRole)
		c.Next()
	}
}
