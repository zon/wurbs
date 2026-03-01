package migrate

import (
	"fmt"
	"log/slog"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Cmd is the top-level `migrate` command group.
type Cmd struct {
	DB DBCmd `cmd:"" name:"db" help:"Apply all pending database migrations."`
}

// DBCmd implements `wurbctl migrate db`.
type DBCmd struct{}

// Run applies all pending database migrations against the configured Postgres database.
func (c *DBCmd) Run() error {
	dsn, err := DSN()
	if err != nil {
		return fmt.Errorf("database configuration error: %w", err)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	slog.Info("running database migrations")
	err = RunMigrations(db)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	slog.Info("database migrations complete")
	return nil
}
