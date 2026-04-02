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

func TestConfigMap_WriteToK8s(t *testing.T) {
	original := applyConfigmapFunc
	defer func() { applyConfigmapFunc = original }()

	var called bool
	var calledName, calledNamespace, calledContext string
	var calledData map[string]string
	applyConfigmapFunc = func(name, namespace, context string, data map[string]string) error {
		called = true
		calledName = name
		calledNamespace = namespace
		calledContext = context
		calledData = data
		return nil
	}

	cm := ConfigMap{
		RESTPort:   8080,
		SocketPort: 9000,
		OIDCIssuer: "https://issuer.example.com",
		NATSURL:    "nats://localhost:4222",
	}
	err := cm.WriteToK8s("test-context")
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "wurbs", calledName)
	assert.Equal(t, "ralph-wurbs", calledNamespace)
	assert.Equal(t, "test-context", calledContext)
	require.NotNil(t, calledData)
	assert.Contains(t, calledData, "config.yaml")
	assert.Contains(t, calledData["config.yaml"], "restPort: 8080")
	assert.Contains(t, calledData["config.yaml"], "socketPort: 9000")
	assert.Contains(t, calledData["config.yaml"], "oidcIssuer: https://issuer.example.com")
	assert.Contains(t, calledData["config.yaml"], "natsURL: nats://localhost:4222")
}
