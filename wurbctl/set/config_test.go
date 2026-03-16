package set

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zon/chat/core/config"
)

func setConfigDir(t *testing.T, dir string) {
	t.Helper()
	config.ResetCache()
	os.Setenv("WURB_CONFIG", dir)
	t.Cleanup(func() {
		os.Unsetenv("WURB_CONFIG")
		config.ResetCache()
	})
}

func mockLoadSecret(name, namespace, context string) (map[string]string, error) {
	if name == natsSecret {
		return map[string]string{
			"dev-token": "nats-dev-token-value",
		}, nil
	}
	return map[string]string{
		"username":      "wurbs",
		"password":      "wurbs_secret",
		"dbname":        "wurbs_db",
		"host":          "postgres.wurbs.svc.cluster.local",
		"port":          "5432",
		"uri":           "postgresql://wurbs:wurbs_secret@postgres.wurbs.svc.cluster.local:5432/wurbs_db",
		"pgpass":        "wurbs_secret",
		"jdbc-uri":      "jdbc:postgresql://postgres.wurbs.svc.cluster.local:5432/wurbs_db",
		"fqdn-uri":      "postgresql://wurbs:wurbs_secret@postgres.wurbs.svc.cluster.local:5432/wurbs_db",
		"fqdn-jdbc-uri": "jdbc:postgresql://postgres.wurbs.svc.cluster.local:5432/wurbs_db",
	}, nil
}

func mockLoadSecretFail(name, namespace, context string) (map[string]string, error) {
	return nil, errors.New("secret not found")
}

func mockLoadClusterIP(context string) (string, error) {
	return "10.96.0.1", nil
}

func mockLoadClusterIPFail(context string) (string, error) {
	return "", errors.New("kubectl config view failed")
}

func fullCmd() ConfigCmd {
	return ConfigCmd{
		ClusterIP:  "10.96.0.1",
		Context:    "test-context",
		Namespace:  "wurbs",
		Local:      true,
		OIDCIssuer: "https://issuer.example.com",
		loadSecret: mockLoadSecret,
	}
}

func TestConfigCmd_AutoDetectClusterIP(t *testing.T) {
	dir := t.TempDir()
	setConfigDir(t, dir)

	cmd := ConfigCmd{
		Namespace:     "wurbs",
		Local:         true,
		OIDCIssuer:    "https://issuer.example.com",
		loadSecret:    mockLoadSecret,
		loadClusterIP: mockLoadClusterIP,
	}
	err := cmd.Run()
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "postgres.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"host": "10.96.0.1"`)
}

func TestConfigCmd_AutoDetectClusterIPError(t *testing.T) {
	cmd := ConfigCmd{
		Local:         true,
		OIDCIssuer:    "https://issuer.example.com",
		loadSecret:    mockLoadSecret,
		loadClusterIP: mockLoadClusterIPFail,
	}
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get cluster IP from kubectl context")
}

func TestConfigCmd_InvalidClusterIP(t *testing.T) {
	cmd := ConfigCmd{
		ClusterIP:  "invalid-ip",
		Local:      true,
		OIDCIssuer: "https://issuer.example.com",
		loadSecret: mockLoadSecret,
	}
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid cluster IP")
}

func TestConfigCmd_LoadSecretError(t *testing.T) {
	cmd := ConfigCmd{
		ClusterIP:  "10.96.0.1",
		Local:      true,
		OIDCIssuer: "https://issuer.example.com",
		loadSecret: mockLoadSecretFail,
	}
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load NATS secret")
}

func TestConfigCmd_WritesPostgresConfig(t *testing.T) {
	dir := t.TempDir()
	setConfigDir(t, dir)

	cmd := fullCmd()
	err := cmd.Run()
	require.NoError(t, err)

	postgresConfigPath := filepath.Join(dir, "postgres.json")
	data, err := os.ReadFile(postgresConfigPath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, `"host": "10.96.0.1"`)
	assert.Contains(t, content, `"port": "32432"`)
	assert.Contains(t, content, `"username": "wurbs"`)
	assert.Contains(t, content, `"dbname": "wurbs_db"`)
}

func TestConfigCmd_PatchesURIFields(t *testing.T) {
	dir := t.TempDir()
	setConfigDir(t, dir)

	cmd := fullCmd()
	err := cmd.Run()
	require.NoError(t, err)

	postgresConfigPath := filepath.Join(dir, "postgres.json")
	data, err := os.ReadFile(postgresConfigPath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, `"uri": "postgresql://wurbs:wurbs_secret@10.96.0.1:32432/wurbs_db"`)
	assert.Contains(t, content, `"jdbc-uri": "jdbc:postgresql://10.96.0.1:32432/wurbs_db"`)
	assert.Contains(t, content, `"fqdn-uri": "postgresql://wurbs:wurbs_secret@10.96.0.1:32432/wurbs_db"`)
	assert.Contains(t, content, `"fqdn-jdbc-uri": "jdbc:postgresql://10.96.0.1:32432/wurbs_db"`)
}

func TestConfigCmd_DefaultNamespace(t *testing.T) {
	dir := t.TempDir()
	setConfigDir(t, dir)

	var capturedNamespace string
	cmd := ConfigCmd{
		ClusterIP:  "10.96.0.1",
		Local:      true,
		OIDCIssuer: "https://issuer.example.com",
		Namespace:  "wurbs",
		loadSecret: func(name, namespace, context string) (map[string]string, error) {
			capturedNamespace = namespace
			return mockLoadSecret(name, namespace, context)
		},
	}
	err := cmd.Run()
	require.NoError(t, err)
	assert.Equal(t, "ralph-wurbs", capturedNamespace)
}

