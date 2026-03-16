package set

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/zon/chat/core/config"
	"github.com/zon/chat/core/k8s"
	"github.com/zon/chat/core/pg"
)

const (
	wurbsNamespace    = "ralph-wurbs"
	postgresNamespace = "ralph-wurbs"
	natsNamespace     = "nats"
	postgresSecret    = "wurbs-postgres-app"
	natsSecret        = "nats-secrets"
	natsTokenKey      = "dev-token"
	localPostgresPort = "32432"

	configMapName  = "wurbs-config"
	natsSecretName = "nats-dev-token"
)

type SecretLoader func(name, namespace, context string) (map[string]string, error)
type ClusterIPLoader func(context string) (string, error)

func DefaultLoadSecret(name, namespace, context string) (map[string]string, error) {
	return k8s.GetSecret(name, namespace, context)
}

type ConfigCmd struct {
	ClusterIP  string `help:"Kubernetes cluster IP for local access. Defaults to the cluster IP from the kubectl context." name:"cluster-ip"`
	Context    string `help:"Kubernetes context to use." name:"context"`
	Namespace  string `help:"Kubernetes namespace to use." name:"namespace" default:"wurbs"`
	Local      bool   `help:"Create configmap and secret files for local development instead of applying to k8s."`
	OIDCIssuer string `help:"OIDC issuer URL." name:"oidc-issuer" required:""`

	loadSecret    SecretLoader
	loadClusterIP ClusterIPLoader
}

