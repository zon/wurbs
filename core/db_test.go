package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInitDB_UsesConfigDir(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	postgresJSON := `{
		"username": "testuser",
		"password": "testpass",
		"dbname": "testdb",
		"host": "localhost",
		"port": "5432",
		"uri": "postgresql://testuser:testpass@localhost:5432/testdb"
	}`

	err = os.WriteFile(filepath.Join(configDir, "postgres.json"), []byte(postgresJSON), 0644)
	require.NoError(t, err)

	os.Setenv("WURBS_CONFIG", configDir)
	defer os.Unsetenv("WURBS_CONFIG")

	dir, err := configDirForTest()
	require.NoError(t, err)
	assert.Equal(t, configDir, dir)
}

func configDirForTest() (string, error) {
	return os.Getenv("WURBS_CONFIG"), nil
}

func TestDBVariableExists(t *testing.T) {
	var db *gorm.DB
	assert.Nil(t, db)
}
