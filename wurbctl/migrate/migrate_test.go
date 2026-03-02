package migrate

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// writeConfigFile writes a k8s ConfigMap YAML to a temp file.
func writeConfigFile(t *testing.T, dir string, data map[string]string) {
	t.Helper()
	var content string
	content += "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: wurbs-config\ndata:\n"
	for k, v := range data {
		content += "  " + k + ": \"" + v + "\"\n"
	}
	err := os.WriteFile(filepath.Join(dir, "main.yaml"), []byte(content), 0600)
	require.NoError(t, err)
}

// writeSecretsFile writes a k8s Secret YAML to a temp file with base64-encoded values.
func writeSecretsFile(t *testing.T, dir string, data map[string]string) {
	t.Helper()
	var content string
	content += "apiVersion: v1\nkind: Secret\nmetadata:\n  name: wurbs-secret\ntype: Opaque\ndata:\n"
	for k, v := range data {
		encoded := base64.StdEncoding.EncodeToString([]byte(v))
		content += "  " + k + ": " + encoded + "\n"
	}
	err := os.WriteFile(filepath.Join(dir, "secrets.yaml"), []byte(content), 0600)
	require.NoError(t, err)
}

// setConfigDir sets WURB_CONFIG to a temp directory and restores it after the test.
func setConfigDir(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("WURB_CONFIG", dir)
}

// --- ConfigDir tests ---

func TestConfigDir_EnvVar(t *testing.T) {
	t.Setenv("WURB_CONFIG", "/custom/config")
	assert.Equal(t, "/custom/config", ConfigDir())
}

func TestConfigDir_Default(t *testing.T) {
	t.Setenv("WURB_CONFIG", "")
	assert.Equal(t, "/etc/wurbs", ConfigDir())
}

// --- LoadConfig tests ---

func TestLoadConfig_Success(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, dir, map[string]string{
		"PGHOST":     "db.example.com",
		"PGPORT":     "5433",
		"PGDATABASE": "wurbs_db",
		"PGUSER":     "wurbs",
	})

	cfg, err := LoadConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, "db.example.com", cfg["PGHOST"])
	assert.Equal(t, "5433", cfg["PGPORT"])
	assert.Equal(t, "wurbs_db", cfg["PGDATABASE"])
	assert.Equal(t, "wurbs", cfg["PGUSER"])
}

func TestLoadConfig_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadConfig(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "main.yaml")
}

// --- LoadSecrets tests ---

func TestLoadSecrets_Success(t *testing.T) {
	dir := t.TempDir()
	writeSecretsFile(t, dir, map[string]string{
		"PGPASSWORD": "supersecret",
	})

	secrets, err := LoadSecrets(dir)
	require.NoError(t, err)
	assert.Equal(t, "supersecret", secrets["PGPASSWORD"])
}

func TestLoadSecrets_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadSecrets(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secrets.yaml")
}

// --- DSN tests ---

func TestDSN_Success(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, dir, map[string]string{
		"PGHOST":     "localhost",
		"PGPORT":     "5432",
		"PGDATABASE": "wurbs",
		"PGUSER":     "admin",
	})
	writeSecretsFile(t, dir, map[string]string{
		"PGPASSWORD": "secret",
	})
	setConfigDir(t, dir)

	dsn, err := DSN()
	require.NoError(t, err)
	assert.Contains(t, dsn, "host=localhost")
	assert.Contains(t, dsn, "port=5432")
	assert.Contains(t, dsn, "user=admin")
	assert.Contains(t, dsn, "password=secret")
	assert.Contains(t, dsn, "dbname=wurbs")
}

func TestDSN_DefaultPort(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, dir, map[string]string{
		"PGHOST":     "localhost",
		"PGDATABASE": "wurbs",
		"PGUSER":     "admin",
	})
	writeSecretsFile(t, dir, map[string]string{
		"PGPASSWORD": "secret",
	})
	setConfigDir(t, dir)

	dsn, err := DSN()
	require.NoError(t, err)
	assert.Contains(t, dsn, "port=5432")
}

func TestDSN_MissingHost(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, dir, map[string]string{
		"PGDATABASE": "wurbs",
		"PGUSER":     "admin",
	})
	writeSecretsFile(t, dir, map[string]string{
		"PGPASSWORD": "secret",
	})
	setConfigDir(t, dir)

	_, err := DSN()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PGHOST")
}

func TestDSN_MissingUser(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, dir, map[string]string{
		"PGHOST":     "localhost",
		"PGDATABASE": "wurbs",
	})
	writeSecretsFile(t, dir, map[string]string{
		"PGPASSWORD": "secret",
	})
	setConfigDir(t, dir)

	_, err := DSN()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PGUSER")
}

func TestDSN_MissingDatabase(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, dir, map[string]string{
		"PGHOST": "localhost",
		"PGUSER": "admin",
	})
	writeSecretsFile(t, dir, map[string]string{
		"PGPASSWORD": "secret",
	})
	setConfigDir(t, dir)

	_, err := DSN()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PGDATABASE")
}

func TestDSN_MissingMultiple(t *testing.T) {
	dir := t.TempDir()
	writeConfigFile(t, dir, map[string]string{})
	writeSecretsFile(t, dir, map[string]string{})
	setConfigDir(t, dir)

	_, err := DSN()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PGHOST")
	assert.Contains(t, err.Error(), "PGUSER")
	assert.Contains(t, err.Error(), "PGDATABASE")
}

func TestDSN_MissingConfigFile(t *testing.T) {
	dir := t.TempDir()
	// Only write secrets, no config
	writeSecretsFile(t, dir, map[string]string{
		"PGPASSWORD": "secret",
	})
	setConfigDir(t, dir)

	_, err := DSN()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config")
}

func TestDSN_MissingSecretsFile(t *testing.T) {
	dir := t.TempDir()
	// Only write config, no secrets
	writeConfigFile(t, dir, map[string]string{
		"PGHOST":     "localhost",
		"PGDATABASE": "wurbs",
		"PGUSER":     "admin",
	})
	setConfigDir(t, dir)

	_, err := DSN()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secrets")
}

// --- RunMigrations tests ---

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

// --- DBCmd.Run tests ---

// TestDBCmd_Run_MissingConfigFiles verifies DBCmd.Run returns an error when config files are absent.
func TestDBCmd_Run_MissingConfigFiles(t *testing.T) {
	dir := t.TempDir()
	setConfigDir(t, dir)
	// No config files written

	cmd := &DBCmd{}
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database configuration error")
}
