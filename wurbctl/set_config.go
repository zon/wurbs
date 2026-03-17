package main

import (
	"fmt"
	"os"

	"github.com/zon/chat/core/auth"
	"github.com/zon/chat/core/config"
	"github.com/zon/chat/core/k8s"
	"github.com/zon/chat/core/pg"
)

const (
	wurbsNamespace    = "ralph-wurbs"
	ralphNamespace    = "ralph"
	postgresNamespace = "wurbs"
	natsNamespace     = "nats"
	postgresSecret    = "wurbs-postgres-app"
	natsSecret        = "nats-secrets"
	natsTokenKey      = "dev-token"
	localPostgresPort = "32432"
	testAdminEmail    = "admin-test@test.com"
	configMapName     = "wurbs"
)

// SetConfigCmd implements `wurbctl set config`.
type SetConfigCmd struct {
	Context    string `help:"Kubernetes context to use." name:"context"`
	OIDCIssuer string `help:"OIDC issuer URL." name:"oidc-issuer"`
}

func (c *SetConfigCmd) Run() error {
	tree, err := config.RepoDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	if err := c.writeConfig(tree); err != nil {
		return err
	}

	if err := c.writeNATSDevToken(tree); err != nil {
		return err
	}

	if err := c.writePostgresSecret(tree); err != nil {
		return err
	}

	if err := c.runMigrations(); err != nil {
		return err
	}

	if err := c.ensureTestAdmin(tree); err != nil {
		return err
	}

	return nil
}

func (c *SetConfigCmd) writeConfig(tree *config.ConfigTree) error {
	if err := os.MkdirAll(tree.Parent, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	var cfg config.Config
	config.ReadAt(tree.Config, &cfg) // ignore error — file may not exist yet
	if c.OIDCIssuer != "" {
		cfg.OIDCIssuer = c.OIDCIssuer
	}
	if cfg.OIDCIssuer == "" {
		return fmt.Errorf("--oidc-issuer is required when not already set in config")
	}
	if err := config.WriteAt(tree.Config, &cfg); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	fmt.Printf("wrote %s\n", tree.Config)

	configmapData, err := cfg.MarshalConfigMap()
	if err != nil {
		return fmt.Errorf("failed to marshal config map: %w", err)
	}

	if err := k8s.ApplyConfigmap(configMapName, wurbsNamespace, c.Context, configmapData); err != nil {
		return fmt.Errorf("failed to apply configmap to %s: %w", wurbsNamespace, err)
	}
	fmt.Printf("applied configmap %s to %s namespace\n", configMapName, wurbsNamespace)

	return nil
}

func (c *SetConfigCmd) writeNATSDevToken(tree *config.ConfigTree) error {
	data, err := k8s.GetSecret(natsSecret, natsNamespace, c.Context)
	if err != nil {
		return fmt.Errorf("failed to load NATS secret: %w", err)
	}
	token := data[natsTokenKey]

	if err := config.WriteNATSToken(tree.NATSDevToken, token); err != nil {
		return fmt.Errorf("failed to write NATS token: %w", err)
	}
	fmt.Printf("wrote %s\n", tree.NATSDevToken)

	if err := k8s.ApplySecret(natsSecret, wurbsNamespace, c.Context, map[string]string{natsTokenKey: token}); err != nil {
		return fmt.Errorf("failed to apply NATS secret to %s: %w", wurbsNamespace, err)
	}
	fmt.Printf("applied secret %s to %s namespace\n", natsSecret, wurbsNamespace)
	return nil
}

func (c *SetConfigCmd) writePostgresSecret(tree *config.ConfigTree) error {
	clusterIP, err := k8s.GetClusterIP(c.Context)
	if err != nil {
		return fmt.Errorf("failed to get cluster IP: %w", err)
	}

	var secret pg.Secret
	if err := secret.ReadK8s(postgresSecret, postgresNamespace, c.Context); err != nil {
		return fmt.Errorf("failed to load postgres secret: %w", err)
	}
	secret.Patch(clusterIP, localPostgresPort)

	if err := secret.Write(tree.Postgres); err != nil {
		return fmt.Errorf("failed to write postgres config: %w", err)
	}
	fmt.Printf("wrote %s\n", tree.Postgres)

	if err := secret.WriteK8s(postgresSecret, wurbsNamespace, c.Context); err != nil {
		return fmt.Errorf("failed to apply postgres secret to %s: %w", wurbsNamespace, err)
	}
	fmt.Printf("applied secret %s to %s namespace\n", postgresSecret, wurbsNamespace)
	return nil
}

func (c *SetConfigCmd) runMigrations() error {
	db, err := pg.Open()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := RunMigrations(db); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	fmt.Println("database migrations complete")
	return nil
}

func (c *SetConfigCmd) ensureTestAdmin(tree *config.ConfigTree) error {
	db, err := pg.Open()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	user, err := auth.EnsureTestAdminUser(db, testAdminEmail)
	if err != nil {
		return fmt.Errorf("failed to ensure test admin user: %w", err)
	}

	privateKey, publicKey, err := auth.GenerateRSAKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate test admin client credential keys: %w", err)
	}

	if err := c.saveTestAdminCredentials(tree, user.Email, privateKey, publicKey); err != nil {
		return err
	}

	return nil
}

func (c *SetConfigCmd) saveTestAdminCredentials(tree *config.ConfigTree, email, privateKey, publicKey string) error {
	secretData := map[string]string{
		"TEST_ADMIN_EMAIL":      email,
		"TEST_ADMIN_CLIENT_KEY": privateKey,
		"TEST_ADMIN_CLIENT_PUB": publicKey,
	}

	if err := k8s.ApplySecret("wurbs-test-admin", ralphNamespace, c.Context, secretData); err != nil {
		return fmt.Errorf("failed to apply test admin secret to %s: %w", ralphNamespace, err)
	}
	fmt.Printf("applied secret wurbs-test-admin to %s namespace\n", ralphNamespace)

	localSecretPath := tree.Parent + "/secret.yaml"
	if err := writeSecretFile(localSecretPath, email, privateKey, publicKey); err != nil {
		return fmt.Errorf("failed to write test admin credentials to local config: %w", err)
	}
	fmt.Printf("wrote %s\n", localSecretPath)

	return nil
}

func writeSecretFile(path, email, privateKey, publicKey string) error {
	secret := map[string]any{
		"auth": map[string]string{
			"client_public_key":  publicKey,
			"client_private_key": privateKey,
			"test_admin_email":   email,
		},
	}

	data, err := config.MarshalSecret(secret)
	if err != nil {
		return fmt.Errorf("failed to marshal secret: %w", err)
	}

	return os.WriteFile(path, data, 0600)
}
