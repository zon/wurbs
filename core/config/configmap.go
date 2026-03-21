package config

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	configMapName          = "wurbs"
	ralphWorkflowNamespace = "ralph-wurbs"
)

type ConfigMap struct {
	RESTPort         int    `yaml:"restPort"`
	SocketPort       int    `yaml:"socketPort"`
	OIDCIssuer       string `yaml:"oidcIssuer"`
	OIDCClientID     string `yaml:"oidcClientID"`
	OIDCClientSecret string `yaml:"oidcClientSecret"`
	NATSURL          string `yaml:"natsURL"`
}

func (c *ConfigMap) Load() error {
	return LoadYAML(c)
}

func (c *ConfigMap) Write() error {
	tree, err := Dir()
	if err != nil {
		return err
	}
	return saveYAML(tree.Config, c)
}

func (c *ConfigMap) WriteToK8s(context string) error {
	data, err := c.MarshalConfigMap()
	if err != nil {
		return err
	}
	return k8sApplyConfigmap(configMapName, ralphWorkflowNamespace, context, data)
}

func (c *ConfigMap) MarshalConfigMap() (map[string]string, error) {
	data, err := yaml.Marshal(c)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"config.yaml": string(data),
	}, nil
}

func k8sApplyConfigmap(name, namespace, context string, data map[string]string) error {
	yaml := BuildConfigmapYAML(name, namespace, data)
	return kubectlApply(yaml, namespace, context)
}

func kubectlApply(yaml, namespace, context string) error {
	args := []string{"apply", "-f", "-"}
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}
	if context != "" {
		args = append(args, "--context", context)
	}

	cmd := exec.Command("kubectl", args...)
	cmd.Stdin = strings.NewReader(yaml)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl apply failed: %w", err)
	}
	return nil
}

func BuildConfigmapYAML(name, namespace string, data map[string]string) string {
	var sb strings.Builder
	sb.WriteString("apiVersion: v1\n")
	sb.WriteString("kind: ConfigMap\n")
	sb.WriteString("metadata:\n")
	sb.WriteString(fmt.Sprintf("  name: %s\n", name))
	if namespace != "" {
		sb.WriteString(fmt.Sprintf("  namespace: %s\n", namespace))
	}
	sb.WriteString("data:\n")
	for k, v := range data {
		sb.WriteString(fmt.Sprintf("  %s: %q\n", k, v))
	}
	return sb.String()
}
