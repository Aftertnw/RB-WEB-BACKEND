package main

import (
	"fmt"
	"judgment-notes/cmd/internal/db"
	"judgment-notes/cmd/internal/httpapi"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}

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

	password := "admin123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	fmt.Println(string(hash))
}
