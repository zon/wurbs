package yaml

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// BuildConfigmapYAML returns a Kubernetes ConfigMap YAML string.
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

// BuildSecretYAML returns a Kubernetes Secret YAML string.
func BuildSecretYAML(name, namespace string, data map[string]string) string {
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
