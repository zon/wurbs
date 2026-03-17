package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigMap_Fields(t *testing.T) {
	cm := ConfigMap{
		RESTPort:   8080,
		SocketPort: 9000,
		OIDCIssuer: "https://issuer.example.com",
		NATSURL:    "nats://localhost:4222",
	}
	assert.Equal(t, 8080, cm.RESTPort)
	assert.Equal(t, 9000, cm.SocketPort)
	assert.Equal(t, "https://issuer.example.com", cm.OIDCIssuer)
	assert.Equal(t, "nats://localhost:4222", cm.NATSURL)
}

func TestConfigMap_Load(t *testing.T) {
	resetCache()
	defer resetCache()
	tmp := t.TempDir()
	content := "restPort: 8080\nsocketPort: 9000\noidcIssuer: https://issuer.example.com\nnatsURL: nats://localhost:4222\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "config.yaml"), []byte(content), 0644))

	t.Setenv(envConfigDir, tmp)

	var cm ConfigMap
	require.NoError(t, cm.Load())
	assert.Equal(t, 8080, cm.RESTPort)
	assert.Equal(t, 9000, cm.SocketPort)
	assert.Equal(t, "https://issuer.example.com", cm.OIDCIssuer)
	assert.Equal(t, "nats://localhost:4222", cm.NATSURL)
}

func TestConfigMap_Write(t *testing.T) {
	resetCache()
	defer resetCache()
	tmp := t.TempDir()
	t.Setenv(envConfigDir, tmp)

	cm := &ConfigMap{
		RESTPort:   8080,
		SocketPort: 9000,
		OIDCIssuer: "https://issuer.example.com",
		NATSURL:    "nats://localhost:4222",
	}
	require.NoError(t, cm.Write())

	var loaded ConfigMap
	require.NoError(t, LoadYAML(&loaded))
	assert.Equal(t, 8080, loaded.RESTPort)
	assert.Equal(t, 9000, loaded.SocketPort)
	assert.Equal(t, "https://issuer.example.com", loaded.OIDCIssuer)
	assert.Equal(t, "nats://localhost:4222", loaded.NATSURL)
}

func TestConfigMap_MarshalConfigMap(t *testing.T) {
	cm := ConfigMap{
		RESTPort:   8080,
		SocketPort: 9000,
		OIDCIssuer: "https://issuer.example.com",
		NATSURL:    "nats://localhost:4222",
	}
	data, err := cm.MarshalConfigMap()
	require.NoError(t, err)
	assert.Contains(t, data, "config.yaml")
	assert.Contains(t, data["config.yaml"], "restPort: 8080")
	assert.Contains(t, data["config.yaml"], "socketPort: 9000")
	assert.Contains(t, data["config.yaml"], "oidcIssuer: https://issuer.example.com")
	assert.Contains(t, data["config.yaml"], "natsURL: nats://localhost:4222")
}
