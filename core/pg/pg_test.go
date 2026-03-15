package pg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecret_Read(t *testing.T) {
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
	err = secret.Read(testFile)

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

func TestSecret_Read_FileNotFound(t *testing.T) {
	secret := &Secret{}
	err := secret.Read("/nonexistent/path/secret.json")
	assert.Error(t, err)
}

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

func TestSecret_AllFields(t *testing.T) {
	secret := &Secret{
		Username:    "user",
		Password:    "pass",
		DBName:      "db",
		Host:        "host",
		Port:        "port",
		URI:         "uri",
		PGPass:      "pgpass",
		JDBCURI:     "jdbc",
		FQDNURI:     "fqdn",
		FQDNJDBCURI: "fqdn-jdbc",
	}

	assert.Equal(t, "user", secret.Username)
	assert.Equal(t, "pass", secret.Password)
	assert.Equal(t, "db", secret.DBName)
	assert.Equal(t, "host", secret.Host)
	assert.Equal(t, "port", secret.Port)
	assert.Equal(t, "uri", secret.URI)
	assert.Equal(t, "pgpass", secret.PGPass)
	assert.Equal(t, "jdbc", secret.JDBCURI)
	assert.Equal(t, "fqdn", secret.FQDNURI)
	assert.Equal(t, "fqdn-jdbc", secret.FQDNJDBCURI)
}
