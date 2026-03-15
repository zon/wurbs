package core

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const defaultDatabase = "chat"
const sqliteFile = defaultDatabase + ".db"

var DB *gorm.DB

func InitDB(cfg *Config, secrets *Secrets) error {
	if cfg.Database.Type == "postgres" {
		dsn := fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			cfg.Server.Host, cfg.Database.Port, secrets.Database.User, secrets.Database.Password, cfg.Database.Name,
		)
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			return fmt.Errorf("failed to connect to PostgreSQL: %w", err)
		}
		DB = db
		return nil
	}

	return fmt.Errorf("unsupported database type: %s", cfg.Database.Type)
}

func AutoMigrate() error {
	return DB.AutoMigrate(
		&Message{},
		&User{},
	)
}
