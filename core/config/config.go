package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	RESTPort      int    `yaml:"rest_port"`
	SocketPort    int    `yaml:"socket_port"`
	OIDCIssuer    string `yaml:"oidc_issuer"`
	OIDCClientID  string `yaml:"oidc_client_id"`
	OIDCClientSec string `yaml:"oidc_client_secret"`
	NATSURL       string `yaml:"nats_url"`
	NATSDevToken  string `yaml:"nats_dev_token"`
	TestAdmin     string `yaml:"test_admin"`
	Postgres      string `yaml:"postgres"`
}

func Load() (*Config, error) {
	tree, err := Dir()
	if err != nil {
		return nil, err
	}

	var cm ConfigMap
	if err := loadYAML(tree.Config, &cm); err != nil {
		return nil, err
	}

	cfg := &Config{
		RESTPort:      cm.RESTPort,
		SocketPort:    cm.SocketPort,
		OIDCIssuer:    cm.OIDCIssuer,
		OIDCClientID:  cm.OIDCClientID,
		OIDCClientSec: cm.OIDCClientSecret,
		NATSURL:       cm.NATSURL,
	}

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

func ReadAt(path string, v any) error {
	return loadYAML(path, v)
}

func WriteAt(path string, v any) error {
	return saveYAML(path, v)
}
