package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// Hardcoded for this specific run, or read from Env via godotenv (but let's just paste it to be safe and quick)
	// User provided details:
	email := "thanawuth.rod@gmail.com"
	username := "After39"
	password := "After#2546"
	role := "owner"

	dbURL := "postgresql://postgres:MrJHmOkEsCrhodjebGfwGUgBqkbPrbDi@hopper.proxy.rlwy.net:56844/railway"

	fmt.Println("Connecting to DB...")
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	fmt.Println("Hashing password...")
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Hotfix: Apply constraint update directly
	fmt.Println("Updating role constraint...")
	_, err = pool.Exec(context.Background(), `
		ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_chk;
		ALTER TABLE users ADD CONSTRAINT users_role_chk CHECK (role IN ('admin', 'user', 'owner'));
	`)
	if err != nil {
		log.Fatalf("Failed to update constraint: %v", err)
	}

	fmt.Println("Upserting user...")
	q := `
		INSERT INTO users (email, password_hash, name, role, is_approved)
		VALUES ($1, $2, $3, $4, TRUE)
		ON CONFLICT (email) DO UPDATE 
		SET role = $4, password_hash = $2, name = $3, is_approved = TRUE, updated_at = now()
		RETURNING id, email, role
	`

	var id, retEmail, retRole string
	err = pool.QueryRow(context.Background(), q, email, string(hashed), username, role).Scan(&id, &retEmail, &retRole)
	if err != nil {
		log.Fatalf("Failed to upsert user: %v", err)
	}

	fmt.Printf("Success! User ID: %s, Email: %s, Role: %s\n", id, retEmail, retRole)
}
