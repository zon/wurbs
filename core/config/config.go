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
	Parent       string
	Config       string
	Postgres     string
	NATSDevToken string
	TestAdmin    string
}

func newConfigTree(parent string) *ConfigTree {
	return &ConfigTree{
		Parent:       parent,
		Config:       filepath.Join(parent, "config.yaml"),
		Postgres:     filepath.Join(parent, "postgres.json"),
		NATSDevToken: filepath.Join(parent, "nats-token"),
		TestAdmin:    filepath.Join(parent, "admin.yaml"),
	}
}

// findRepoRoot walks up from the working directory to find the git repo root.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if isDir(filepath.Join(dir, ".git")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// RepoDir returns a ConfigTree rooted at <git-repo-root>/config.
// The config directory does not need to exist yet.
func RepoDir() (*ConfigTree, error) {
	root, err := findRepoRoot()
	if err != nil {
		return nil, err
	}
	return newConfigTree(filepath.Join(root, "config")), nil
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

// findRepoConfigDir returns <git-repo-root>/config if it exists.
func findRepoConfigDir() (string, error) {
	root, err := findRepoRoot()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(root, "config")
	if !isDir(configDir) {
		return "", os.ErrNotExist
	}
	return configDir, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

type Config struct {
	RESTPort   int    `yaml:"restPort"`
	SocketPort int    `yaml:"socketPort"`
	OIDCIssuer string `yaml:"oidcIssuer"`
	NATSURL    string `yaml:"natsURL"`
}

func Write(cfg *Config) error {
	tree, err := Dir()
	if err != nil {
		return err
	}
	return saveYAML(tree.Config, cfg)
}

func ReadAt(path string, cfg *Config) error {
	return loadYAML(path, cfg)
}

func WriteAt(path string, cfg *Config) error {
	return saveYAML(path, cfg)
}

func saveYAML(path string, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func WriteNATSToken(path string, token string) error {
	return os.WriteFile(path, []byte(token), 0600)
}
