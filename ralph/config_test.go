package ralph_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// ralphConfig mirrors the structure of .ralph/config.yaml.
type ralphConfig struct {
	Model  string `yaml:"model"`
	Before []struct {
		Name    string   `yaml:"name"`
		Command string   `yaml:"command"`
		Args    []string `yaml:"args"`
	} `yaml:"before"`
	Services []struct {
		Name    string   `yaml:"name"`
		Command string   `yaml:"command"`
		Args    []string `yaml:"args"`
	} `yaml:"services"`
	Workflow struct {
		Namespace  string `yaml:"namespace"`
		ConfigMaps []struct {
			Name     string `yaml:"name"`
			DestFile string `yaml:"destFile"`
		} `yaml:"configMaps"`
		Secrets []struct {
			Name     string `yaml:"name"`
			DestFile string `yaml:"destFile"`
		} `yaml:"secrets"`
		Env map[string]string `yaml:"env"`
	} `yaml:"workflow"`
}

// loadRalphConfig reads and parses .ralph/config.yaml from the repo root.
func loadRalphConfig(t *testing.T) ralphConfig {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "could not determine test file path")
	repoRoot := filepath.Join(filepath.Dir(file), "..")
	configPath := filepath.Join(repoRoot, ".ralph", "config.yaml")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err, "failed to read .ralph/config.yaml")
	var cfg ralphConfig
	require.NoError(t, yaml.Unmarshal(data, &cfg), "failed to parse .ralph/config.yaml")
	return cfg
}

func TestRalphConfig_Namespace(t *testing.T) {
	cfg := loadRalphConfig(t)
	assert.Equal(t, "ralph-wurbs", cfg.Workflow.Namespace, "workflow namespace should be ralph-wurbs")
}

func TestRalphConfig_WorkflowNamespace(t *testing.T) {
	cfg := loadRalphConfig(t)
	assert.Equal(t, "ralph-wurbs", cfg.Workflow.Namespace, "workflow namespace should be ralph-wurbs")
}

func TestRalphConfig_BeforeBuildsRest(t *testing.T) {
	cfg := loadRalphConfig(t)
	require.NotEmpty(t, cfg.Before, "before scripts should be defined")
	var found bool
	for _, b := range cfg.Before {
		if b.Name == "build rest" {
			found = true
			assert.Equal(t, "go", b.Command)
			assert.Contains(t, b.Args, "build")
			assert.Contains(t, b.Args, "./server/rest")
		}
	}
	assert.True(t, found, "before scripts should include building the rest binary")
}

func TestRalphConfig_BeforeBuildsSocket(t *testing.T) {
	cfg := loadRalphConfig(t)
	require.NotEmpty(t, cfg.Before, "before scripts should be defined")
	var found bool
	for _, b := range cfg.Before {
		if b.Name == "build socket" {
			found = true
			assert.Equal(t, "go", b.Command)
			assert.Contains(t, b.Args, "build")
			assert.Contains(t, b.Args, "./server/socket")
		}
	}
	assert.True(t, found, "before scripts should include building the socket binary")
}

func TestRalphConfig_RestService(t *testing.T) {
	cfg := loadRalphConfig(t)
	require.NotEmpty(t, cfg.Services, "services should be defined")
	var found bool
	for _, s := range cfg.Services {
		if s.Name == "rest" {
			found = true
			assert.Equal(t, "make", s.Command)
			assert.Contains(t, s.Args, "rest")
		}
	}
	assert.True(t, found, "services should include a rest service started via make rest")
}

func TestRalphConfig_SocketService(t *testing.T) {
	cfg := loadRalphConfig(t)
	require.NotEmpty(t, cfg.Services, "services should be defined")
	var found bool
	for _, s := range cfg.Services {
		if s.Name == "socket" {
			found = true
			assert.Equal(t, "make", s.Command)
			assert.Contains(t, s.Args, "socket")
		}
	}
	assert.True(t, found, "services should include a socket service started via make socket")
}

func TestRalphConfig_ConfigMapMounted(t *testing.T) {
	cfg := loadRalphConfig(t)
	require.NotEmpty(t, cfg.Workflow.ConfigMaps, "configMaps should be defined")
	var found bool
	for _, cm := range cfg.Workflow.ConfigMaps {
		if cm.Name == "wurbs-config" {
			found = true
			assert.Equal(t, "./config/main.yaml", cm.DestFile)
		}
	}
	assert.True(t, found, "wurbs-config configmap should be mounted to ./config/main.yaml")
}

func TestRalphConfig_SecretMounted(t *testing.T) {
	cfg := loadRalphConfig(t)
	require.NotEmpty(t, cfg.Workflow.Secrets, "secrets should be defined")
	var found bool
	for _, s := range cfg.Workflow.Secrets {
		if s.Name == "wurbs-secret" {
			found = true
			assert.Equal(t, "./config/secrets.yaml", s.DestFile)
		}
	}
	assert.True(t, found, "wurbs-secret secret should be mounted to ./config/secrets.yaml")
}

func TestRalphConfig_WurbConfigEnv(t *testing.T) {
	cfg := loadRalphConfig(t)
	val, ok := cfg.Workflow.Env["WURB_CONFIG"]
	assert.True(t, ok, "WURB_CONFIG env var should be set")
	assert.Equal(t, "/workspace/config", val, "WURB_CONFIG should be /workspace/config")
}
