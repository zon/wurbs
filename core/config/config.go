package config

import (
	"os"
	"path/filepath"
)

const envConfigDir = "WURBS_CONFIG"

func ConfigDir() (string, error) {
	if envDir := os.Getenv(envConfigDir); envDir != "" {
		return envDir, nil
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return "", err
	}

	return filepath.Join(repoRoot, "config"), nil
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
