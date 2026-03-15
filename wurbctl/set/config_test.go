package set

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockLoadSecret(name, namespace, context string) (map[string]string, error) {
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

func fullCmd() ConfigCmd {
	return ConfigCmd{
		ClusterIP:  "10.96.0.1",
		Context:    "test-context",
		Namespace:  "wurbs",
		OIDCIssuer: "https://issuer.example.com",
		loadSecret: mockLoadSecret,
	}
}

func TestConfigCmd_MissingClusterIP(t *testing.T) {
	cmd := ConfigCmd{
		ClusterIP:  "",
		loadSecret: mockLoadSecret,
	}
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--cluster-ip")
}

func TestConfigCmd_InvalidClusterIP(t *testing.T) {
	cmd := ConfigCmd{
		ClusterIP:  "invalid-ip",
		loadSecret: mockLoadSecret,
	}
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid cluster IP")
}

func TestConfigCmd_LoadSecretError(t *testing.T) {
	cmd := ConfigCmd{
		ClusterIP:  "10.96.0.1",
		loadSecret: mockLoadSecretFail,
	}
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load secret")
}

func TestConfigCmd_WritesPostgresConfig(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("WURBS_CONFIG", dir)
	defer os.Unsetenv("WURBS_CONFIG")

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
	os.Setenv("WURBS_CONFIG", dir)
	defer os.Unsetenv("WURBS_CONFIG")

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
	var capturedNamespace string
	cmd := ConfigCmd{
		ClusterIP: "10.96.0.1",
		Namespace: "wurbs",
		loadSecret: func(name, namespace, context string) (map[string]string, error) {
			capturedNamespace = namespace
			return mockLoadSecret(name, namespace, context)
		},
	}
	err := cmd.Run()
	require.NoError(t, err)
	assert.Equal(t, "wurbs", capturedNamespace)
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
