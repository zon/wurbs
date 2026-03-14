package core

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const defaultDatabase = "chat"
const sqliteFile = defaultDatabase + ".db"

var DB *gorm.DB

func InitDB() error {
	cfg, err := LoadPostgresConfig("config")
	if err == nil {
		dsn := fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database,
		)
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			return fmt.Errorf("failed to connect to PostgreSQL: %w", err)
		}
		DB = db
		return nil
	}

	return fmt.Errorf("postgres.json not found and no fallback configured: %w", err)
}

func AutoMigrate() error {
	return DB.AutoMigrate(
		&Message{},
	)
}
