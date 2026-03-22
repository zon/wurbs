package auth

import (
	"os"

	"github.com/zon/chat/core/k8s"
	"gopkg.in/yaml.v3"
)

// TestAdmin holds the credentials for a test administrator.
type TestAdmin struct {
	Email      string `yaml:"email"`
	PublicKey  string `yaml:"publicKey"`
	PrivateKey string `yaml:"privateKey"`
}

// ReadK8s populates the TestAdmin from a Kubernetes secret.
func (t *TestAdmin) ReadK8s(name, namespace, context string) error {
	data, err := k8s.GetSecret(name, namespace, context)
	if err != nil {
		return err
	}
	return yaml.Unmarshal([]byte(data["test-admin.yaml"]), t)
}

// WriteK8s applies the TestAdmin as a Kubernetes secret.
func (t *TestAdmin) WriteK8s(name, namespace, context string) error {
	data, err := yaml.Marshal(t)
	if err != nil {
		return err
	}
	return k8s.ApplySecret(name, namespace, context, map[string]string{
		"test-admin.yaml": string(data),
	})
}

// Write serializes the TestAdmin to a YAML file at path.
func (t *TestAdmin) Write(path string) error {
	data, err := yaml.Marshal(t)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// Read deserializes the TestAdmin from a YAML file at path.
func (t *TestAdmin) Read(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, t)
}
