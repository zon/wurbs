package yaml

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildConfigmapYAML(t *testing.T) {
	data := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	yaml := BuildConfigmapYAML("my-config", "default", data)

	assert.Contains(t, yaml, "apiVersion: v1")
	assert.Contains(t, yaml, "kind: ConfigMap")
	assert.Contains(t, yaml, "name: my-config")
	assert.Contains(t, yaml, "namespace: default")
	assert.Contains(t, yaml, "key1: \"value1\"")
	assert.Contains(t, yaml, "key2: \"value2\"")
}

func TestBuildConfigmapYAML_NoNamespace(t *testing.T) {
	data := map[string]string{"key1": "value1"}

	yaml := BuildConfigmapYAML("my-config", "", data)

	assert.Contains(t, yaml, "name: my-config")
	assert.NotContains(t, yaml, "namespace:")
}

func TestBuildSecretYAML(t *testing.T) {
	data := map[string]string{
		"username": "admin",
		"password": "secret123",
	}

	yaml := BuildSecretYAML("my-secret", "default", data)

	assert.Contains(t, yaml, "apiVersion: v1")
	assert.Contains(t, yaml, "kind: Secret")
	assert.Contains(t, yaml, "name: my-secret")
	assert.Contains(t, yaml, "namespace: default")
	assert.Contains(t, yaml, "type: Opaque")

	encodedUsername := base64.StdEncoding.EncodeToString([]byte("admin"))
	encodedPassword := base64.StdEncoding.EncodeToString([]byte("secret123"))
	assert.Contains(t, yaml, "username: "+encodedUsername)
	assert.Contains(t, yaml, "password: "+encodedPassword)
}

func TestBuildSecretYAML_NoNamespace(t *testing.T) {
	data := map[string]string{"key": "value"}

	yaml := BuildSecretYAML("my-secret", "", data)

	assert.Contains(t, yaml, "name: my-secret")
	assert.NotContains(t, yaml, "namespace:")
}
