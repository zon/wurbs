package migrate

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConfigDir returns the config directory path.
// It uses the WURB_CONFIG env var, falling back to /etc/wurbs.
func ConfigDir() string {
	if dir := os.Getenv("WURB_CONFIG"); dir != "" {
		return dir
	}
	return "/etc/wurbs"
}

// LoadConfig reads the k8s ConfigMap YAML file from the config directory
// and returns the data map.
func LoadConfig(dir string) (map[string]string, error) {
	path := filepath.Join(dir, "main.yaml")
	return parseConfigmapYAML(path)
}

// LoadSecrets reads the k8s Secret YAML file from the config directory
// and returns the decoded data map.
func LoadSecrets(dir string) (map[string]string, error) {
	path := filepath.Join(dir, "secrets.yaml")
	return parseSecretYAML(path)
}

// DSN builds a PostgreSQL DSN from the config and secret files.
// Returns an error if any required value is missing.
func DSN() (string, error) {
	dir := ConfigDir()

	cfg, err := LoadConfig(dir)
	if err != nil {
		return "", fmt.Errorf("failed to load config from %s: %w", dir, err)
	}

	secrets, err := LoadSecrets(dir)
	if err != nil {
		return "", fmt.Errorf("failed to load secrets from %s: %w", dir, err)
	}

	host := cfg["PGHOST"]
	port := cfg["PGPORT"]
	user := cfg["PGUSER"]
	database := cfg["PGDATABASE"]
	password := secrets["PGPASSWORD"]

	var missing []string
	if host == "" {
		missing = append(missing, "PGHOST")
	}
	if user == "" {
		missing = append(missing, "PGUSER")
	}
	if database == "" {
		missing = append(missing, "PGDATABASE")
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("missing required config values: %v", missing)
	}

	if port == "" {
		port = "5432"
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, database,
	)
	return dsn, nil
}

// parseConfigmapYAML parses a k8s ConfigMap YAML file and returns the data map.
// Values in ConfigMap data are plain strings (possibly quoted).
func parseConfigmapYAML(path string) (map[string]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return parseDataSection(string(content), false)
}

// parseSecretYAML parses a k8s Secret YAML file and returns the decoded data map.
// Values in Secret data are base64-encoded.
func parseSecretYAML(path string) (map[string]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return parseDataSection(string(content), true)
}

// parseDataSection parses the `data:` section of a k8s YAML file.
// If base64Decode is true, values are base64-decoded.
func parseDataSection(yaml string, base64Decode bool) (map[string]string, error) {
	result := make(map[string]string)
	lines := strings.Split(yaml, "\n")

	inData := false
	for _, line := range lines {
		// Check for the start of the data section
		if strings.TrimSpace(line) == "data:" {
			inData = true
			continue
		}

		// If we're in the data section, parse key-value pairs
		if inData {
			// A line starting without spaces at the beginning signals a new section
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
				inData = false
				continue
			}

			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}

			// Parse "KEY: value" format
			idx := strings.Index(trimmed, ": ")
			if idx < 0 {
				continue
			}

			key := trimmed[:idx]
			value := trimmed[idx+2:]

			// Strip surrounding quotes if present (ConfigMap values may be quoted)
			if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
				// Unescape Go-style quoted string
				unquoted, err := unquoteString(value)
				if err == nil {
					value = unquoted
				}
			}

			if base64Decode {
				decoded, err := base64.StdEncoding.DecodeString(value)
				if err != nil {
					return nil, fmt.Errorf("failed to base64 decode value for key %s: %w", key, err)
				}
				value = string(decoded)
			}

			result[key] = value
		}
	}

	return result, nil
}

// unquoteString removes surrounding double quotes and handles basic escape sequences.
func unquoteString(s string) (string, error) {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return s, nil
	}
	// Use strconv-style unquoting
	inner := s[1 : len(s)-1]
	var result strings.Builder
	i := 0
	for i < len(inner) {
		if inner[i] == '\\' && i+1 < len(inner) {
			switch inner[i+1] {
			case 'n':
				result.WriteByte('\n')
				i += 2
			case 't':
				result.WriteByte('\t')
				i += 2
			case '"':
				result.WriteByte('"')
				i += 2
			case '\\':
				result.WriteByte('\\')
				i += 2
			default:
				result.WriteByte(inner[i])
				i++
			}
		} else {
			result.WriteByte(inner[i])
			i++
		}
	}
	return result.String(), nil
}
