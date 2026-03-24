package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadYAML reads config.yaml from the config directory and unmarshals it into v.
func LoadYAML(v any) error {
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
		return fmt.Errorf("failed to read %s: %w", path, err)
	}
	return yaml.Unmarshal(data, v)
}

func saveYAML(path string, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func Marshal(v any) ([]byte, error) {
	return yaml.Marshal(v)
}

func MarshalSecret(v map[string]any) ([]byte, error) {
	return yaml.Marshal(v)
}

func WriteNATSToken(path string, token string) error {
	return os.WriteFile(path, []byte(token), 0600)
}
