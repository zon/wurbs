package config

import (
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
