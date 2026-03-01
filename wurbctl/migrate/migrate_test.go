package migrate

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestDSN_Success verifies DSN is built correctly when all required env vars are set.
func TestDSN_Success(t *testing.T) {
	t.Setenv("PGHOST", "localhost")
	t.Setenv("PGPORT", "5432")
	t.Setenv("PGUSER", "admin")
	t.Setenv("PGPASSWORD", "secret")
	t.Setenv("PGDATABASE", "wurbs")

	dsn, err := DSN()
	require.NoError(t, err)
	assert.Contains(t, dsn, "host=localhost")
	assert.Contains(t, dsn, "port=5432")
	assert.Contains(t, dsn, "user=admin")
	assert.Contains(t, dsn, "password=secret")
	assert.Contains(t, dsn, "dbname=wurbs")
}

// TestDSN_DefaultPort verifies the port defaults to 5432 when PGPORT is not set.
func TestDSN_DefaultPort(t *testing.T) {
	t.Setenv("PGHOST", "localhost")
	t.Setenv("PGPORT", "")
	t.Setenv("PGUSER", "admin")
	t.Setenv("PGPASSWORD", "secret")
	t.Setenv("PGDATABASE", "wurbs")

	dsn, err := DSN()
	require.NoError(t, err)
	assert.Contains(t, dsn, "port=5432")
}

// TestDSN_MissingHost verifies an error is returned when PGHOST is missing.
func TestDSN_MissingHost(t *testing.T) {
	t.Setenv("PGHOST", "")
	t.Setenv("PGUSER", "admin")
	t.Setenv("PGDATABASE", "wurbs")

	_, err := DSN()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PGHOST")
}

// TestDSN_MissingUser verifies an error is returned when PGUSER is missing.
func TestDSN_MissingUser(t *testing.T) {
	t.Setenv("PGHOST", "localhost")
	t.Setenv("PGUSER", "")
	t.Setenv("PGDATABASE", "wurbs")

	_, err := DSN()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PGUSER")
}

// TestDSN_MissingDatabase verifies an error is returned when PGDATABASE is missing.
func TestDSN_MissingDatabase(t *testing.T) {
	t.Setenv("PGHOST", "localhost")
	t.Setenv("PGUSER", "admin")
	t.Setenv("PGDATABASE", "")

	_, err := DSN()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PGDATABASE")
}

// TestDSN_MissingMultiple verifies that all missing variables are reported.
func TestDSN_MissingMultiple(t *testing.T) {
	t.Setenv("PGHOST", "")
	t.Setenv("PGUSER", "")
	t.Setenv("PGDATABASE", "")

	_, err := DSN()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PGHOST")
	assert.Contains(t, err.Error(), "PGUSER")
	assert.Contains(t, err.Error(), "PGDATABASE")
}

// TestRunMigrations verifies that RunMigrations applies all pending migrations successfully.
// Uses an in-memory SQLite database to avoid requiring a running PostgreSQL instance.
func TestRunMigrations(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err, "failed to open in-memory sqlite database")

	err = RunMigrations(db)
	require.NoError(t, err, "RunMigrations should not return an error")

	// Verify the messages table was created by checking GORM can perform operations.
	assert.True(t, db.Migrator().HasTable(&Message{}), "messages table should exist after migration")
}

// TestRunMigrations_Idempotent verifies that running migrations multiple times is safe.
func TestRunMigrations_Idempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = RunMigrations(db)
	require.NoError(t, err, "first RunMigrations should succeed")

	err = RunMigrations(db)
	require.NoError(t, err, "second RunMigrations should also succeed (idempotent)")
}

// TestRunMigrations_CreatesColumns verifies the messages table has the expected columns.
func TestRunMigrations_CreatesColumns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = RunMigrations(db)
	require.NoError(t, err)

	migrator := db.Migrator()
	assert.True(t, migrator.HasColumn(&Message{}, "id"), "messages table should have id column")
	assert.True(t, migrator.HasColumn(&Message{}, "created_at"), "messages table should have created_at column")
	assert.True(t, migrator.HasColumn(&Message{}, "updated_at"), "messages table should have updated_at column")
	assert.True(t, migrator.HasColumn(&Message{}, "deleted_at"), "messages table should have deleted_at column")
	assert.True(t, migrator.HasColumn(&Message{}, "user_id"), "messages table should have user_id column")
	assert.True(t, migrator.HasColumn(&Message{}, "content"), "messages table should have content column")
}

// TestDBCmd_Run_MissingEnvVars verifies DBCmd.Run returns an error when PG env vars are missing.
func TestDBCmd_Run_MissingEnvVars(t *testing.T) {
	// Clear all PG env vars
	for _, key := range []string{"PGHOST", "PGPORT", "PGUSER", "PGPASSWORD", "PGDATABASE"} {
		os.Unsetenv(key)
	}

	cmd := &DBCmd{}
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database configuration error")
}
