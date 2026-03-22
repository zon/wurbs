package config

import (
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
