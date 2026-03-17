package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDir_EnvVarOverride(t *testing.T) {
	resetCache()
	defer resetCache()
	t.Setenv(envConfigDir, "/custom/config/path")
	tree, err := Dir()
	require.NoError(t, err)
	assert.Equal(t, "/custom/config/path", tree.Parent)
}

func TestDir_DefaultsToEtcWurbs(t *testing.T) {
	resetCache()
	defer resetCache()
	t.Setenv(envConfigDir, "")
	SetTestMode(false)
	defer SetTestMode(false)

	tree, err := Dir()
	require.NoError(t, err)
	assert.Equal(t, defaultConfigDir, tree.Parent)
}

func TestDir_TestModeFindsRepoConfig(t *testing.T) {
	resetCache()
	defer resetCache()
	// Create a fake repo with .git dir and config dir.
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "myrepo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755))
	configDir := filepath.Join(repoRoot, "config")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	// Subdir to start the walk from.
	subdir := filepath.Join(repoRoot, "a", "b")
	require.NoError(t, os.MkdirAll(subdir, 0755))

	t.Setenv(envConfigDir, "")
	SetTestMode(true)
	defer SetTestMode(false)

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(subdir))
	defer os.Chdir(origDir)

	tree, err := Dir()
	require.NoError(t, err)
	assert.Equal(t, configDir, tree.Parent)
}

func TestDir_TestModeFallsBackWhenNoConfigDir(t *testing.T) {
	resetCache()
	defer resetCache()
	// Repo root exists but has no ./config directory.
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "myrepo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, ".git"), 0755))

	t.Setenv(envConfigDir, "")
	SetTestMode(true)
	defer SetTestMode(false)

	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(repoRoot))
	defer os.Chdir(origDir)

	tree, err := Dir()
	require.NoError(t, err)
	assert.Equal(t, defaultConfigDir, tree.Parent)
}

func TestDir_EnvVarTakesPrecedenceOverTestMode(t *testing.T) {
	resetCache()
	defer resetCache()
	t.Setenv(envConfigDir, "/override")
	SetTestMode(true)
	defer SetTestMode(false)

	tree, err := Dir()
	require.NoError(t, err)
	assert.Equal(t, "/override", tree.Parent)
}

func TestDir_CachesResult(t *testing.T) {
	resetCache()
	defer resetCache()
	t.Setenv(envConfigDir, "/first/path")

	tree1, err := Dir()
	require.NoError(t, err)
	assert.Equal(t, "/first/path", tree1.Parent)

	t.Setenv(envConfigDir, "/second/path")

	tree2, err := Dir()
	require.NoError(t, err)
	assert.Equal(t, "/first/path", tree2.Parent, "should return cached value")
}

func TestEnvVarName(t *testing.T) {
	assert.Equal(t, "WURB_CONFIG", envConfigDir)
}

func TestDefaultDir(t *testing.T) {
	assert.Equal(t, "/etc/wurbs", defaultConfigDir)
}
