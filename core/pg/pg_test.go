package pg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretStruct(t *testing.T) {
	secret := Secret{
		Username: "admin",
		Password: "secretpassword",
		DBName:   "mydb",
		Host:     "localhost",
		Port:     5432,
		URI:      "postgresql://admin:secretpassword@localhost:5432/mydb",
		PGPass:   "localhost:5432:mydb:admin:secretpassword",
		JDBCURI:  "jdbc:postgresql://localhost:5432/mydb",
		FQDNURI:  "postgresql://admin:secretpassword@mydb.namespace.svc.cluster.local:5432/mydb",
		FQDNJDBC: "jdbc:postgresql://mydb.namespace.svc.cluster.local:5432/mydb",
	}

	assert.Equal(t, "admin", secret.Username)
	assert.Equal(t, "secretpassword", secret.Password)
	assert.Equal(t, "mydb", secret.DBName)
	assert.Equal(t, "localhost", secret.Host)
	assert.Equal(t, 5432, secret.Port)
	assert.Equal(t, "postgresql://admin:secretpassword@localhost:5432/mydb", secret.URI)
	assert.Equal(t, "localhost:5432:mydb:admin:secretpassword", secret.PGPass)
	assert.Equal(t, "jdbc:postgresql://localhost:5432/mydb", secret.JDBCURI)
	assert.Equal(t, "postgresql://admin:secretpassword@mydb.namespace.svc.cluster.local:5432/mydb", secret.FQDNURI)
	assert.Equal(t, "jdbc:postgresql://mydb.namespace.svc.cluster.local:5432/mydb", secret.FQDNJDBC)
}

func TestReadSecret(t *testing.T) {
	tmpDir := t.TempDir()

	secretJSON := `{
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

	secretPath := filepath.Join(tmpDir, "secret.json")
	err := os.WriteFile(secretPath, []byte(secretJSON), 0644)
	require.NoError(t, err)

	secret, err := ReadSecret(secretPath)
	require.NoError(t, err)
	assert.Equal(t, "admin", secret.Username)
	assert.Equal(t, "secretpassword", secret.Password)
	assert.Equal(t, "mydb", secret.DBName)
	assert.Equal(t, "localhost", secret.Host)
	assert.Equal(t, 5432, secret.Port)
	assert.Equal(t, "postgresql://admin:secretpassword@localhost:5432/mydb", secret.URI)
	assert.Equal(t, "localhost:5432:mydb:admin:secretpassword", secret.PGPass)
	assert.Equal(t, "jdbc:postgresql://localhost:5432/mydb", secret.JDBCURI)
	assert.Equal(t, "postgresql://admin:secretpassword@mydb.namespace.svc.cluster.local:5432/mydb", secret.FQDNURI)
	assert.Equal(t, "jdbc:postgresql://mydb.namespace.svc.cluster.local:5432/mydb", secret.FQDNJDBC)
}

func TestReadSecretFileNotFound(t *testing.T) {
	_, err := ReadSecret("/nonexistent/path/secret.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read secret file")
}

func TestReadSecretInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	secretPath := filepath.Join(tmpDir, "secret.json")
	err := os.WriteFile(secretPath, []byte("invalid json"), 0644)
	require.NoError(t, err)

	_, err = ReadSecret(secretPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse secret file")
}

func TestWriteSecret(t *testing.T) {
	tmpDir := t.TempDir()

	secret := &Secret{
		Username: "admin",
		Password: "secretpassword",
		DBName:   "mydb",
		Host:     "localhost",
		Port:     5432,
		URI:      "postgresql://admin:secretpassword@localhost:5432/mydb",
		PGPass:   "localhost:5432:mydb:admin:secretpassword",
		JDBCURI:  "jdbc:postgresql://localhost:5432/mydb",
		FQDNURI:  "postgresql://admin:secretpassword@mydb.namespace.svc.cluster.local:5432/mydb",
		FQDNJDBC: "jdbc:postgresql://mydb.namespace.svc.cluster.local:5432/mydb",
	}

	secretPath := filepath.Join(tmpDir, "secret.json")
	err := WriteSecret(secretPath, secret)
	require.NoError(t, err)

	data, err := os.ReadFile(secretPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"username": "admin"`)
	assert.Contains(t, string(data), `"password": "secretpassword"`)
	assert.Contains(t, string(data), `"dbname": "mydb"`)
	assert.Contains(t, string(data), `"host": "localhost"`)
	assert.Contains(t, string(data), `"port": 5432`)
}

func TestWriteAndReadSecretRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	original := &Secret{
		Username: "admin",
		Password: "secretpassword",
		DBName:   "mydb",
		Host:     "localhost",
		Port:     5432,
		URI:      "postgresql://admin:secretpassword@localhost:5432/mydb",
		PGPass:   "localhost:5432:mydb:admin:secretpassword",
		JDBCURI:  "jdbc:postgresql://localhost:5432/mydb",
		FQDNURI:  "postgresql://admin:secretpassword@mydb.namespace.svc.cluster.local:5432/mydb",
		FQDNJDBC: "jdbc:postgresql://mydb.namespace.svc.cluster.local:5432/mydb",
	}

	secretPath := filepath.Join(tmpDir, "secret.json")
	err := WriteSecret(secretPath, original)
	require.NoError(t, err)

	loaded, err := ReadSecret(secretPath)
	require.NoError(t, err)

	assert.Equal(t, original.Username, loaded.Username)
	assert.Equal(t, original.Password, loaded.Password)
	assert.Equal(t, original.DBName, loaded.DBName)
	assert.Equal(t, original.Host, loaded.Host)
	assert.Equal(t, original.Port, loaded.Port)
	assert.Equal(t, original.URI, loaded.URI)
	assert.Equal(t, original.PGPass, loaded.PGPass)
	assert.Equal(t, original.JDBCURI, loaded.JDBCURI)
	assert.Equal(t, original.FQDNURI, loaded.FQDNURI)
	assert.Equal(t, original.FQDNJDBC, loaded.FQDNJDBC)
}

func TestOpenDB(t *testing.T) {
	secret := &Secret{
		Username: "nonexistent_user",
		Password: "wrongpassword",
		DBName:   "nonexistent_db",
		Host:     "localhost",
		Port:     5432,
	}

	_, err := OpenDB(secret)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to PostgreSQL")
}
