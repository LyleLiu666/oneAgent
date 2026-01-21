package main

import (
	"flag"
	"log"
	"path/filepath"
	"runtime"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/liu_y/oneAgent/backend/internal/config"
)

func main() {
	// Parse command line flags
	cmd := flag.String("cmd", "up", "Command to run: up, down, or version")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	// Resolve absolute path to migrations
	// Assumes this binary is run from project root or code location
	// We'll try to find the migrations directory relative to valid source locations
	// Or we could embed them. For now, let's assume standard layout.
	_, b, _, _ := runtime.Caller(0)
	basepath := filepath.Dir(b)
	// cmd/migrate -> ../../internal/database/migrations
	migrationsPath := filepath.Join(basepath, "../../internal/database/migrations")

	// Ensure the path is absolute and uses file:// scheme
	sourceURL := "file://" + migrationsPath

	log.Printf("Using migrations from: %s", sourceURL)

	m, err := migrate.New(sourceURL, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to create migration instance: %v", err)
	}

	switch *cmd {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("Failed to run up migrations: %v", err)
		}
		log.Println("Migrations up completed successfully")
	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("Failed to run down migrations: %v", err)
		}
		log.Println("Migrations down completed successfully")
	case "version":
		v, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("Failed to get version: %v", err)
		}
		log.Printf("Version: %d, Dirty: %v", v, dirty)
	default:
		log.Fatalf("Unknown command: %s", *cmd)
	}
}
