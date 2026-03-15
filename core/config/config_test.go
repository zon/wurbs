package config

import (
	"os"
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

func TestFindGitRoot(t *testing.T) {
	gitRoot, err := FindGitRoot("/workspace/repo/subdir/nested")
	require.NoError(t, err)
	assert.Equal(t, "/workspace/repo", gitRoot)

	_, err = FindGitRoot("/tmp/nonexistent")
	assert.Error(t, err)
}
