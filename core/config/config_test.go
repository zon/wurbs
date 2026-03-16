package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDir_EnvVarOverride(t *testing.T) {
	t.Setenv(envConfigDir, "/custom/config/path")
	dir, err := Dir()
	require.NoError(t, err)
	assert.Equal(t, "/custom/config/path", dir)
}

func TestDir_DefaultsToEtcWurbs(t *testing.T) {
	t.Setenv(envConfigDir, "")
	SetTestMode(false)
	defer SetTestMode(false)

	dir, err := Dir()
	require.NoError(t, err)
	assert.Equal(t, defaultConfigDir, dir)
}

func TestDir_TestModeFindsRepoConfig(t *testing.T) {
	// Create a fake repo with .git dir and config dir.
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "myrepo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755))
	configDir := filepath.Join(repoRoot, "config")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	// Subdir to start the walk from.
	subdir := filepath.Join(repoRoot, "a", "b")
	require.NoError(t, os.MkdirAll(subdir, 0755))

	t.Setenv(envConfigDir, "")
	SetTestMode(true)
	defer SetTestMode(false)

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(subdir))
	defer os.Chdir(origDir)

	dir, err := Dir()
	require.NoError(t, err)
	assert.Equal(t, configDir, dir)
}

func TestDir_TestModeFallsBackWhenNoConfigDir(t *testing.T) {
	// Repo root exists but has no ./config directory.
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "myrepo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755))

	t.Setenv(envConfigDir, "")
	SetTestMode(true)
	defer SetTestMode(false)

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(repoRoot))
	defer os.Chdir(origDir)

	dir, err := Dir()
	require.NoError(t, err)
	assert.Equal(t, defaultConfigDir, dir)
}

func TestDir_EnvVarTakesPrecedenceOverTestMode(t *testing.T) {
	t.Setenv(envConfigDir, "/override")
	SetTestMode(true)
	defer SetTestMode(false)

	dir, err := Dir()
	require.NoError(t, err)
	assert.Equal(t, "/override", dir)
}

func TestLoad_ReadsConfigYAML(t *testing.T) {
	tmp := t.TempDir()
	content := "host: localhost\nport: 8080\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "config.yaml"), []byte(content), 0644))

	t.Setenv(envConfigDir, tmp)

	var cfg struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	}
	require.NoError(t, Load(&cfg))
	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, 8080, cfg.Port)
}

func TestLoadSecret_ReadsSecretYAML(t *testing.T) {
	tmp := t.TempDir()
	content := "db_password: s3cret\napi_key: abc123\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "secret.yaml"), []byte(content), 0644))

	t.Setenv(envConfigDir, tmp)

	var secret struct {
		DBPassword string `yaml:"db_password"`
		APIKey     string `yaml:"api_key"`
	}
	require.NoError(t, LoadSecret(&secret))
	assert.Equal(t, "s3cret", secret.DBPassword)
	assert.Equal(t, "abc123", secret.APIKey)
}

func TestLoad_ErrorWhenFileMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(envConfigDir, tmp)

	var cfg struct{}
	err := Load(&cfg)
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestLoadSecret_ErrorWhenFileMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(envConfigDir, tmp)

	var secret struct{}
	err := LoadSecret(&secret)
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestEnvVarName(t *testing.T) {
	assert.Equal(t, "WURB_CONFIG", envConfigDir)
}

func TestDefaultDir(t *testing.T) {
	assert.Equal(t, "/etc/wurbs", defaultConfigDir)
}

func TestLoad_NestedYAML(t *testing.T) {
	tmp := t.TempDir()
	content := `
database:
  host: db.example.com
  port: 5432
nats:
  url: nats://localhost:4222
`
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "config.yaml"), []byte(content), 0644))

	t.Setenv(envConfigDir, tmp)

	var cfg struct {
		Database struct {
			Host string `yaml:"host"`
			Port int    `yaml:"port"`
		} `yaml:"database"`
		NATS struct {
			URL string `yaml:"url"`
		} `yaml:"nats"`
	}
	require.NoError(t, Load(&cfg))
	assert.Equal(t, "db.example.com", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "nats://localhost:4222", cfg.NATS.URL)
}