func TestConfigCmd_WritesConfigYaml(t *testing.T) {
	dir := t.TempDir()
	setConfigDir(t, dir)

	cmd := fullCmd()
	err := cmd.Run()
	require.NoError(t, err)

	configPath := filepath.Join(dir, "config.yaml")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "oidc-issuer: https://issuer.example.com")
}

func TestConfigCmd_WritesNatsToken(t *testing.T) {
	dir := t.TempDir()
	setConfigDir(t, dir)

	cmd := fullCmd()
	err := cmd.Run()
	require.NoError(t, err)

	natsTokenPath := filepath.Join(dir, "nats-token")
	data, err := os.ReadFile(natsTokenPath)
	require.NoError(t, err)

	assert.Equal(t, "nats-dev-token-value", string(data))
}

func TestConfigCmd_WritesConfigmapFile(t *testing.T) {
	dir := t.TempDir()
	setConfigDir(t, dir)

	cmd := fullCmd()
	err := cmd.Run()
	require.NoError(t, err)

	configMapPath := filepath.Join(dir, "configmap.yaml")
	data, err := os.ReadFile(configMapPath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "kind: ConfigMap")
	assert.Contains(t, content, "name: wurbs-config")
	assert.Contains(t, content, "namespace: ralph-wurbs")
	assert.Contains(t, content, `oidc-issuer: "https://issuer.example.com"`)
}

func TestConfigCmd_WritesNatsSecretFile(t *testing.T) {
	dir := t.TempDir()
	setConfigDir(t, dir)

	cmd := fullCmd()
	err := cmd.Run()
	require.NoError(t, err)

	natsSecretPath := filepath.Join(dir, "nats-secret.yaml")
	data, err := os.ReadFile(natsSecretPath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "kind: Secret")
	assert.Contains(t, content, "name: nats-dev-token")
	assert.Contains(t, content, "namespace: ralph-wurbs")
	assert.Contains(t, content, "dev-token:")
}

func TestConfigCmd_WritesPostgresSecretFile(t *testing.T) {
	dir := t.TempDir()
	setConfigDir(t, dir)

	cmd := fullCmd()
	err := cmd.Run()
	require.NoError(t, err)

	postgresSecretPath := filepath.Join(dir, "postgres-secret.yaml")
	data, err := os.ReadFile(postgresSecretPath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "kind: Secret")
	assert.Contains(t, content, "name: wurbs-postgres-app")
	assert.Contains(t, content, "namespace: ralph-wurbs")
	assert.Contains(t, content, "host: cG9zdGdyZXMucmFscGgtd3VyYnMuc3ZjLmNsdXN0ZXIubG9jYWw=")
}

func TestConfigCmd_PostgresSecretContainsJSON(t *testing.T) {
	dir := t.TempDir()
	setConfigDir(t, dir)

	cmd := fullCmd()
	err := cmd.Run()
	require.NoError(t, err)

	postgresSecretPath := filepath.Join(dir, "postgres-secret.yaml")
	data, err := os.ReadFile(postgresSecretPath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "postgres.json:")
	assert.Contains(t, content, "username")
	assert.Contains(t, content, "password")
}

func TestConfigCmd_LoadsNatsSecretFromNatsNamespace(t *testing.T) {
	dir := t.TempDir()
	setConfigDir(t, dir)

	var capturedNatsNamespace string
	cmd := ConfigCmd{
		ClusterIP:  "10.96.0.1",
		Local:      true,
		OIDCIssuer: "https://issuer.example.com",
		loadSecret: func(name, namespace, context string) (map[string]string, error) {
			if name == natsSecret {
				capturedNatsNamespace = namespace
			}
			return mockLoadSecret(name, namespace, context)
		},
	}
	err := cmd.Run()
	require.NoError(t, err)
	assert.Equal(t, "nats", capturedNatsNamespace)
}

func TestPatchURI(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		newHost  string
		newPort  string
		expected string
	}{
		{
			name:     "simple URI",
			uri:      "postgresql://user:pass@host:5432/db",
			newHost:  "10.96.0.1",
			newPort:  "32432",
			expected: "postgresql://user:pass@10.96.0.1:32432/db",
		},
		{
			name:     "JDBC URI",
			uri:      "jdbc:postgresql://host:5432/db",
			newHost:  "10.96.0.1",
			newPort:  "32432",
			expected: "jdbc:postgresql://10.96.0.1:32432/db",
		},
		{
			name:     "empty URI",
			uri:      "",
			newHost:  "10.96.0.1",
			newPort:  "32432",
			expected: "",
		},
		{
			name:     "port already patched",
			uri:      "postgresql://user:pass@host:32432/db",
			newHost:  "10.96.0.1",
			newPort:  "32432",
			expected: "postgresql://user:pass@10.96.0.1:32432/db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := patchURI(tt.uri, tt.newHost, tt.newPort)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidIP(t *testing.T) {
	tests := []struct {
		ip    string
		valid bool
	}{
		{"10.96.0.1", true},
		{"192.168.1.1", true},
		{"127.0.0.1", true},
		{"invalid", false},
		{"", false},
		{"10.96.0.1.1", false},
		{"0.0.0.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			result := isValidIP(tt.ip)
			assert.Equal(t, tt.valid, result)
		})
	}
}

func TestGetSecret(t *testing.T) {
	secretData := map[string]string{
		"username": "wurbs",
		"password": "secret",
		"dbname":   "wurbs_db",
		"host":     "postgres",
		"port":     "5432",
		"uri":      "postgresql://wurbs:secret@postgres:5432/wurbs_db",
	}

	assert.Equal(t, "wurbs", secretData["username"])
	assert.Equal(t, "secret", secretData["password"])
}
