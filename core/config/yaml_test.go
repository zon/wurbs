package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ReadsConfigYAML(t *testing.T) {
	resetCache()
	defer resetCache()
	tmp := t.TempDir()
	content := "host: localhost\nport: 8080\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "config.yaml"), []byte(content), 0644))

	t.Setenv(envConfigDir, tmp)

	var cfg struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	}
	require.NoError(t, LoadYAML(&cfg))
	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, 8080, cfg.Port)
}

func TestLoad_ErrorWhenFileMissing(t *testing.T) {
	resetCache()
	defer resetCache()
	tmp := t.TempDir()
	t.Setenv(envConfigDir, tmp)

	var cfg struct{}
	err := LoadYAML(&cfg)
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestLoad_NestedYAML(t *testing.T) {
	resetCache()
	defer resetCache()
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
	require.NoError(t, LoadYAML(&cfg))
	assert.Equal(t, "db.example.com", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "nats://localhost:4222", cfg.NATS.URL)
}

func TestLoadSecret_ReadsSecretYAML(t *testing.T) {
	resetCache()
	defer resetCache()
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

func TestLoadSecret_ErrorWhenFileMissing(t *testing.T) {
	resetCache()
	defer resetCache()
	tmp := t.TempDir()
	t.Setenv(envConfigDir, tmp)

	var secret struct{}
	err := LoadSecret(&secret)
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}
