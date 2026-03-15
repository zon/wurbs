package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zon/chat/core"
	"github.com/zon/chat/core/pg"
)

func TestServerUsesCorePg(t *testing.T) {
	tmpDir := t.TempDir()

	os.Setenv("WURBS_CONFIG", tmpDir)
	defer os.Unsetenv("WURBS_CONFIG")

	postgresJSON := `{
		"username": "admin",
		"password": "secretpassword",
		"dbname": "mydb",
		"host": "localhost",
		"port": 5432,
		"uri": "postgresql://admin:secretpassword@localhost:5432/mydb",
		"pgpass": "localhost:5432:mydb:admin:secretpassword",
		"jdbc-uri": "jdbc:postgresql://localhost:5432/mydb",
		"fqdn-uri": "postgresql://admin:secretpassword@mydb.namespace.svc.cluster.local:5432/mydb",
		"fqdn-jdbc-uri": "jdbc:postgresql://mydb.namespace.svc.cluster.local:5432/mydb"
	}`

	secretPath := filepath.Join(tmpDir, "postgres.json")
	err := os.WriteFile(secretPath, []byte(postgresJSON), 0644)
	require.NoError(t, err)

	configDir, err := core.GetConfigDir("/workspace/repo")
	require.NoError(t, err)
	assert.Equal(t, tmpDir, configDir)

	secret, err := pg.ReadSecret(secretPath)
	require.NoError(t, err)
	assert.Equal(t, "admin", secret.Username)
	assert.Equal(t, "localhost", secret.Host)
	assert.Equal(t, 5432, secret.Port)
}

func TestServerDbConnectionWithPgSecret(t *testing.T) {
	tmpDir := t.TempDir()

	postgresJSON := `{
		"username": "nonexistent_user",
		"password": "wrongpassword",
		"dbname": "nonexistent_db",
		"host": "localhost",
		"port": 5432
	}`

	secretPath := filepath.Join(tmpDir, "postgres.json")
	err := os.WriteFile(secretPath, []byte(postgresJSON), 0644)
	require.NoError(t, err)

	secret, err := pg.ReadSecret(secretPath)
	require.NoError(t, err)

	_, err = pg.OpenDB(secret)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to PostgreSQL")
}
