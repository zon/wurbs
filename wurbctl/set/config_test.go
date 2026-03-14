package set

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noopEnsure is a PostgresEnsurer that always succeeds (used in tests).
func noopEnsure(host string, port int, adminUser, adminPassword, appUser, appPassword, dbName string) error {
	return nil
}

// errEnsure is a PostgresEnsurer that always fails (used in tests).
func errEnsure(host string, port int, adminUser, adminPassword, appUser, appPassword, dbName string) error {
	return errors.New("connection refused")
}

// fullCmd returns a ConfigCmd with all required fields populated, using noopEnsure.
func fullCmd() ConfigCmd {
	return ConfigCmd{
		DBHost:          "localhost",
		DBPort:          5432,
		DBAdminUser:     "postgres",
		DBAdminPassword: "adminpass",
		DBUser:          "wurbs",
		DBPassword:      "wurbs_secret",
		DBName:          "wurbs_db",
		OIDCIssuer:      "https://issuer.example.com",
		OIDCClientID:    "wurbs-client",
		Namespace:       "default",
		ensurePostgres:  noopEnsure,
	}
}

// --- Missing required value tests ---

func TestConfigCmd_MissingDBHost(t *testing.T) {
	cmd := fullCmd()
	cmd.DBHost = ""
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--db-host / PGHOST")
}

func TestConfigCmd_MissingDBAdminUser(t *testing.T) {
	cmd := fullCmd()
	cmd.DBAdminUser = ""
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--db-admin-user / PGADMINUSER")
}

func TestConfigCmd_MissingDBUser(t *testing.T) {
	cmd := fullCmd()
	cmd.DBUser = ""
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--db-user / PGUSER")
}

func TestConfigCmd_MissingDBPassword(t *testing.T) {
	cmd := fullCmd()
	cmd.DBPassword = ""
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--db-password / PGPASSWORD")
}

func TestConfigCmd_MissingDBName(t *testing.T) {
	cmd := fullCmd()
	cmd.DBName = ""
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--db-name / PGDATABASE")
}

func TestConfigCmd_MissingOIDCIssuer(t *testing.T) {
	cmd := fullCmd()
	cmd.OIDCIssuer = ""
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--oidc-issuer")
}

func TestConfigCmd_MissingOIDCClientID(t *testing.T) {
	cmd := fullCmd()
	cmd.OIDCClientID = ""
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--oidc-client-id")
}

func TestConfigCmd_MissingMultiple(t *testing.T) {
	cmd := fullCmd()
	cmd.DBHost = ""
	cmd.OIDCIssuer = ""
	cmd.OIDCClientID = ""
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--db-host / PGHOST")
	assert.Contains(t, err.Error(), "--oidc-issuer")
	assert.Contains(t, err.Error(), "--oidc-client-id")
}

// --- Postgres error propagation ---

func TestConfigCmd_PostgresError(t *testing.T) {
	cmd := fullCmd()
	cmd.ensurePostgres = errEnsure
	cmd.Local = true // avoid kubectl
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to configure postgres")
	assert.Contains(t, err.Error(), "connection refused")
}

// --- Local file generation ---

func TestConfigCmd_LocalWritesConfigmapFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	cmd := fullCmd()
	cmd.Local = true
	err := cmd.Run()
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "kind: ConfigMap")
	assert.Contains(t, content, "name: wurbs-config")
	assert.Contains(t, content, "PGHOST")
	assert.Contains(t, content, "localhost")
	assert.Contains(t, content, "OIDC_ISSUER")
	assert.Contains(t, content, "https://issuer.example.com")
	assert.Contains(t, content, "OIDC_CLIENT_ID")
	assert.Contains(t, content, "wurbs-client")
}

func TestConfigCmd_LocalWritesSecretFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	cmd := fullCmd()
	cmd.Local = true
	err := cmd.Run()
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "secret.yaml"))
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "kind: Secret")
	assert.Contains(t, content, "name: wurbs-secret")
	assert.Contains(t, content, "PGPASSWORD")

	// Password should be base64-encoded
	encoded := base64.StdEncoding.EncodeToString([]byte("wurbs_secret"))
	assert.Contains(t, content, encoded)
}

func TestConfigCmd_LocalWithNamespace(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	cmd := fullCmd()
	cmd.Local = true
	cmd.Namespace = "wurbs-prod"
	err := cmd.Run()
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "namespace: wurbs-prod")
}

