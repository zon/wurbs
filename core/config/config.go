package config

import (
	"fmt"
	"os"
	"path/filepath"
)

func GetConfigDir(workingDir string) (string, error) {
	if wurbsConfig := os.Getenv("WURBS_CONFIG"); wurbsConfig != "" {
		return wurbsConfig, nil
	}

	gitRoot, err := FindGitRoot(workingDir)
	if err != nil {
		return "", err
	}

	return filepath.Join(gitRoot, "config"), nil
}

func FindGitRoot(startDir string) (string, error) {
	dir := startDir
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no git repository found")
		}
		dir = parent
	}
}
