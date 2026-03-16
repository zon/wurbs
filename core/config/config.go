package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	envConfigDir     = "WURB_CONFIG"
	defaultConfigDir = "/etc/wurbs"
	configFile       = "config.yaml"
	secretFile       = "secret.yaml"
)

var testMode bool
var cachedDir string
var dirCached bool

// SetTestMode enables or disables test mode.
// In test mode, if WURB_CONFIG is not set, the module walks up from the
// working directory to find the git repo root and uses ./config there.
func SetTestMode(enabled bool) {
	testMode = enabled
}

func resetCache() {
	cachedDir = ""
	dirCached = false
}

// Dir returns the configuration directory path.
// Resolution order:
//  1. WURB_CONFIG environment variable
//  2. In test mode: <git-repo-root>/config (if it exists)
//  3. /etc/wurbs
//
// The result is cached after the first call.
func Dir() (string, error) {
	if dirCached {
		return cachedDir, nil
	}

	if dir := os.Getenv(envConfigDir); dir != "" {
		cachedDir = dir
		dirCached = true
		return dir, nil
	}

	if testMode {
		dir, err := findRepoConfigDir()
		if err == nil {
			cachedDir = dir
			dirCached = true
			return dir, nil
		}
	}

	cachedDir = defaultConfigDir
	dirCached = true
	return defaultConfigDir, nil
}

// Load reads config.yaml from the config directory and unmarshals it into v.
func Load(v any) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	return loadYAML(filepath.Join(dir, configFile), v)
}

// LoadSecret reads secret.yaml from the config directory and unmarshals it into v.
func LoadSecret(v any) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	return loadYAML(filepath.Join(dir, secretFile), v)
}

func loadYAML(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, v)
}

// findRepoConfigDir walks up from the working directory looking for a .git
// directory. If found, it returns <repo-root>/config when that directory exists.
func findRepoConfigDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if isDir(filepath.Join(dir, ".git")) {
			configDir := filepath.Join(dir, "config")
			if isDir(configDir) {
				return configDir, nil
			}
			return "", os.ErrNotExist
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

type Config struct {
	RESTPort    int    `yaml:"rest_port"`
	SocketPort  int    `yaml:"socket_port"`
	DatabaseURI string `yaml:"database_uri"`
	NATSURL     string `yaml:"nats_url"`
}

func Write(cfg *Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	return saveYAML(filepath.Join(dir, configFile), cfg)
}

func saveYAML(path string, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
