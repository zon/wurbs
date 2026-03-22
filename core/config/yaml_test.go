package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad_ErrorWhenFileMissing(t *testing.T) {
	resetCache()
	defer resetCache()
	tmp := t.TempDir()
	t.Setenv(envConfigDir, tmp)

	var cfg struct{}
	err := LoadYAML(&cfg)
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestLoadSecret_ErrorWhenFileMissing(t *testing.T) {
	resetCache()
	defer resetCache()
	tmp := t.TempDir()
	t.Setenv(envConfigDir, tmp)

	var secret struct{}
	err := LoadSecret(&secret)
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}
