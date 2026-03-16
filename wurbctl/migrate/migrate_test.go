package migrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zon/chat/core"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func writePostgresJSON(t *testing.T, dir string, secret map[string]string) {
	t.Helper()
	data, err := json.Marshal(secret)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "postgres.json"), data, 0600)
	require.NoError(t, err)
}

func setWurbsConfigDir(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("WURB_CONFIG", dir)
}

// --- RunMigrations tests ---

func TestRunMigrations(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err, "failed to open in-memory sqlite database")

	err = RunMigrations(db)
	require.NoError(t, err, "RunMigrations should not return an error")

	assert.True(t, db.Migrator().HasTable(&core.User{}), "users table should exist after migration")
	assert.True(t, db.Migrator().HasTable(&core.Message{}), "messages table should exist after migration")
}

func TestRunMigrations_Idempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = RunMigrations(db)
	require.NoError(t, err, "first RunMigrations should succeed")

	err = RunMigrations(db)
	require.NoError(t, err, "second RunMigrations should also succeed (idempotent)")
}

func TestRunMigrations_CreatesColumns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = RunMigrations(db)
	require.NoError(t, err)

	migrator := db.Migrator()
	assert.True(t, migrator.HasColumn(&core.Message{}, "id"), "messages table should have id column")
	assert.True(t, migrator.HasColumn(&core.Message{}, "created_at"), "messages table should have created_at column")
	assert.True(t, migrator.HasColumn(&core.Message{}, "updated_at"), "messages table should have updated_at column")
	assert.True(t, migrator.HasColumn(&core.Message{}, "deleted_at"), "messages table should have deleted_at column")
	assert.True(t, migrator.HasColumn(&core.Message{}, "user_id"), "messages table should have user_id column")
	assert.True(t, migrator.HasColumn(&core.Message{}, "content"), "messages table should have content column")

	assert.True(t, migrator.HasColumn(&core.User{}, "id"), "users table should have id column")
	assert.True(t, migrator.HasColumn(&core.User{}, "created_at"), "users table should have created_at column")
	assert.True(t, migrator.HasColumn(&core.User{}, "updated_at"), "users table should have updated_at column")
	assert.True(t, migrator.HasColumn(&core.User{}, "deleted_at"), "users table should have deleted_at column")
}

// --- DBCmd.Run tests ---

func TestDBCmd_Run_MissingConfigFiles(t *testing.T) {
	dir := t.TempDir()
	setWurbsConfigDir(t, dir)

	cmd := &DBCmd{}
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to database")
}

func TestDBCmd_Run_InvalidPostgresJSON(t *testing.T) {
	dir := t.TempDir()
	setWurbsConfigDir(t, dir)

	err := os.WriteFile(filepath.Join(dir, "postgres.json"), []byte("invalid json"), 0600)
	require.NoError(t, err)

	cmd := &DBCmd{}
	err = cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to database")
}
