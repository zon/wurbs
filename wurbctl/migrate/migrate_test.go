package migrate

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zon/chat/core/pg"
)

func writePostgresSecret(t *testing.T, dir string, secret *pg.Secret) {
	t.Helper()
	path := filepath.Join(dir, "postgres.json")
	err := pg.WriteSecret(path, secret)
	require.NoError(t, err)
}

func TestDBCmd_Run_UsesCoreConfig(t *testing.T) {
	dir := t.TempDir()
	secret := &pg.Secret{
		Username: "user",
		Password: "pass",
		DBName:   "testdb",
		Host:     "localhost",
		Port:     5432,
		URI:      "postgres://user:pass@localhost:5432/testdb",
	}
	writePostgresSecret(t, dir, secret)

	t.Setenv("WURBS_CONFIG", dir)

	cmd := &DBCmd{}
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connect to database")
}

func TestDBCmd_Run_MissingPostgresJson(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WURBS_CONFIG", dir)

	cmd := &DBCmd{}
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read postgres secret")
}

func TestDBCmd_Run_MissingConfigDir(t *testing.T) {
	dir := t.TempDir()
	secret := &pg.Secret{
		Username: "user",
		Password: "pass",
		DBName:   "testdb",
		Host:     "localhost",
		Port:     5432,
	}
	writePostgresSecret(t, dir, secret)

	t.Setenv("WURBS_CONFIG", "/nonexistent/path")

	cmd := &DBCmd{}
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read postgres secret")
}
