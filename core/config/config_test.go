package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigDir_WithEnvVar(t *testing.T) {
	os.Setenv("WURBS_CONFIG", "/custom/config/path")
	defer os.Unsetenv("WURBS_CONFIG")

	dir, err := ConfigDir()
	require.NoError(t, err)
	assert.Equal(t, "/custom/config/path", dir)
}

func TestConfigDir_WithoutEnvVar(t *testing.T) {
	os.Unsetenv("WURBS_CONFIG")

	dir, err := ConfigDir()
	require.NoError(t, err)

	repoRoot, err := findRepoRoot()
	require.NoError(t, err)
	expected := filepath.Join(repoRoot, "config")
	assert.Equal(t, expected, dir)
}
