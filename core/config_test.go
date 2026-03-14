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
		wurbConfigEnv    string
		testMode         bool
		workingDir       string
		expectedContains string
		wantErr          bool
	}{
		{
			name:             "uses WURB_CONFIG when set",
			wurbConfigEnv:    "/custom/config",
			testMode:         false,
			expectedContains: "/custom/config",
		},
		{
			name:             "defaults to /etc/wurbs when no env and not test mode",
			wurbConfigEnv:    "",
			testMode:         false,
			expectedContains: "/etc/wurbs",
		},
		{
			name:             "finds git root config in test mode",
			wurbConfigEnv:    "",
			testMode:         true,
			workingDir:       "/workspace/repo/subdir",
			expectedContains: "/workspace/repo/config",
		},
		{
			name:             "falls back to /etc/wurbs in test mode when no git repo",
			wurbConfigEnv:    "",
			testMode:         true,
			workingDir:       "/tmp/nonexistent",
			expectedContains: "/etc/wurbs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wurbConfigEnv != "" {
				os.Setenv("WURB_CONFIG", tt.wurbConfigEnv)
				defer os.Unsetenv("WURB_CONFIG")
			} else {
				os.Unsetenv("WURB_CONFIG")
			}

			dir, err := GetConfigDir(tt.testMode, tt.workingDir)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, dir, tt.expectedContains)
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
