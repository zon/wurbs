package set

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/zon/chat/core/config"
	"github.com/zon/chat/core/pg"
)

const (
	wurbsNamespace    = "wurbs"
	postgresSecret    = "wurbs-postgres-app"
	localPostgresPort = "32432"
)

type SecretLoader func(name, namespace, context string) (map[string]string, error)
type ClusterIPLoader func(context string) (string, error)

func DefaultLoadSecret(name, namespace, context string) (map[string]string, error) {
	return GetSecret(name, namespace, context)
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
			loadClusterIP = GetClusterIP
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

	secretData, err := loadSecret(postgresSecret, c.Namespace, c.Context)
	if err != nil {
		return fmt.Errorf("failed to load secret: %w", err)
	}

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

	configDir, err := config.Dir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	postgresConfigPath := filepath.Join(configDir, "postgres.json")
	if err := secret.Write(postgresConfigPath); err != nil {
		return fmt.Errorf("failed to write postgres config: %w", err)
	}

	fmt.Printf("wrote %s\n", postgresConfigPath)

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
