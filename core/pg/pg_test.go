package pg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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

func TestSecret_Patch(t *testing.T) {
	secret := &Secret{
		Host:        "oldhost",
		Port:        "5432",
		URI:         "postgresql://user:pass@oldhost:5432/mydb",
		JDBCURI:     "jdbc:postgresql://oldhost:5432/mydb",
		FQDNURI:     "postgresql://user:pass@oldhost:5432/mydb",
		FQDNJDBCURI: "jdbc:postgresql://oldhost:5432/mydb",
	}

	secret.Patch("newhost", "5433")

	assert.Equal(t, "newhost", secret.Host)
	assert.Equal(t, "5433", secret.Port)
	assert.Equal(t, "postgresql://user:pass@newhost:5433/mydb", secret.URI)
	assert.Equal(t, "jdbc:postgresql://newhost:5433/mydb", secret.JDBCURI)
	assert.Equal(t, "postgresql://user:pass@newhost:5433/mydb", secret.FQDNURI)
	assert.Equal(t, "jdbc:postgresql://newhost:5433/mydb", secret.FQDNJDBCURI)
}

func TestSecret_Patch_EmptyURI(t *testing.T) {
	secret := &Secret{
		Host: "oldhost",
		Port: "5432",
	}

	secret.Patch("newhost", "5433")

	assert.Equal(t, "newhost", secret.Host)
	assert.Equal(t, "5433", secret.Port)
	assert.Equal(t, "", secret.URI)
	assert.Equal(t, "", secret.JDBCURI)
}
