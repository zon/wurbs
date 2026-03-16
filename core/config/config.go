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

// SetTestMode enables or disables test mode.
// In test mode, if WURB_CONFIG is not set, the module walks up from the
// working directory to find the git repo root and uses ./config there.
func SetTestMode(enabled bool) {
	testMode = enabled
}

// Dir returns the configuration directory path.
// Resolution order:
//  1. WURB_CONFIG environment variable
//  2. In test mode: <git-repo-root>/config (if it exists)
//  3. /etc/wurbs
func Dir() (string, error) {
	if dir := os.Getenv(envConfigDir); dir != "" {
		return dir, nil
	}

	if testMode {
		dir, err := findRepoConfigDir()
		if err == nil {
			return dir, nil
		}
	}

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