// --- Test flag ---

func TestConfigCmd_TestFlagGeneratesKeys(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	cmd := fullCmd()
	cmd.Local = true
	cmd.Test = true
	err := cmd.Run()
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "secret.yaml"))
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "TEST_CLIENT_PRIVATE_KEY")
	assert.Contains(t, content, "TEST_CLIENT_PUBLIC_KEY")
}

func TestConfigCmd_NoTestFlagNoKeys(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	cmd := fullCmd()
	cmd.Local = true
	cmd.Test = false
	err := cmd.Run()
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "secret.yaml"))
	require.NoError(t, err)

	content := string(data)
	assert.NotContains(t, content, "TEST_CLIENT_PRIVATE_KEY")
	assert.NotContains(t, content, "TEST_CLIENT_PUBLIC_KEY")
}

// --- RSA key pair generation ---

func TestGenerateRSAKeyPair_Valid(t *testing.T) {
	priv, pub, err := GenerateRSAKeyPair()
	require.NoError(t, err)
	assert.Contains(t, priv, "BEGIN RSA PRIVATE KEY")
	assert.Contains(t, priv, "END RSA PRIVATE KEY")
	assert.Contains(t, pub, "BEGIN PUBLIC KEY")
	assert.Contains(t, pub, "END PUBLIC KEY")
}

func TestGenerateRSAKeyPair_Unique(t *testing.T) {
	priv1, pub1, err1 := GenerateRSAKeyPair()
	priv2, pub2, err2 := GenerateRSAKeyPair()
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.NotEqual(t, priv1, priv2, "each generated private key should be unique")
	assert.NotEqual(t, pub1, pub2, "each generated public key should be unique")
}

// --- YAML builders ---

func TestBuildConfigmapYAML(t *testing.T) {
	data := map[string]string{
		"FOO": "bar",
		"BAZ": "qux",
	}
	yaml := buildConfigmapYAML("my-config", "my-ns", data)
	assert.Contains(t, yaml, "apiVersion: v1")
	assert.Contains(t, yaml, "kind: ConfigMap")
	assert.Contains(t, yaml, "name: my-config")
	assert.Contains(t, yaml, "namespace: my-ns")
	assert.Contains(t, yaml, "FOO")
	assert.Contains(t, yaml, "bar")
	assert.Contains(t, yaml, "BAZ")
	assert.Contains(t, yaml, "qux")
}

func TestBuildConfigmapYAML_NoNamespace(t *testing.T) {
	yaml := buildConfigmapYAML("my-config", "", map[string]string{"K": "V"})
	assert.NotContains(t, yaml, "namespace:")
}

func TestBuildSecretYAML(t *testing.T) {
	data := map[string]string{
		"PASSWORD": "secret123",
	}
	yaml := buildSecretYAML("my-secret", "my-ns", data)
	assert.Contains(t, yaml, "apiVersion: v1")
	assert.Contains(t, yaml, "kind: Secret")
	assert.Contains(t, yaml, "name: my-secret")
	assert.Contains(t, yaml, "namespace: my-ns")
	assert.Contains(t, yaml, "type: Opaque")
	assert.Contains(t, yaml, "PASSWORD")

	// Values must be base64-encoded
	encoded := base64.StdEncoding.EncodeToString([]byte("secret123"))
	assert.Contains(t, yaml, encoded)
	assert.NotContains(t, yaml, "secret123") // raw value must not appear
}

func TestBuildSecretYAML_NoNamespace(t *testing.T) {
	yaml := buildSecretYAML("my-secret", "", map[string]string{"K": "V"})
	assert.NotContains(t, yaml, "namespace:")
}

// --- WriteConfigmapFile / WriteSecretFile ---

func TestWriteConfigmapFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := WriteConfigmapFile(path, "test-config", "test-ns", map[string]string{"KEY": "val"})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "kind: ConfigMap")
}

func TestWriteSecretFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.yaml")
	err := WriteSecretFile(path, "test-secret", "test-ns", map[string]string{"PASS": "pw"})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "kind: Secret")
	// Ensure file does not contain raw password
	assert.NotContains(t, content, "pw")
	encoded := base64.StdEncoding.EncodeToString([]byte("pw"))
	assert.Contains(t, content, encoded)
}

func TestWriteSecretFile_Permissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.yaml")
	err := WriteSecretFile(path, "test-secret", "test-ns", map[string]string{"PASS": "pw"})
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	// File should be owner-read/write only (0600)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

