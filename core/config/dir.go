package config

import (
	"fmt"
	"os"
	"path/filepath"
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
		NATSDevToken: filepath.Join(parent, "nats-dev-token"),
		TestAdmin:    filepath.Join(parent, "test-admin.yaml"),
	}
}

// findRepoRoot walks up from the working directory to find the git repo root.
func findRepoRoot() (dir string, err error) {
	dir, err = os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
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

// findRepoConfigDir returns <git-repo-root>/config if it exists.
func findRepoConfigDir() (dir string, err error) {
	root, err := findRepoRoot()
	if err != nil {
		return "", fmt.Errorf("failed to find repo config dir: %w", err)
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
