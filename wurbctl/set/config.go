package set

import (
	"fmt"
)

// PostgresEnsurer is a function that creates or updates a postgres user and database.
// It is a field on ConfigCmd so tests can inject a mock.
type PostgresEnsurer func(host string, port int, adminUser, adminPassword, appUser, appPassword, dbName string) error

// ConfigCmd implements `wurbctl set config`.
type ConfigCmd struct {
	// Postgres admin credentials (to create the app user and database)
	DBHost          string `help:"PostgreSQL host." env:"PGHOST" required:""`
	DBPort          int    `help:"PostgreSQL port." env:"PGPORT" default:"5432"`
	DBAdminUser     string `help:"PostgreSQL admin user." name:"db-admin-user" env:"PGADMINUSER" required:""`
	DBAdminPassword string `help:"PostgreSQL admin password." name:"db-admin-password" env:"PGADMINPASSWORD"`

	// App database credentials
	DBUser     string `help:"PostgreSQL app user to create." name:"db-user" env:"PGUSER" required:""`
	DBPassword string `help:"PostgreSQL app user password." name:"db-password" env:"PGPASSWORD" required:""`
	DBName     string `help:"PostgreSQL database name to create." name:"db-name" env:"PGDATABASE" required:""`

	// OIDC settings
	OIDCIssuer   string `help:"OIDC issuer URL." name:"oidc-issuer" required:""`
	OIDCClientID string `help:"OIDC client ID." name:"oidc-client-id" required:""`

	// Optional flags
	Test      bool   `help:"Create or update test user client credential flow keys."`
	Local     bool   `help:"Create configmap and secret files for local development instead of applying to k8s."`
	Context   string `help:"Kubernetes context to use." name:"context"`
	Namespace string `help:"Kubernetes namespace to use." name:"namespace" default:"default"`

	// ensurePostgres is injectable for testing; defaults to EnsurePostgresUserAndDB.
	ensurePostgres PostgresEnsurer
}

// Run executes the set config command.
func (c *ConfigCmd) Run() error {
	// Validate required fields
	var missing []string
	if c.DBHost == "" {
		missing = append(missing, "--db-host / PGHOST")
	}
	if c.DBAdminUser == "" {
		missing = append(missing, "--db-admin-user / PGADMINUSER")
	}
	if c.DBUser == "" {
		missing = append(missing, "--db-user / PGUSER")
	}
	if c.DBPassword == "" {
		missing = append(missing, "--db-password / PGPASSWORD")
	}
	if c.DBName == "" {
		missing = append(missing, "--db-name / PGDATABASE")
	}
	if c.OIDCIssuer == "" {
		missing = append(missing, "--oidc-issuer")
	}
	if c.OIDCClientID == "" {
		missing = append(missing, "--oidc-client-id")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required values: %v", missing)
	}

	// Resolve postgres ensurer
	ensurePostgres := c.ensurePostgres
	if ensurePostgres == nil {
		ensurePostgres = EnsurePostgresUserAndDB
	}

	// Step 1: Create or update postgres user and database
	if err := ensurePostgres(c.DBHost, c.DBPort, c.DBAdminUser, c.DBAdminPassword, c.DBUser, c.DBPassword, c.DBName); err != nil {
		return fmt.Errorf("failed to configure postgres: %w", err)
	}

	// Step 2: Build configmap and secret data
	configData := map[string]string{
		"PGHOST":         c.DBHost,
		"PGPORT":         fmt.Sprintf("%d", c.DBPort),
		"PGDATABASE":     c.DBName,
		"PGUSER":         c.DBUser,
		"OIDC_ISSUER":    c.OIDCIssuer,
		"OIDC_CLIENT_ID": c.OIDCClientID,
	}

	secretData := map[string]string{
		"PGPASSWORD": c.DBPassword,
	}

	// Step 3: Optionally generate test user client credential flow keys
	if c.Test {
		privateKey, publicKey, err := GenerateRSAKeyPair()
		if err != nil {
			return fmt.Errorf("failed to generate test keys: %w", err)
		}
		secretData["TEST_CLIENT_PRIVATE_KEY"] = privateKey
		secretData["TEST_CLIENT_PUBLIC_KEY"] = publicKey
	}

	// Step 4: Apply configmap and secret
	if c.Local {
		if err := WriteConfigmapFile("wurbs-config.yaml", "wurbs-config", c.Namespace, configData); err != nil {
			return fmt.Errorf("failed to write configmap file: %w", err)
		}
		if err := WriteSecretFile("wurbs-secret.yaml", "wurbs-secret", c.Namespace, secretData); err != nil {
			return fmt.Errorf("failed to write secret file: %w", err)
		}
		fmt.Println("wrote wurbs-config.yaml and wurbs-secret.yaml")
	} else {
		if err := ApplyConfigmap("wurbs-config", c.Namespace, c.Context, configData); err != nil {
			return fmt.Errorf("failed to apply configmap: %w", err)
		}
		if err := ApplySecret("wurbs-secret", c.Namespace, c.Context, secretData); err != nil {
			return fmt.Errorf("failed to apply secret: %w", err)
		}
		fmt.Println("applied wurbs-config configmap and wurbs-secret secret to kubernetes")
	}

	return nil
}