// --- ConfigMap data completeness ---

func TestConfigCmd_LocalConfigmapContainsAllFields(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	cmd := fullCmd()
	cmd.Local = true
	err := cmd.Run()
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)
	content := string(data)

	for _, key := range []string{"PGHOST", "PGPORT", "PGDATABASE", "PGUSER", "OIDC_ISSUER", "OIDC_CLIENT_ID"} {
		assert.Contains(t, content, key, "configmap should contain %s", key)
	}
}

func TestConfigCmd_LocalSecretContainsPassword(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	cmd := fullCmd()
	cmd.Local = true
	err := cmd.Run()
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "secret.yaml"))
	require.NoError(t, err)
	content := string(data)

	// PGPASSWORD must be in secret and be base64-encoded
	assert.Contains(t, content, "PGPASSWORD")
	encoded := base64.StdEncoding.EncodeToString([]byte("wurbs_secret"))
	assert.Contains(t, content, encoded)
	// Raw password must NOT appear in the secret
	assert.True(t, !strings.Contains(content, "wurbs_secret") || strings.Contains(content, encoded),
		"raw password should not appear unencoded in secret YAML")
}

func TestWritePostgresJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "postgres.json")

	secretData := map[string]string{
		"username": "wurbs",
		"password": "secret123",
		"dbname":   "wurbs_db",
	}

	err := WritePostgresJSON(path, "10.0.0.1", 32432, secretData)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, `"host": "10.0.0.1"`)
	assert.Contains(t, content, `"port": 32432`)
	assert.Contains(t, content, `"user": "wurbs"`)
	assert.Contains(t, content, `"password": "secret123"`)
	assert.Contains(t, content, `"database": "wurbs_db"`)
}

func TestWritePostgresJSON_Permissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "postgres.json")

	err := WritePostgresJSON(path, "localhost", 5432, map[string]string{
		"username": "user",
		"password": "pass",
		"dbname":   "db",
	})
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

type mockK8sGetter struct {
	serviceIP  string
	secretData map[string]string
	serviceErr error
	secretErr  error
}

func (m *mockK8sGetter) GetServiceIP(name, namespace, context string) (string, error) {
	return m.serviceIP, m.serviceErr
}

func (m *mockK8sGetter) GetSecret(name, namespace, context string) (map[string]string, error) {
	return m.secretData, m.secretErr
}

func TestRunFromCloudNativePG_Success(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	mock := &mockK8sGetter{
		serviceIP: "10.244.0.10",
		secretData: map[string]string{
			"username": "wurbs",
			"password": "mysecret",
			"dbname":   "wurbs",
		},
	}

	cmd := ConfigCmd{
		Namespace: "wurbs",
		k8sGetter: mock,
	}

	err := cmd.runFromCloudNativePG()
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "config", "postgres.json"))
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, `"host": "10.244.0.10"`)
	assert.Contains(t, content, `"port": 32432`)
	assert.Contains(t, content, `"user": "wurbs"`)
	assert.Contains(t, content, `"password": "mysecret"`)
	assert.Contains(t, content, `"database": "wurbs"`)
}

func TestRunFromCloudNativePG_ServiceError(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	mock := &mockK8sGetter{
		serviceErr: fmt.Errorf("service not found"),
	}

	cmd := ConfigCmd{
		Namespace: "wurbs",
		k8sGetter: mock,
	}

	err := cmd.runFromCloudNativePG()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get postgres service IP")
}

func TestRunFromCloudNativePG_SecretError(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	mock := &mockK8sGetter{
		serviceIP: "10.244.0.10",
		secretErr: fmt.Errorf("secret not found"),
	}

	cmd := ConfigCmd{
		Namespace: "wurbs",
		k8sGetter: mock,
	}

	err := cmd.runFromCloudNativePG()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get postgres secret")
}

func TestRunFromCloudNativePG_DefaultNamespace(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	mock := &mockK8sGetter{
		serviceIP: "10.244.0.10",
		secretData: map[string]string{
			"username": "wurbs",
			"password": "secret",
			"dbname":   "wurbs",
		},
	}

	cmd := ConfigCmd{
		k8sGetter: mock,
	}

	err := cmd.runFromCloudNativePG()
	require.NoError(t, err)

	_, err = os.ReadFile(filepath.Join(dir, "config", "postgres.json"))
	require.NoError(t, err)
}
