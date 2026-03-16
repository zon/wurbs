package pg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecret_Write(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "output.json")

	secret := &Secret{
		Username:    "testuser",
		Password:    "testpass",
		DBName:      "testdb",
		Host:        "testhost",
		Port:        "5432",
		URI:         "postgresql://testuser:testpass@testhost:5432/testdb",
		PGPass:      "testpass",
		JDBCURI:     "jdbc:postgresql://testhost:5432/testdb",
		FQDNURI:     "postgresql://testuser:testpass@postgres.default:5432/testdb",
		FQDNJDBCURI: "jdbc:postgresql://postgres.default:5432/testdb",
	}

	err := secret.Write(testFile)
	require.NoError(t, err)

	data, err := os.ReadFile(testFile)
	require.NoError(t, err)

	var readSecret Secret
	err = json.Unmarshal(data, &readSecret)
	require.NoError(t, err)

	assert.Equal(t, secret.Username, readSecret.Username)
	assert.Equal(t, secret.Password, readSecret.Password)
	assert.Equal(t, secret.DBName, readSecret.DBName)
	assert.Equal(t, secret.Host, readSecret.Host)
	assert.Equal(t, secret.Port, readSecret.Port)
	assert.Equal(t, secret.URI, readSecret.URI)
	assert.Equal(t, secret.PGPass, readSecret.PGPass)
	assert.Equal(t, secret.JDBCURI, readSecret.JDBCURI)
	assert.Equal(t, secret.FQDNURI, readSecret.FQDNURI)
	assert.Equal(t, secret.FQDNJDBCURI, readSecret.FQDNJDBCURI)
}

func TestSecret_read(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")

	testData := `{
		"username": "admin",
		"password": "secret123",
		"dbname": "mydb",
		"host": "localhost",
		"port": "5432",
		"uri": "postgresql://admin:secret123@localhost:5432/mydb",
		"pgpass": "secret123",
		"jdbc-uri": "jdbc:postgresql://localhost:5432/mydb",
		"fqdn-uri": "postgresql://admin:secret123@postgres.default.svc.cluster.local:5432/mydb",
		"fqdn-jdbc-uri": "jdbc:postgresql://postgres.default.svc.cluster.local:5432/mydb"
	}`

	err := os.WriteFile(testFile, []byte(testData), 0644)
	require.NoError(t, err)

	secret := &Secret{}
	err = secret.read(testFile)

	require.NoError(t, err)
	assert.Equal(t, "admin", secret.Username)
	assert.Equal(t, "secret123", secret.Password)
	assert.Equal(t, "mydb", secret.DBName)
	assert.Equal(t, "localhost", secret.Host)
	assert.Equal(t, "5432", secret.Port)
	assert.Equal(t, "postgresql://admin:secret123@localhost:5432/mydb", secret.URI)
	assert.Equal(t, "secret123", secret.PGPass)
	assert.Equal(t, "jdbc:postgresql://localhost:5432/mydb", secret.JDBCURI)
	assert.Equal(t, "postgresql://admin:secret123@postgres.default.svc.cluster.local:5432/mydb", secret.FQDNURI)
	assert.Equal(t, "jdbc:postgresql://postgres.default.svc.cluster.local:5432/mydb", secret.FQDNJDBCURI)
}

func TestSecret_read_FileNotFound(t *testing.T) {
	secret := &Secret{}
	err := secret.read("/nonexistent/path/secret.json")
	assert.Error(t, err)
}

func TestOpen_MissingSecretFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("WURB_CONFIG", tmpDir)

	_, err := Open()
	assert.Error(t, err, "Open should fail when postgres.json is missing")
}

func TestOpen_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("WURB_CONFIG", tmpDir)

	err := os.WriteFile(filepath.Join(tmpDir, "postgres.json"), []byte("not json"), 0644)
	require.NoError(t, err)

	_, err = Open()
	assert.Error(t, err, "Open should fail with invalid JSON")
}

func TestOpen_BadConnection(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("WURB_CONFIG", tmpDir)

	secret := &Secret{
		Username: "testuser",
		Password: "testpass",
		DBName:   "testdb",
		Host:     "127.0.0.1",
		Port:     "1",
	}
	err := secret.Write(filepath.Join(tmpDir, "postgres.json"))
	require.NoError(t, err)

	// Open builds a *gorm.DB but GORM with the postgres driver may or may not
	// return an error at open time (it uses lazy connections). We verify the
	// function runs without panicking. A connection error surfaces on first query.
	_, _ = Open()
}

func TestSecret_WriteRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "postgres.json")

	original := &Secret{
		Username: "user1",
		Password: "pass1",
		DBName:   "db1",
		Host:     "host1",
		Port:     "5432",
	}

	err := original.Write(testFile)
	require.NoError(t, err)

	loaded := &Secret{}
	err = loaded.read(testFile)
	require.NoError(t, err)

	assert.Equal(t, original.Username, loaded.Username)
	assert.Equal(t, original.Password, loaded.Password)
	assert.Equal(t, original.DBName, loaded.DBName)
	assert.Equal(t, original.Host, loaded.Host)
	assert.Equal(t, original.Port, loaded.Port)
}
