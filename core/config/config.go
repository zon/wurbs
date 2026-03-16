package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	envConfigDir     = "WURB_CONFIG"
	defaultConfigDir = "/etc/wurbs"
)

var testMode bool
var cachedTree *ConfigTree

// SetTestMode enables or disables test mode.
// In test mode, if WURB_CONFIG is not set, the module walks up from the
// working directory to find the git repo root and uses ./config there.
func SetTestMode(enabled bool) {
	testMode = enabled
}

func resetCache() {
	cachedTree = nil
}

// ResetCache clears the cached configuration directory.
// This is useful for testing when the config directory needs to be re-resolved.
func ResetCache() {
	resetCache()
}

// ConfigTree holds absolute paths to the config directory and each config file within it.
type ConfigTree struct {
	Parent         string
	Config         string
	Postgres  string
	NATSDevToken string
}

func newConfigTree(parent string) *ConfigTree {
	return &ConfigTree{
		Parent:         parent,
		Config:         filepath.Join(parent, "config.yaml"),
		Postgres:  filepath.Join(parent, "postgres.json"),
		NATSDevToken: filepath.Join(parent, "nats-token"),
	}
}

// Dir returns a ConfigTree rooted at the configuration directory.
// Resolution order:
//  1. WURB_CONFIG environment variable
//  2. In test mode: <git-repo-root>/config (if it exists)
//  3. /etc/wurbs
//
// The result is cached after the first call.
func Dir() (*ConfigTree, error) {
	if cachedTree != nil {
		return cachedTree, nil
	}

	if dir := os.Getenv(envConfigDir); dir != "" {
		cachedTree = newConfigTree(dir)
		return cachedTree, nil
	}

	if testMode {
		dir, err := findRepoConfigDir()
		if err == nil {
			cachedTree = newConfigTree(dir)
			return cachedTree, nil
		}
	}

	cachedTree = newConfigTree(defaultConfigDir)
	return cachedTree, nil
}

// Load reads config.yaml from the config directory and unmarshals it into v.
func Load(v any) error {
	tree, err := Dir()
	if err != nil {
		return err
	}
	return loadYAML(tree.Config, v)
}

// LoadSecret reads secret.yaml from the config directory and unmarshals it into v.
func LoadSecret(v any) error {
	tree, err := Dir()
	if err != nil {
		return err
	}
	return loadYAML(filepath.Join(tree.Parent, "secret.yaml"), v)
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
	RESTPort   int    `yaml:"rest_port"`
	SocketPort int    `yaml:"socket_port"`
	OIDCIssuer string `yaml:"oidc_issuer"`
	NATSURL    string `yaml:"nats_url"`
}

func Write(cfg *Config) error {
	tree, err := Dir()
	if err != nil {
		return err
	}
	return saveYAML(tree.Config, cfg)
}

func saveYAML(path string, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
