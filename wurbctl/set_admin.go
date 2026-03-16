package main

import (
	"fmt"
	"strings"

	"github.com/zon/chat/core/auth"
	"github.com/zon/chat/core/k8s"
	"github.com/zon/chat/core/pg"
)

// SetAdminCmd implements `wurbctl set admin`.
type SetAdminCmd struct {
	Email string `arg:"" name:"email" help:"Email address of the admin user to create or update."`
}

// Run executes the set admin command.
func (c *SetAdminCmd) Run() error {
	db, err := pg.Open()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	user, err := auth.EnsureAdminUser(db, c.Email)
	if err != nil {
		return fmt.Errorf("failed to create or update admin user: %w", err)
	}

	privateKey, publicKey, err := auth.GenerateRSAKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate admin client credential keys: %w", err)
	}

	secretName := adminSecretName(c.Email)

	secretData := map[string]string{
		"ADMIN_CLIENT_PRIVATE_KEY": privateKey,
		"ADMIN_CLIENT_PUBLIC_KEY":  publicKey,
		"ADMIN_EMAIL":              user.Email,
	}

	filename := secretName + ".yaml"
	if err := k8s.WriteSecretFile(filename, secretName, "ralph", secretData); err != nil {
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

