package database

import (
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/model"
)

var DB *gorm.DB

// Connect establishes connection to PostgreSQL database
func Connect(cfg *config.Config) (*gorm.DB, error) {
	var err error

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	// Retry connection with backoff
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		DB, err = gorm.Open(postgres.Open(cfg.DatabaseURL), gormConfig)
		if err == nil {
			break
		}
		log.Printf("Failed to connect to database (attempt %d/%d): %v", i+1, maxRetries, err)
		time.Sleep(time.Duration(i+1) * 2 * time.Second)
	}

	if err != nil {
		return nil, err
	}

	// Configure connection pool
	sqlDB, err := DB.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Println("Database connection established")
	return DB, nil
}

// AutoMigrate runs database migrations
func AutoMigrate() error {
	if DB == nil {
		return nil
	}

	log.Println("Running database migrations...")
	return DB.AutoMigrate(
		&model.User{},
		&model.ChatSession{},
		&model.ChatMessage{},
		&model.LLMProvider{},
		&model.LLMModel{},
		&model.LLMCall{},
		&model.UserSettings{},
	)
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	return DB
}

// HealthCheck verifies database connectivity
func HealthCheck() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Ping()
}
