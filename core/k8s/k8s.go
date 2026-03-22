package k8s

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// GetNodeIP returns the InternalIP of the first ready worker node.
func GetNodeIP(context string) (string, error) {
	args := []string{"get", "nodes", "-o", "jsonpath={.items[0].status.addresses[?(@.type==\"InternalIP\")].address}"}
	if context != "" {
		args = append(args, "--context", context)
	}

	cmd := exec.Command("kubectl", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("kubectl get nodes failed: %w", err)
	}

	ip := strings.TrimSpace(string(output))
	if ip == "" {
		return "", fmt.Errorf("no InternalIP found on cluster nodes")
	}

	return ip, nil
}

func GetClusterIP(context string) (string, error) {
	args := []string{"config", "view", "--minify", "-o", "jsonpath={.clusters[0].cluster.server}"}
	if context != "" {
		args = append(args, "--context", context)
	}

	cmd := exec.Command("kubectl", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("kubectl config view failed: %w", err)
	}

	server := strings.TrimSpace(string(output))
	u, err := url.Parse(server)
	if err != nil {
		return "", fmt.Errorf("failed to parse cluster server URL %q: %w", server, err)
	}

	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("no host found in cluster server URL %q", server)
	}

	if net.ParseIP(host) == nil {
		return "", fmt.Errorf("cluster server host is not a valid IP address: %q", host)
	}

	return host, nil
}

func GetSecret(name, namespace, context string) (map[string]string, error) {
	args := []string{"get", "secret", name, "-o", "json"}
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}
	if context != "" {
		args = append(args, "--context", context)
	}

	cmd := exec.Command("kubectl", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("kubectl get secret failed: %w", err)
	}

	data, err := parseK8sSecretJSON(output)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func parseK8sSecretJSON(data []byte) (map[string]string, error) {
	var secret struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(data, &secret); err != nil {
		return nil, fmt.Errorf("failed to parse secret JSON: %w", err)
	}

	result := make(map[string]string)
	for k, v := range secret.Data {
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			continue
		}
		result[k] = string(decoded)
	}

	return result, nil
}

func ApplyConfigmap(name, namespace, context string, data map[string]string) error {
	yaml := BuildConfigmapYAML(name, namespace, data)
	return kubectlApply(yaml, namespace, context)
}

func ApplySecret(name, namespace, context string, data map[string]string) error {
	yaml := BuildSecretYAML(name, namespace, data)
	return kubectlApply(yaml, namespace, context)
}

func WriteConfigmapFile(filename, name, namespace string, data map[string]string) error {
	yaml := BuildConfigmapYAML(name, namespace, data)
	return os.WriteFile(filename, []byte(yaml), 0600)
}

func WriteSecretFile(filename, name, namespace string, data map[string]string) error {
	yaml := BuildSecretYAML(name, namespace, data)
	return os.WriteFile(filename, []byte(yaml), 0600)
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