func (c *ConfigCmd) Run() error {
	clusterIP := c.ClusterIP

	if clusterIP == "" {
		loadClusterIP := c.loadClusterIP
		if loadClusterIP == nil {
			loadClusterIP = k8s.GetClusterIP
		}
		ip, err := loadClusterIP(c.Context)
		if err != nil {
			return fmt.Errorf("failed to get cluster IP from kubectl context: %w", err)
		}
		clusterIP = ip
	}

	if !isValidIP(clusterIP) {
		return fmt.Errorf("invalid cluster IP: %s", clusterIP)
	}

	loadSecret := c.loadSecret
	if loadSecret == nil {
		loadSecret = DefaultLoadSecret
	}

	configDir, err := config.Dir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configYAML := map[string]string{
		"oidc-issuer": c.OIDCIssuer,
	}
	configBytes, err := yaml.Marshal(configYAML)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, configBytes, 0644); err != nil {
		return fmt.Errorf("failed to write config.yaml: %w", err)
	}
	fmt.Printf("wrote %s\n", configPath)

	configMapData := map[string]string{
		"oidc-issuer": c.OIDCIssuer,
	}

	if c.Local {
		configMapPath := filepath.Join(configDir, "configmap.yaml")
		if err := k8s.WriteConfigmapFile(configMapPath, configMapName, wurbsNamespace, configMapData); err != nil {
			return fmt.Errorf("failed to write configmap file: %w", err)
		}
		fmt.Printf("wrote %s\n", configMapPath)
	} else {
		if err := k8s.ApplyConfigmap(configMapName, wurbsNamespace, c.Context, configMapData); err != nil {
			return fmt.Errorf("failed to apply configmap: %w", err)
		}
		fmt.Printf("applied ConfigMap %s to %s namespace\n", configMapName, wurbsNamespace)
	}

	natsTokenData, err := loadSecret(natsSecret, natsNamespace, c.Context)
	if err != nil {
		return fmt.Errorf("failed to load NATS secret: %w", err)
	}
	natsToken := natsTokenData[natsTokenKey]

	natsTokenPath := filepath.Join(configDir, "nats-token")
	if err := os.WriteFile(natsTokenPath, []byte(natsToken), 0600); err != nil {
		return fmt.Errorf("failed to write NATS token: %w", err)
	}
	fmt.Printf("wrote %s\n", natsTokenPath)

	natsSecretData := map[string]string{
		natsTokenKey: natsToken,
	}

	if c.Local {
		natsSecretPath := filepath.Join(configDir, "nats-secret.yaml")
		if err := k8s.WriteSecretFile(natsSecretPath, natsSecretName, wurbsNamespace, natsSecretData); err != nil {
			return fmt.Errorf("failed to write NATS secret file: %w", err)
		}
		fmt.Printf("wrote %s\n", natsSecretPath)
	} else {
		if err := k8s.ApplySecret(natsSecretName, wurbsNamespace, c.Context, natsSecretData); err != nil {
			return fmt.Errorf("failed to apply NATS secret: %w", err)
		}
		fmt.Printf("applied Secret %s to %s namespace\n", natsSecretName, wurbsNamespace)
	}

	secretData, err := loadSecret(postgresSecret, postgresNamespace, c.Context)
	if err != nil {
		return fmt.Errorf("failed to load postgres secret: %w", err)
	}

	internalHost := "postgres." + postgresNamespace + ".svc.cluster.local"
	secret := &pg.Secret{
		Username:    secretData["username"],
		Password:    secretData["password"],
		DBName:      secretData["dbname"],
		Host:        clusterIP,
		Port:        localPostgresPort,
		URI:         patchURI(secretData["uri"], clusterIP, localPostgresPort),
		PGPass:      secretData["pgpass"],
		JDBCURI:     patchURI(secretData["jdbc-uri"], clusterIP, localPostgresPort),
		FQDNURI:     patchURI(secretData["fqdn-uri"], clusterIP, localPostgresPort),
		FQDNJDBCURI: patchURI(secretData["fqdn-jdbc-uri"], clusterIP, localPostgresPort),
	}

	postgresConfigPath := filepath.Join(configDir, "postgres.json")
	if err := secret.Write(postgresConfigPath); err != nil {
		return fmt.Errorf("failed to write postgres config: %w", err)
	}
	fmt.Printf("wrote %s\n", postgresConfigPath)

	postgresJSON, _ := json.Marshal(secret)
	postgresSecretData := map[string]string{
		"username":      secretData["username"],
		"password":      secretData["password"],
		"dbname":        secretData["dbname"],
		"host":          internalHost,
		"port":          "5432",
		"uri":           secretData["uri"],
		"pgpass":        secretData["pgpass"],
		"jdbc-uri":      secretData["jdbc-uri"],
		"fqdn-uri":      secretData["fqdn-uri"],
		"postgres.json": string(postgresJSON),
	}

	if c.Local {
		postgresSecretPath := filepath.Join(configDir, "postgres-secret.yaml")
		if err := k8s.WriteSecretFile(postgresSecretPath, postgresSecret, wurbsNamespace, postgresSecretData); err != nil {
			return fmt.Errorf("failed to write postgres secret file: %w", err)
		}
		fmt.Printf("wrote %s\n", postgresSecretPath)
	} else {
		if err := k8s.ApplySecret(postgresSecret, wurbsNamespace, c.Context, postgresSecretData); err != nil {
			return fmt.Errorf("failed to apply postgres secret: %w", err)
		}
		fmt.Printf("applied Secret %s to %s namespace\n", postgresSecret, wurbsNamespace)
	}

	return nil
}

func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

func patchURI(uri, newHost, newPort string) string {
	if uri == "" {
		return ""
	}

	parts := strings.Split(uri, "://")
	if len(parts) < 2 {
		return uri
	}

	scheme := parts[0]
	rest := parts[1]

	atIdx := strings.Index(rest, "@")
	if atIdx != -1 {
		userinfo := rest[:atIdx]
		hostPath := rest[atIdx+1:]

		slashIdx := strings.Index(hostPath, "/")
		if slashIdx == -1 {
			slashIdx = len(hostPath)
		}
		hostPath = newHost + ":" + newPort + hostPath[slashIdx:]

		return scheme + "://" + userinfo + "@" + hostPath
	}

	slashIdx := strings.Index(rest, "/")
	if slashIdx == -1 {
		slashIdx = len(rest)
	}
	rest = newHost + ":" + newPort + rest[slashIdx:]

	return scheme + "://" + rest
}
