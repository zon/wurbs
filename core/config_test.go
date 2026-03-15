package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetConfigDir(t *testing.T) {
	tests := []struct {
		name             string
		wurbsConfigEnv   string
		workingDir       string
		expectedContains string
		wantErr          bool
	}{
		{
			name:             "uses WURBS_CONFIG when set",
			wurbsConfigEnv:   "/custom/config",
			workingDir:       "/workspace/repo",
			expectedContains: "/custom/config",
		},
		{
			name:             "defaults to ./config relative to git root",
			wurbsConfigEnv:   "",
			workingDir:       "/workspace/repo/subdir",
			expectedContains: "/workspace/repo/config",
		},
		{
			name:           "returns error when no git repo and no env var",
			wurbsConfigEnv: "",
			workingDir:     "/tmp/nonexistent",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wurbsConfigEnv != "" {
				os.Setenv("WURBS_CONFIG", tt.wurbsConfigEnv)
				defer os.Unsetenv("WURBS_CONFIG")
			} else {
				os.Unsetenv("WURBS_CONFIG")
			}

			dir, err := GetConfigDir(tt.workingDir)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedContains, dir)
		})
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `
server:
  host: "0.0.0.0"
  port: 8080
database:
  type: postgres
nats:
  url: "nats://localhost:4222"
`
	secretsContent := `
database:
  password: "secretpassword"
  user: "admin"
nats:
  token: "natssecret"
oidc:
  client_secret: "oidcsecret"
`

	err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(configContent), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "secret.yaml"), []byte(secretsContent), 0644)
	require.NoError(t, err)

	cfg, secrets, err := LoadConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "postgres", cfg.Database.Type)
	assert.Equal(t, "nats://localhost:4222", cfg.NATS.URL)

	assert.Equal(t, "secretpassword", secrets.Database.Password)
	assert.Equal(t, "admin", secrets.Database.User)
	assert.Equal(t, "natssecret", secrets.NATS.Token)
	assert.Equal(t, "oidcsecret", secrets.OIDC.ClientSecret)
}

func TestLoadConfigMissingFiles(t *testing.T) {
	tmpDir := t.TempDir()

	_, _, err := LoadConfig(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config.yaml")

	err = os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("server:\n  port: 8080\n"), 0644)
	require.NoError(t, err)

	_, _, err = LoadConfig(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secret.yaml")
}

func TestFindGitRoot(t *testing.T) {
	gitRoot, err := FindGitRoot("/workspace/repo/subdir/nested")
	require.NoError(t, err)
	assert.Equal(t, "/workspace/repo", gitRoot)

	_, err = FindGitRoot("/tmp/nonexistent")
	assert.Error(t, err)
}
