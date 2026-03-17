package main

import (
	"fmt"
	"strings"

	"github.com/zon/chat/core/auth"
	"github.com/zon/chat/core/config"
	"github.com/zon/chat/core/k8s"
)

// SetAdminCmd implements `wurbctl set admin`.
type SetAdminCmd struct {
	Email string `arg:"" name:"email" help:"Email address of the admin user to create or update."`
}

// Run executes the set admin command.
func (c *SetAdminCmd) Run(ctx *Context) error {
	db, err := ctx.DB()
	if err != nil {
		return err
	}

	user, err := auth.EnsureAdminUser(db, c.Email)
	if err != nil {
		return fmt.Errorf("failed to create or update admin user: %w", err)
	}

	privateKey, publicKey, err := auth.GenerateRSAKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate admin client credential keys: %w", err)
	}

	admin := &auth.TestAdmin{
		Email:      user.Email,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}

	tree, err := config.RepoDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	if err := admin.Write(tree.TestAdmin); err != nil {
		return fmt.Errorf("failed to write admin config: %w", err)
	}
	fmt.Printf("wrote %s\n", tree.TestAdmin)

	secretName := adminSecretName(c.Email)
	filename := secretName + ".yaml"

	if err := k8s.WriteSecretFile(filename, secretName, "ralph", map[string]string{
		"email":      admin.Email,
		"publicKey":  admin.PublicKey,
		"privateKey": admin.PrivateKey,
	}); err != nil {
		return fmt.Errorf("failed to write admin secret file: %w", err)
	}
	fmt.Printf("wrote %s\n", filename)

	return nil
}

// adminSecretName produces a valid Kubernetes resource name from an email address.
func adminSecretName(email string) string {
	sanitized := strings.ToLower(email)
	sanitized = strings.ReplaceAll(sanitized, "@", "-")
	sanitized = strings.ReplaceAll(sanitized, ".", "-")
	var sb strings.Builder
	for _, ch := range sanitized {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			sb.WriteRune(ch)
		}
	}
	return "wurbs-admin-" + sb.String()
}
