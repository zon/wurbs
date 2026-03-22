package config

import (
	"os"
	"path/filepath"
	"sync"
)

const (
	envConfigDir     = "WURB_CONFIG"
	defaultConfigDir = "/etc/wurbs"
)

var (
	testMode   bool
	cachedTree *ConfigTree
	dirMu      sync.RWMutex
)

// SetTestMode enables or disables test mode.
// In test mode, if WURB_CONFIG is not set, the module walks up from the
// working directory to find the git repo root and uses ./config there.
func SetTestMode(enabled bool) {
	dirMu.Lock()
	defer dirMu.Unlock()
	testMode = enabled
}

func resetCache() {
	dirMu.Lock()
	defer dirMu.Unlock()
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
	dirMu.RLock()
	tree := cachedTree
	dirMu.RUnlock()
	if tree != nil {
		return tree, nil
	}

	dirMu.Lock()
	defer dirMu.Unlock()
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
