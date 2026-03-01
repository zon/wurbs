package set

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// EnsurePostgresUserAndDB connects as the admin user and creates the app user
// and database if they do not already exist.
func EnsurePostgresUserAndDB(host string, port int, adminUser, adminPassword, appUser, appPassword, dbName string) error {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=postgres sslmode=disable",
		host, port, adminUser, adminPassword,
	)

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("failed to connect as admin: %w", err)
	}
	defer conn.Close(ctx)

	// Create user if not exists
	var userExists bool
	err = conn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", appUser,
	).Scan(&userExists)
	if err != nil {
		return fmt.Errorf("failed to check if user exists: %w", err)
	}

	if !userExists {
		_, err = conn.Exec(ctx, fmt.Sprintf(
			"CREATE USER %s WITH PASSWORD '%s'",
			pgx.Identifier{appUser}.Sanitize(), appPassword,
		))
		if err != nil {
			return fmt.Errorf("failed to create user %s: %w", appUser, err)
		}
	} else {
		// Update password
		_, err = conn.Exec(ctx, fmt.Sprintf(
			"ALTER USER %s WITH PASSWORD '%s'",
			pgx.Identifier{appUser}.Sanitize(), appPassword,
		))
		if err != nil {
			return fmt.Errorf("failed to update user %s: %w", appUser, err)
		}
	}

	// Create database if not exists
	var dbExists bool
	err = conn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName,
	).Scan(&dbExists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}

	if !dbExists {
		_, err = conn.Exec(ctx, fmt.Sprintf(
			"CREATE DATABASE %s OWNER %s",
			pgx.Identifier{dbName}.Sanitize(), pgx.Identifier{appUser}.Sanitize(),
		))
		if err != nil {
			return fmt.Errorf("failed to create database %s: %w", dbName, err)
		}
	} else {
		// Grant ownership in case it exists but owner is different
		_, err = conn.Exec(ctx, fmt.Sprintf(
			"GRANT ALL PRIVILEGES ON DATABASE %s TO %s",
			pgx.Identifier{dbName}.Sanitize(), pgx.Identifier{appUser}.Sanitize(),
		))
		if err != nil {
			return fmt.Errorf("failed to grant privileges on %s: %w", dbName, err)
		}
	}

	return nil
}
