package migrate

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/zon/chat/core"
)

// Cmd is the top-level `migrate` command group.
type Cmd struct {
	DB DBCmd `cmd:"" name:"db" help:"Apply all pending database migrations."`
}

// DBCmd implements `wurbctl migrate db`.
type DBCmd struct{}

// Run applies all pending database migrations against the configured Postgres database.
func (c *DBCmd) Run() error {
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	configDir, err := core.GetConfigDir(workingDir)
	if err != nil {
		return fmt.Errorf("config directory error: %w", err)
	}

	postgresSecretPath := filepath.Join(configDir, "postgres.json")
	secret, err := core.ReadSecret(postgresSecretPath)
	if err != nil {
		return fmt.Errorf("failed to read postgres secret: %w", err)
	}

	db, err := core.OpenDB(secret)
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
