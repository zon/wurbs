package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	RESTPort     int
	SocketPort   int
	OIDCIssuer   string
	NATSURL      string
	NATSDevToken string
	TestAdmin    string
	Postgres     string
}

func Load() (*Config, error) {
	tree, err := Dir()
	if err != nil {
		return nil, err
	}

	cfg := &Config{}

	var cm ConfigMap
	if err := loadYAML(tree.Config, &cm); err != nil {
		return nil, err
	}
	cfg.RESTPort = cm.RESTPort
	cfg.SocketPort = cm.SocketPort
	cfg.OIDCIssuer = cm.OIDCIssuer
	cfg.NATSURL = cm.NATSURL

	if data, err := os.ReadFile(tree.NATSDevToken); err == nil {
		cfg.NATSDevToken = strings.TrimSpace(string(data))
	}

	if data, err := os.ReadFile(tree.TestAdmin); err == nil {
		cfg.TestAdmin = strings.TrimSpace(string(data))
	}

	if data, err := os.ReadFile(tree.Postgres); err == nil {
		cfg.Postgres = strings.TrimSpace(string(data))
	}

	return cfg, nil
}

func (c *Config) MarshalConfigMap() (map[string]string, error) {
	data, err := yaml.Marshal(c)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"config.yaml": string(data),
	}, nil
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
