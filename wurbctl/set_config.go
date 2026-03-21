package main

import (
	"fmt"
	"os"

	"github.com/zon/chat/core/auth"
	"github.com/zon/chat/core/config"
	"github.com/zon/chat/core/k8s"
	"github.com/zon/chat/core/pg"
	"gorm.io/gorm"
)

const (
	ralphWorkflowNamespace = "ralph-wurbs"
	wurbsNamespace         = "wurbs"
	natsNamespace          = "nats"
	postgresSecret         = "wurbs-postgres-app"
	natsReadSecret         = "nats-secrets"
	natsReadTokenKey       = "dev-token"
	natsWriteSecret        = "nats-dev-token"
	localPostgresPort      = "32432"
	localNATSPort          = "32422"
	natsInternalURL        = "nats://nats.nats.svc.cluster.local:4222"
	testAdminEmail         = "test-admin@example.com"
	testAdminSecretName    = "test-admin"
	configMapName          = "wurbs"
)

// SetConfigCmd implements `wurbctl set config`.
type SetConfigCmd struct {
	Context          string `help:"Kubernetes context to use." name:"context"`
	OIDCIssuer       string `help:"OIDC issuer URL." name:"oidc-issuer"`
	OIDCClientID     string `help:"OIDC client ID." name:"oidc-client-id"`
	OIDCClientSecret string `help:"OIDC client secret." name:"oidc-client-secret"`
}

func (c *SetConfigCmd) Run(ctx *Context) error {
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

	db, err := ctx.DB()
	if err != nil {
		return err
	}

	if err := c.runMigrations(db); err != nil {
		return err
	}

	if err := c.ensureTestAdmin(db, tree); err != nil {
		return err
	}

	return nil
}

func (c *SetConfigCmd) writeConfig(tree *config.ConfigTree) error {
	if err := os.MkdirAll(tree.Parent, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	var cm config.ConfigMap
	config.ReadAt(tree.Config, &cm) // ignore error — file may not exist yet
	if c.OIDCIssuer != "" {
		cm.OIDCIssuer = c.OIDCIssuer
	}
	if cm.OIDCIssuer == "" {
		return fmt.Errorf("--oidc-issuer is required when not already set in config")
	}
	if c.OIDCClientID != "" {
		cm.OIDCClientID = c.OIDCClientID
	}
	if cm.OIDCClientID == "" {
		return fmt.Errorf("--oidc-client-id is required when not already set in config")
	}
	if c.OIDCClientSecret != "" {
		cm.OIDCClientSecret = c.OIDCClientSecret
	}
	if cm.OIDCClientSecret == "" {
		return fmt.Errorf("--oidc-client-secret is required when not already set in config")
	}
	if cm.RESTPort == 0 {
		cm.RESTPort = 8080
	}
	if cm.SocketPort == 0 {
		cm.SocketPort = 8081
	}

	nodeIP, err := k8s.GetNodeIP(c.Context)
	if err != nil {
		return fmt.Errorf("failed to get node IP: %w", err)
	}
	cm.NATSURL = "nats://" + nodeIP + ":" + localNATSPort
	if err := config.WriteAt(tree.Config, &cm); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	fmt.Printf("wrote %s\n", tree.Config)

	remoteCM := cm
	remoteCM.NATSURL = natsInternalURL
	configmapData, err := remoteCM.MarshalConfigMap()
	if err != nil {
		return fmt.Errorf("failed to marshal config map: %w", err)
	}

	if err := k8s.ApplyConfigmap(configMapName, ralphWorkflowNamespace, c.Context, configmapData); err != nil {
		return fmt.Errorf("failed to apply configmap to %s: %w", ralphWorkflowNamespace, err)
	}
	fmt.Printf("applied configmap %s to %s namespace\n", configMapName, ralphWorkflowNamespace)

	return nil
}

func (c *SetConfigCmd) writeNATSDevToken(tree *config.ConfigTree) error {
	data, err := k8s.GetSecret(natsReadSecret, natsNamespace, c.Context)
	if err != nil {
		return fmt.Errorf("failed to load NATS secret: %w", err)
	}
	token := data[natsReadTokenKey]

	if err := config.WriteNATSToken(tree.NATSDevToken, token); err != nil {
		return fmt.Errorf("failed to write NATS token: %w", err)
	}
	fmt.Printf("wrote %s\n", tree.NATSDevToken)

	if err := k8s.ApplySecret(natsWriteSecret, ralphWorkflowNamespace, c.Context, map[string]string{natsReadTokenKey: token}); err != nil {
		return fmt.Errorf("failed to apply NATS secret to %s: %w", ralphWorkflowNamespace, err)
	}
	fmt.Printf("applied secret %s to %s namespace\n", natsWriteSecret, ralphWorkflowNamespace)
	return nil
}

func (c *SetConfigCmd) writePostgresSecret(tree *config.ConfigTree) error {
	clusterIP, err := k8s.GetClusterIP(c.Context)
	if err != nil {
		return fmt.Errorf("failed to get cluster IP: %w", err)
	}

	var secret pg.Secret
	if err := secret.ReadK8s(postgresSecret, wurbsNamespace, c.Context); err != nil {
		return fmt.Errorf("failed to load postgres secret: %w", err)
	}
	secret.Patch(clusterIP, localPostgresPort)

	if err := secret.Write(tree.Postgres); err != nil {
		return fmt.Errorf("failed to write postgres config: %w", err)
	}
	fmt.Printf("wrote %s\n", tree.Postgres)

	if err := secret.WriteK8s(postgresSecret, ralphWorkflowNamespace, c.Context); err != nil {
		return fmt.Errorf("failed to apply postgres secret to %s: %w", ralphWorkflowNamespace, err)
	}
	fmt.Printf("applied secret %s to %s namespace\n", postgresSecret, ralphWorkflowNamespace)
	return nil
}

func (c *SetConfigCmd) runMigrations(db *gorm.DB) error {
	if err := RunMigrations(db); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	fmt.Println("database migrations complete")
	return nil
}

func (c *SetConfigCmd) ensureTestAdmin(db *gorm.DB, tree *config.ConfigTree) error {
	user, err := auth.EnsureTestAdminUser(db, testAdminEmail)
	if err != nil {
		return fmt.Errorf("failed to ensure test admin user: %w", err)
	}

	privateKey, publicKey, err := auth.GenerateRSAKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate test admin client credential keys: %w", err)
	}

	if err := c.writeTestAdmin(tree, user.Email, privateKey, publicKey); err != nil {
		return err
	}

	return nil
}

func (c *SetConfigCmd) writeTestAdmin(tree *config.ConfigTree, email, privateKey, publicKey string) error {
	ta := auth.TestAdmin{
		Email:      email,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}

	if err := ta.WriteK8s(testAdminSecretName, ralphWorkflowNamespace, c.Context); err != nil {
		return fmt.Errorf("failed to apply test admin secret to %s: %w", ralphWorkflowNamespace, err)
	}
	fmt.Printf("applied secret %s to %s namespace\n", testAdminSecretName, ralphWorkflowNamespace)

	if err := ta.Write(tree.TestAdmin); err != nil {
		return fmt.Errorf("failed to write test admin credentials: %w", err)
	}
	fmt.Printf("wrote %s\n", tree.TestAdmin)

	return nil
}
