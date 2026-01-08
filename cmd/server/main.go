package main

import (
	"judgment-notes/cmd/internal/db"
	"judgment-notes/cmd/internal/httpapi"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file (using system env instead)")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}

	// ✅ run migrations before creating DB pool / starting server
	db.RunMigrations(dsn)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	pool, err := db.New(dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	r := httpapi.NewRouter(pool)
	log.Printf("API listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
