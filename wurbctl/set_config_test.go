package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zon/chat/core/config"
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

	yaml := k8s.BuildConfigmapYAML("wurbs-config", ralphWorkflowNamespace, data)

	assert.Contains(t, yaml, "apiVersion: v1")
	assert.Contains(t, yaml, "kind: ConfigMap")
	assert.Contains(t, yaml, "name: wurbs-config")
	assert.Contains(t, yaml, "namespace: ralph-wurbs")
	assert.Contains(t, yaml, "oidc-issuer: \"https://issuer.example.com\"")
}

func TestSetConfigCmd_Constants(t *testing.T) {
	assert.Equal(t, "ralph-wurbs", ralphWorkflowNamespace)
	assert.Equal(t, "wurbs", wurbsNamespace)
	assert.Equal(t, "nats", natsNamespace)
	assert.Equal(t, "wurbs-postgres-app", postgresSecret)
	assert.Equal(t, "nats-secrets", natsReadSecret)
	assert.Equal(t, "dev-token", natsReadTokenKey)
	assert.Equal(t, "nats-dev-token", natsWriteSecret)
	assert.Equal(t, "32432", localPostgresPort)
	assert.Equal(t, "test-admin@example.com", testAdminEmail)
	assert.Equal(t, "test-admin", testAdminSecretName)
}

func TestSetConfigCmd_ConfigTreePaths(t *testing.T) {
	config.SetTestMode(true)
	defer config.SetTestMode(false)
	config.ResetCache()

	tree, err := config.RepoDir()
	assert.NoError(t, err)

	assert.Equal(t, "config.yaml", filepath.Base(tree.Config))
	assert.Equal(t, "nats-dev-token", filepath.Base(tree.NATSDevToken))
	assert.Equal(t, "test-admin.yaml", filepath.Base(tree.TestAdmin))
	assert.Equal(t, "postgres.json", filepath.Base(tree.Postgres))
}

func TestSetConfigCmd_NATSSecretYAML(t *testing.T) {
	token := "my-dev-token"
	yaml := k8s.BuildSecretYAML(natsWriteSecret, ralphWorkflowNamespace, map[string]string{natsReadTokenKey: token})

	assert.Contains(t, yaml, "kind: Secret")
	assert.Contains(t, yaml, "name: "+natsWriteSecret)
	assert.Contains(t, yaml, "namespace: "+ralphWorkflowNamespace)
	assert.Contains(t, yaml, "type: Opaque")
	assert.Contains(t, yaml, natsReadTokenKey+":")
}

func TestSetConfigCmd_TestAdminSecretYAML(t *testing.T) {
	yaml := k8s.BuildSecretYAML(testAdminSecretName, ralphWorkflowNamespace, map[string]string{
		"email":      "test-admin@example.com",
		"privateKey": "private-key-data",
		"publicKey":  "public-key-data",
	})

	assert.Contains(t, yaml, "kind: Secret")
	assert.Contains(t, yaml, "name: "+testAdminSecretName)
	assert.Contains(t, yaml, "namespace: "+ralphWorkflowNamespace)
	assert.Contains(t, yaml, "type: Opaque")
}
