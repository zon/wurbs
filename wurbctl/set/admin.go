package set

import (
	"fmt"
	"strings"

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
// It is a field on AdminCmd so tests can inject a mock.
type AdminEnsurer func(db *gorm.DB, email string) (*AdminUser, error)

// DBOpener is a function that opens a GORM database connection.
// It is a field on AdminCmd so tests can inject a mock.
type DBOpener func() (*gorm.DB, error)

// AdminCmd implements `wurbctl set admin`.
type AdminCmd struct {
	Email string `arg:"" name:"email" help:"Email address of the admin user to create or update."`

	// Kubernetes options
	Context   string `help:"Kubernetes context to use." name:"context"`
	Namespace string `help:"Kubernetes namespace to use." name:"namespace" default:"default"`

	// Local output option
	Local bool `help:"Write secret file locally instead of applying to k8s."`

	// ensureAdmin is injectable for testing; defaults to EnsureAdminUser.
	ensureAdmin AdminEnsurer
	// openDB is injectable for testing; defaults to OpenAdminDB.
	openDB DBOpener
}

// Run executes the set admin command.
func (c *AdminCmd) Run() error {
	if c.Email == "" {
		return fmt.Errorf("email argument is required")
	}

	// Resolve the DB opener
	openDB := c.openDB
	if openDB == nil {
		openDB = OpenAdminDB
	}

	// Open database connection
	db, err := openDB()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate the admin user table
	if err := db.AutoMigrate(&AdminUser{}); err != nil {
		return fmt.Errorf("failed to migrate admin user table: %w", err)
	}

	// Resolve the admin ensurer
	ensureAdmin := c.ensureAdmin
	if ensureAdmin == nil {
		ensureAdmin = EnsureAdminUser
	}

	// Create or update the admin user
	user, err := ensureAdmin(db, c.Email)
	if err != nil {
		return fmt.Errorf("failed to create or update admin user: %w", err)
	}

	// Generate RSA key pair for admin client credential flow
	privateKey, publicKey, err := GenerateRSAKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate admin client credential keys: %w", err)
	}

	// Build secret name from email (sanitize for k8s)
	secretName := adminSecretName(c.Email)

	secretData := map[string]string{
		"ADMIN_CLIENT_PRIVATE_KEY": privateKey,
		"ADMIN_CLIENT_PUBLIC_KEY":  publicKey,
		"ADMIN_EMAIL":              user.Email,
	}

	// Apply or write the secret
	if c.Local {
		filename := secretName + ".yaml"
		if err := WriteSecretFile(filename, secretName, c.Namespace, secretData); err != nil {
			return fmt.Errorf("failed to write admin secret file: %w", err)
		}
		fmt.Printf("wrote %s\n", filename)
	} else {
		if err := ApplySecret(secretName, c.Namespace, c.Context, secretData); err != nil {
			return fmt.Errorf("failed to apply admin secret: %w", err)
		}
		fmt.Printf("applied %s secret to kubernetes\n", secretName)
	}

	return nil
}

// EnsureAdminUser creates or updates an admin user with the given email.
func EnsureAdminUser(db *gorm.DB, email string) (*AdminUser, error) {
	user := &AdminUser{}

	// Try to find an existing user by email
	result := db.Where("email = ?", email).First(user)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to find admin user: %w", result.Error)
	}

	if result.Error == gorm.ErrRecordNotFound {
		// User does not exist — create them as admin
		user = &AdminUser{Email: email, IsAdmin: true}
		if err := db.Create(user).Error; err != nil {
			return nil, fmt.Errorf("failed to create admin user: %w", err)
		}
	} else if !user.IsAdmin {
		// User exists but is not admin — promote them
		if err := db.Model(user).Update("is_admin", true).Error; err != nil {
			return nil, fmt.Errorf("failed to update admin flag: %w", err)
		}
		user.IsAdmin = true
	}

	return user, nil
}

// adminSecretName produces a valid Kubernetes resource name from an email address.
func adminSecretName(email string) string {
	// Replace @ and . with - and lowercase
	sanitized := strings.ToLower(email)
	sanitized = strings.ReplaceAll(sanitized, "@", "-")
	sanitized = strings.ReplaceAll(sanitized, ".", "-")
	// Remove any characters that aren't alphanumeric or hyphens
	var sb strings.Builder
	for _, ch := range sanitized {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			sb.WriteRune(ch)
		}
	}
	return "wurbs-admin-" + sb.String()
}
