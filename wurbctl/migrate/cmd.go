package migrate

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/zon/chat/core/config"
	"github.com/zon/chat/core/pg"
)

// Cmd is the top-level `migrate` command group.
type Cmd struct {
	DB DBCmd `cmd:"" name:"db" help:"Apply all pending database migrations."`
}

// DBCmd implements `wurbctl migrate db`.
type DBCmd struct{}

// Run applies all pending database migrations against the configured Postgres database.
func (c *DBCmd) Run() error {
	configDir, err := config.Dir()
	if err != nil {
		return fmt.Errorf("failed to resolve config directory: %w", err)
	}

	postgresPath := filepath.Join(configDir, "postgres.json")
	var secret pg.Secret
	if err := secret.Read(postgresPath); err != nil {
		return fmt.Errorf("failed to read postgres config: %w", err)
	}

	db, err := secret.Open()
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
