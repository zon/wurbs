package main

import (
	"fmt"
	"log/slog"

	"github.com/zon/chat/core/auth"
	"github.com/zon/chat/core/message"
	"github.com/zon/chat/core/pg"
	"gorm.io/gorm"
)

// MigrateDBCmd implements `wurbctl migrate db`.
type MigrateDBCmd struct{}

// Run applies all pending database migrations against the configured Postgres database.
func (c *MigrateDBCmd) Run() error {
	db, err := pg.Open()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := RunMigrations(db); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	slog.Info("database migrations complete")
	return nil
}

// RunMigrations applies all pending GORM AutoMigrate migrations.
func RunMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&auth.User{},
		&message.Message{},
	)
}
