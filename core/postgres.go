package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type PostgresConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

func LoadPostgresConfig(configDir string) (*PostgresConfig, error) {
	if configDir == "" {
		configDir = "config"
	}

	configPath := filepath.Join(configDir, "postgres.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read postgres.json: %w", err)
	}

	var cfg PostgresConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse postgres.json: %w", err)
	}

	if cfg.Host == "" {
		return nil, fmt.Errorf("postgres.json: host is required")
	}
	if cfg.Port == 0 {
		return nil, fmt.Errorf("postgres.json: port is required")
	}
	if cfg.User == "" {
		return nil, fmt.Errorf("postgres.json: user is required")
	}
	if cfg.Password == "" {
		return nil, fmt.Errorf("postgres.json: password is required")
	}
	if cfg.Database == "" {
		return nil, fmt.Errorf("postgres.json: database is required")
	}

	return &cfg, nil
}
