package set

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ApplyConfigmap creates or updates a Kubernetes ConfigMap using kubectl.
func ApplyConfigmap(name, namespace, context string, data map[string]string) error {
	yaml := buildConfigmapYAML(name, namespace, data)
	return kubectlApply(yaml, namespace, context)
}

// ApplySecret creates or updates a Kubernetes Secret using kubectl.
func ApplySecret(name, namespace, context string, data map[string]string) error {
	yaml := buildSecretYAML(name, namespace, data)
	return kubectlApply(yaml, namespace, context)
}

// WriteConfigmapFile writes a Kubernetes ConfigMap YAML to a local file.
func WriteConfigmapFile(filename, name, namespace string, data map[string]string) error {
	yaml := buildConfigmapYAML(name, namespace, data)
	return os.WriteFile(filename, []byte(yaml), 0600)
}

// WriteSecretFile writes a Kubernetes Secret YAML to a local file.
func WriteSecretFile(filename, name, namespace string, data map[string]string) error {
	yaml := buildSecretYAML(name, namespace, data)
	return os.WriteFile(filename, []byte(yaml), 0600)
}

// kubectlApply runs kubectl apply -f - with the given YAML input.
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

// buildConfigmapYAML constructs a Kubernetes ConfigMap YAML string.
func buildConfigmapYAML(name, namespace string, data map[string]string) string {
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
		// Quote values to handle special characters
		sb.WriteString(fmt.Sprintf("  %s: %q\n", k, v))
	}
	return sb.String()
}

// buildSecretYAML constructs a Kubernetes Secret YAML string with base64-encoded values.
func buildSecretYAML(name, namespace string, data map[string]string) string {
	var sb strings.Builder
	sb.WriteString("apiVersion: v1\n")
	sb.WriteString("kind: Secret\n")
	sb.WriteString("metadata:\n")
	sb.WriteString(fmt.Sprintf("  name: %s\n", name))
	if namespace != "" {
		sb.WriteString(fmt.Sprintf("  namespace: %s\n", namespace))
	}
	sb.WriteString("type: Opaque\n")
	sb.WriteString("data:\n")
	for k, v := range data {
		encoded := base64.StdEncoding.EncodeToString([]byte(v))
		sb.WriteString(fmt.Sprintf("  %s: %s\n", k, encoded))
	}
	return sb.String()
}
