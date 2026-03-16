package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/zon/chat/core/auth"
	"github.com/zon/chat/core/k8s"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// AdminUser is a minimal user model used by wurbctl set admin.
// It mirrors the application user table without depending on the gonf package.
type AdminUser struct {
	gorm.Model
	Email   string `gorm:"uniqueIndex"`
	IsAdmin bool
}

// AdminEnsurer is a function that creates or updates an admin user in the database.
type AdminEnsurer func(db *gorm.DB, email string) (*AdminUser, error)

// DBOpener is a function that opens a GORM database connection.
type DBOpener func() (*gorm.DB, error)

// SetAdminCmd implements `wurbctl set admin`.
type SetAdminCmd struct {
	Email string `arg:"" name:"email" help:"Email address of the admin user to create or update."`

	Context   string `help:"Kubernetes context to use." name:"context"`
	Namespace string `help:"Kubernetes namespace to use." name:"namespace" default:"default"`
	Local     bool   `help:"Write secret file locally instead of applying to k8s."`

	ensureAdmin AdminEnsurer
	openDB      DBOpener
}

// Run executes the set admin command.
func (c *SetAdminCmd) Run() error {
	if c.Email == "" {
		return fmt.Errorf("email argument is required")
	}

	openDB := c.openDB
	if openDB == nil {
		openDB = OpenAdminDB
	}

	db, err := openDB()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.AutoMigrate(&AdminUser{}); err != nil {
		return fmt.Errorf("failed to migrate admin user table: %w", err)
	}

	ensureAdmin := c.ensureAdmin
	if ensureAdmin == nil {
		ensureAdmin = EnsureAdminUser
	}

	user, err := ensureAdmin(db, c.Email)
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

	if c.Local {
		filename := secretName + ".yaml"
		if err := k8s.WriteSecretFile(filename, secretName, c.Namespace, secretData); err != nil {
			return fmt.Errorf("failed to write admin secret file: %w", err)
		}
		fmt.Printf("wrote %s\n", filename)
	} else {
		if err := k8s.ApplySecret(secretName, c.Namespace, c.Context, secretData); err != nil {
			return fmt.Errorf("failed to apply admin secret: %w", err)
		}
		fmt.Printf("applied %s secret to kubernetes\n", secretName)
	}

	return nil
}

// EnsureAdminUser creates or updates an admin user with the given email.
func EnsureAdminUser(db *gorm.DB, email string) (*AdminUser, error) {
	user := &AdminUser{}

	result := db.Where("email = ?", email).First(user)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to find admin user: %w", result.Error)
	}

	if result.Error == gorm.ErrRecordNotFound {
		user = &AdminUser{Email: email, IsAdmin: true}
		if err := db.Create(user).Error; err != nil {
			return nil, fmt.Errorf("failed to create admin user: %w", err)
		}
	} else if !user.IsAdmin {
		if err := db.Model(user).Update("is_admin", true).Error; err != nil {
			return nil, fmt.Errorf("failed to update admin flag: %w", err)
		}
		user.IsAdmin = true
	}

	return user, nil
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

// OpenAdminDB opens a GORM database connection using standard PG environment variables.
func OpenAdminDB() (*gorm.DB, error) {
	dsn, err := adminDSN()
	if err != nil {
		return nil, err
	}
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

// adminDSN builds a PostgreSQL DSN from standard PG environment variables.
func adminDSN() (string, error) {
	host := os.Getenv("PGHOST")
	port := os.Getenv("PGPORT")
	user := os.Getenv("PGUSER")
	password := os.Getenv("PGPASSWORD")
	database := os.Getenv("PGDATABASE")

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
		return "", fmt.Errorf("missing required environment variables: %v", missing)
	}

	if port == "" {
		port = "5432"
	}

	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, database,
	), nil
}
