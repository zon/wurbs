package core

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const defaultConfigDir = "/etc/wurbs"

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	NATS     NATSConfig     `yaml:"nats"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type DatabaseConfig struct {
	Type string `yaml:"type"`
	Port int    `yaml:"port"`
	Name string `yaml:"name"`
	Host string `yaml:"host"`
}

type NATSConfig struct {
	URL string `yaml:"url"`
}

type Secrets struct {
	Database DatabaseSecrets `yaml:"database"`
	NATS     NATSSecrets     `yaml:"nats"`
	OIDC     OIDCSecrets     `yaml:"oidc"`
}

type DatabaseSecrets struct {
	Password string `yaml:"password"`
	User     string `yaml:"user"`
}

type NATSSecrets struct {
	Token string `yaml:"token"`
}

type OIDCSecrets struct {
	ClientSecret string `yaml:"client_secret"`
}

func GetConfigDir(testMode bool, workingDir string) (string, error) {
	if wurbConfig := os.Getenv("WURB_CONFIG"); wurbConfig != "" {
		return wurbConfig, nil
	}

	if testMode {
		gitRoot, err := FindGitRoot(workingDir)
		if err == nil {
			configPath := filepath.Join(gitRoot, "config")
			if _, err := os.Stat(configPath); err == nil {
				return configPath, nil
			}
		}
	}

	return defaultConfigDir, nil
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

func LoadConfig(configDir string) (*Config, *Secrets, error) {
	configPath := filepath.Join(configDir, "config.yaml")
	secretsPath := filepath.Join(configDir, "secret.yaml")

	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read config.yaml: %w", err)
	}

	secretsData, err := os.ReadFile(secretsPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read secret.yaml: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(configData, &cfg); err != nil {
		return nil, nil, fmt.Errorf("failed to parse config.yaml: %w", err)
	}

	var secrets Secrets
	if err := yaml.Unmarshal(secretsData, &secrets); err != nil {
		return nil, nil, fmt.Errorf("failed to parse secret.yaml: %w", err)
	}

	return &cfg, &secrets, nil
}
