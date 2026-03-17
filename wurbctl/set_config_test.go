package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zon/chat/core/k8s"
)

func TestSetConfigCmd_ConfigmapWithOIDCIssuer(t *testing.T) {
	cmd := &SetConfigCmd{
		OIDCIssuer: "https://issuer.example.com",
		Context:    "test-context",
	}

	data := map[string]string{
		"oidc-issuer": cmd.OIDCIssuer,
	}

	yaml := k8s.BuildConfigmapYAML("wurbs-config", wurbsNamespace, data)

	assert.Contains(t, yaml, "apiVersion: v1")
	assert.Contains(t, yaml, "kind: ConfigMap")
	assert.Contains(t, yaml, "name: wurbs-config")
	assert.Contains(t, yaml, "namespace: ralph-wurbs")
	assert.Contains(t, yaml, "oidc-issuer: \"https://issuer.example.com\"")
}

func TestSetConfigCmd_Constants(t *testing.T) {
	assert.Equal(t, "ralph-wurbs", wurbsNamespace)
	assert.Equal(t, "wurbs", postgresNamespace)
	assert.Equal(t, "nats", natsNamespace)
	assert.Equal(t, "wurbs-postgres-app", postgresSecret)
	assert.Equal(t, "nats-secrets", natsSecret)
	assert.Equal(t, "dev-token", natsTokenKey)
	assert.Equal(t, "32432", localPostgresPort)
	assert.Equal(t, "admin-test@test.com", testAdminEmail)
	assert.Equal(t, "test-admin", testAdminSecretName)
}
