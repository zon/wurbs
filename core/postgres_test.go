package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPostgresConfig_Success(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "postgres.json")

	jsonContent := `{
		"host": "localhost",
		"port": 5432,
		"user": "testuser",
		"password": "testpass",
		"database": "testdb"
	}`

	err := os.WriteFile(configPath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadPostgresConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, 5432, cfg.Port)
	assert.Equal(t, "testuser", cfg.User)
	assert.Equal(t, "testpass", cfg.Password)
	assert.Equal(t, "testdb", cfg.Database)
}

func TestLoadPostgresConfig_FileNotFound(t *testing.T) {
	dir := t.TempDir()

	_, err := LoadPostgresConfig(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "postgres.json")
}

func TestLoadPostgresConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "postgres.json")

	err := os.WriteFile(configPath, []byte("invalid json"), 0644)
	require.NoError(t, err)

	_, err = LoadPostgresConfig(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestLoadPostgresConfig_MissingHost(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "postgres.json")

	jsonContent := `{
		"port": 5432,
		"user": "testuser",
		"password": "testpass",
		"database": "testdb"
	}`

	err := os.WriteFile(configPath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	_, err = LoadPostgresConfig(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "host is required")
}

func TestLoadPostgresConfig_MissingPort(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "postgres.json")

	jsonContent := `{
		"host": "localhost",
		"user": "testuser",
		"password": "testpass",
		"database": "testdb"
	}`

	err := os.WriteFile(configPath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	_, err = LoadPostgresConfig(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "port is required")
}

func TestLoadPostgresConfig_MissingUser(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "postgres.json")

	jsonContent := `{
		"host": "localhost",
		"port": 5432,
		"password": "testpass",
		"database": "testdb"
	}`

	err := os.WriteFile(configPath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	_, err = LoadPostgresConfig(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user is required")
}

func TestLoadPostgresConfig_MissingPassword(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "postgres.json")

	jsonContent := `{
		"host": "localhost",
		"port": 5432,
		"user": "testuser",
		"database": "testdb"
	}`

	err := os.WriteFile(configPath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	_, err = LoadPostgresConfig(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "password is required")
}

func TestLoadPostgresConfig_MissingDatabase(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "postgres.json")

	jsonContent := `{
		"host": "localhost",
		"port": 5432,
		"user": "testuser",
		"password": "testpass"
	}`

	err := os.WriteFile(configPath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	_, err = LoadPostgresConfig(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database is required")
}

func TestLoadPostgresConfig_DefaultConfigDir(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	configPath := filepath.Join(dir, "config", "postgres.json")
	err := os.MkdirAll(filepath.Dir(configPath), 0755)
	require.NoError(t, err)

	jsonContent := `{
		"host": "localhost",
		"port": 5432,
		"user": "testuser",
		"password": "testpass",
		"database": "testdb"
	}`

	err = os.WriteFile(configPath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadPostgresConfig("")
	require.NoError(t, err)
	assert.Equal(t, "localhost", cfg.Host)
}
