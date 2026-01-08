package db

import (
	"errors"
	"log"
	"runtime"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(dsn string) {
	source := "file://migrations" // default: รันจาก root ของโปรเจกต์

	// ถ้ารันใน container (linux) และคุณ copy migrations ไป /app/migrations
	if runtime.GOOS == "linux" {
		// ให้ชัวร์สุดใน Railway container
		source = "file:///app/migrations"
	}

	m, err := migrate.New(source, dsn)
	if err != nil {
		log.Fatal(err)
	}

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Println("migrations: no change")
			return
		}
		log.Fatal(err)
	}

	log.Println("migrations: applied successfully")
}
