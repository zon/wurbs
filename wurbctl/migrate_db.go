package main

import (
	"fmt"
	"log/slog"

	"github.com/zon/chat/core/user"
	"github.com/zon/chat/core/message"
	"gorm.io/gorm"
)

// MigrateDBCmd implements `wurbctl migrate db`.
type MigrateDBCmd struct{}

// Run applies all pending database migrations against the configured Postgres database.
func (c *MigrateDBCmd) Run(ctx *Context) error {
	db, err := ctx.DB()
	if err != nil {
		return err
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
		&user.User{},
		&message.Message{},
	)
}
