package set

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/zon/chat/core/pg"
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

// GetSecret retrieves a Kubernetes Secret and returns its data as a map.
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
		return nil, fmt.Errorf("failed to get secret %s: %w", name, err)
	}

	data, err := parseJSONSecret(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse secret: %w", err)
	}
	return data, nil
}

func parseJSONSecret(output []byte) (map[string]string, error) {
	var secret struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(output, &secret); err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for k, v := range secret.Data {
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			result[k] = v
		} else {
			result[k] = string(decoded)
		}
	}
	return result, nil
}

type PostgresConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

func WritePostgresJSON(filename string, host string, port int, secretData map[string]string) error {
	cfg := PostgresConfig{
		Host:     host,
		Port:     port,
		User:     secretData["username"],
		Password: secretData["password"],
		Database: secretData["dbname"],
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal postgres config: %w", err)
	}
	return os.WriteFile(filename, data, 0600)
}

func GetServiceIP(name, namespace, context string) (string, error) {
	args := []string{"get", "svc", name, "-o", "json"}
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}
	if context != "" {
		args = append(args, "--context", context)
	}

	cmd := exec.Command("kubectl", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get service %s: %w", name, err)
	}

	var svc struct {
		ClusterIP string `json:"clusterIP"`
	}
	if err := json.Unmarshal(output, &svc); err != nil {
		return "", fmt.Errorf("failed to parse service: %w", err)
	}
	return svc.ClusterIP, nil
}

func PatchSecret(secretData map[string]string, newHost string, newPort int) *pg.Secret {
	secret := &pg.Secret{
		Username: secretData["username"],
		Password: secretData["password"],
		DBName:   secretData["dbname"],
		Host:     newHost,
		Port:     newPort,
		URI:      secretData["uri"],
		PGPass:   secretData["pgpass"],
		JDBCURI:  secretData["jdbc-uri"],
		FQDNURI:  secretData["fqdn-uri"],
		FQDNJDBC: secretData["fqdn-jdbc"],
	}

	oldHost := secretData["host"]
	oldPort := 5432
	if p, err := strconv.Atoi(secretData["port"]); err == nil {
		oldPort = p
	}

	oldHostPort := fmt.Sprintf("%s:%d", oldHost, oldPort)
	newHostPort := fmt.Sprintf("%s:%d", newHost, newPort)

	secret.URI = strings.ReplaceAll(secret.URI, oldHostPort, newHostPort)
	secret.PGPass = strings.ReplaceAll(secret.PGPass, oldHostPort, newHostPort)
	secret.JDBCURI = strings.ReplaceAll(secret.JDBCURI, oldHostPort, newHostPort)

	secret.FQDNURI = strings.ReplaceAll(secret.FQDNURI, oldHost, newHost)
	secret.FQDNURI = strings.ReplaceAll(secret.FQDNURI, fmt.Sprintf(":%d", oldPort), fmt.Sprintf(":%d", newPort))
	secret.FQDNJDBC = strings.ReplaceAll(secret.FQDNJDBC, oldHost, newHost)
	secret.FQDNJDBC = strings.ReplaceAll(secret.FQDNJDBC, fmt.Sprintf(":%d", oldPort), fmt.Sprintf(":%d", newPort))

	return secret
}
