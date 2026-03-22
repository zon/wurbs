package k8s

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestParseK8sSecretJSON(t *testing.T) {
	secretData := map[string]string{
		"username": base64.StdEncoding.EncodeToString([]byte("admin")),
		"password": base64.StdEncoding.EncodeToString([]byte("s3cr3t")),
	}
	secretJSON, err := json.Marshal(map[string]interface{}{
		"data": secretData,
	})
	require.NoError(t, err)

	result, err := parseK8sSecretJSON(secretJSON)
	require.NoError(t, err)

	assert.Equal(t, "admin", result["username"])
	assert.Equal(t, "s3cr3t", result["password"])
}

func TestParseK8sSecretJSON_InvalidBase64(t *testing.T) {
	secretData := map[string]string{
		"invalid": "not-valid-base64!!!",
	}
	secretJSON, err := json.Marshal(map[string]interface{}{
		"data": secretData,
	})
	require.NoError(t, err)

	result, err := parseK8sSecretJSON(secretJSON)
	require.NoError(t, err)

	assert.Empty(t, result["invalid"])
	assert.Equal(t, 0, len(result))
}

func TestParseK8sSecretJSON_InvalidJSON(t *testing.T) {
	invalidJSON := []byte("not json at all")

	result, err := parseK8sSecretJSON(invalidJSON)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestParseK8sSecretJSON_EmptyData(t *testing.T) {
	secretJSON := []byte(`{"data": {}}`)

	result, err := parseK8sSecretJSON(secretJSON)
	require.NoError(t, err)
	assert.Empty(t, result)
}
