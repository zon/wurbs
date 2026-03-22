package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Fields(t *testing.T) {
	cfg := Config{
		RESTPort:   8080,
		SocketPort: 9000,
		OIDCIssuer: "https://issuer.example.com",
		NATSURL:    "nats://localhost:4222",
	}
	assert.Equal(t, 8080, cfg.RESTPort)
	assert.Equal(t, 9000, cfg.SocketPort)
	assert.Equal(t, "https://issuer.example.com", cfg.OIDCIssuer)
	assert.Equal(t, "nats://localhost:4222", cfg.NATSURL)
}

func TestLoad_ReturnsConfig(t *testing.T) {
	resetCache()
	defer resetCache()
	tmp := t.TempDir()
	content := "restPort: 8080\nsocketPort: 9000\noidcIssuer: https://issuer.example.com\nnatsURL: nats://localhost:4222\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "config.yaml"), []byte(content), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "nats-dev-token"), []byte("dev-token-123"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "test-admin.yaml"), []byte("admin-user:admin"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "postgres.json"), []byte(`{"username":"user","password":"pass"}`), 0644))

	t.Setenv(envConfigDir, tmp)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.RESTPort)
	assert.Equal(t, 9000, cfg.SocketPort)
	assert.Equal(t, "https://issuer.example.com", cfg.OIDCIssuer)
	assert.Equal(t, "nats://localhost:4222", cfg.NATSURL)
	assert.Equal(t, "dev-token-123", cfg.NATSDevToken)
	assert.Equal(t, "admin-user:admin", cfg.TestAdmin)
	assert.Equal(t, `{"username":"user","password":"pass"}`, cfg.Postgres)
}

func TestLoad_ConfigFieldsFromConfigMap(t *testing.T) {
	resetCache()
	defer resetCache()
	tmp := t.TempDir()
	content := "restPort: 8080\nsocketPort: 9000\noidcIssuer: https://issuer.example.com\nnatsURL: nats://localhost:4222\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "config.yaml"), []byte(content), 0644))

	t.Setenv(envConfigDir, tmp)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.RESTPort)
	assert.Equal(t, 9000, cfg.SocketPort)
	assert.Equal(t, "https://issuer.example.com", cfg.OIDCIssuer)
	assert.Equal(t, "nats://localhost:4222", cfg.NATSURL)
	assert.Equal(t, "", cfg.NATSDevToken)
	assert.Equal(t, "", cfg.TestAdmin)
	assert.Equal(t, "", cfg.Postgres)
}

func TestLoad_NATSDevTokenFromFile(t *testing.T) {
	resetCache()
	defer resetCache()
	tmp := t.TempDir()
	content := "restPort: 8080\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "config.yaml"), []byte(content), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "nats-dev-token"), []byte("token-value"), 0600))

	t.Setenv(envConfigDir, tmp)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "token-value", cfg.NATSDevToken)
}

func TestLoad_TestAdminFromFile(t *testing.T) {
	resetCache()
	defer resetCache()
	tmp := t.TempDir()
	content := "restPort: 8080\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "config.yaml"), []byte(content), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "test-admin.yaml"), []byte("testadmin:password"), 0644))

	t.Setenv(envConfigDir, tmp)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "testadmin:password", cfg.TestAdmin)
}

func TestLoad_PostgresFromFile(t *testing.T) {
	resetCache()
	defer resetCache()
	tmp := t.TempDir()
	content := "restPort: 8080\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "config.yaml"), []byte(content), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "postgres.json"), []byte(`{"username":"dbuser","password":"dbpass"}`), 0644))

	t.Setenv(envConfigDir, tmp)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, `{"username":"dbuser","password":"dbpass"}`, cfg.Postgres)
}

func TestLoad_ErrorWhenConfigYamlMissing(t *testing.T) {
	resetCache()
	defer resetCache()
	tmp := t.TempDir()
	t.Setenv(envConfigDir, tmp)

	_, err := Load()
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestWrite_WritesConfigYAML(t *testing.T) {
	resetCache()
	defer resetCache()
	tmp := t.TempDir()
	t.Setenv(envConfigDir, tmp)

	cfg := &Config{
		RESTPort:   8080,
		SocketPort: 8081,
		OIDCIssuer: "https://issuer.example.com",
		NATSURL:    "nats://localhost:4222",
	}
	require.NoError(t, Write(cfg))

	var loaded Config
	require.NoError(t, LoadYAML(&loaded))
	assert.Equal(t, 8080, loaded.RESTPort)
	assert.Equal(t, 8081, loaded.SocketPort)
	assert.Equal(t, "https://issuer.example.com", loaded.OIDCIssuer)
	assert.Equal(t, "nats://localhost:4222", loaded.NATSURL)
}

func TestWrite_ErrorWhenDirNotWritable(t *testing.T) {
	resetCache()
	defer resetCache()
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, "nonexistent")
	t.Setenv(envConfigDir, configDir)

	err := Write(&Config{RESTPort: 8080})
	assert.Error(t, err)
}
