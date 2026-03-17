package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"strings"
	"testing"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/require"
	"github.com/zon/chat/core/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}))
	return db
}

func generateTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func encodePEM(pub *rsa.PublicKey) string {
	der, _ := x509.MarshalPKIXPublicKey(pub)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

// customClaims is used in tests to add email to JWT tokens.
type customClaims struct {
	Email string `json:"email,omitempty"`
}

func signToken(t *testing.T, key *rsa.PrivateKey, stdClaims jwt.Claims, email string) string {
	t.Helper()
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	require.NoError(t, err)

	builder := jwt.Signed(signer).Claims(stdClaims)
	if email != "" {
		builder = builder.Claims(customClaims{Email: email})
	}
	token, err := builder.Serialize()
	require.NoError(t, err)
	return token
}

func setupTestConfig(t *testing.T, issuerURL, clientPublicKey string) {
	t.Helper()
	config.ResetCache()
	tmpDir := t.TempDir()
	t.Setenv("WURB_CONFIG", tmpDir)

	configPath := tmpDir + "/config.yaml"
	configContent := ""
	if issuerURL != "" {
		configContent = "oidcIssuer: " + issuerURL + "\n"
	}
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	adminPath := tmpDir + "/admin.yaml"
	adminContent := ""
	if clientPublicKey != "" {
		adminContent = "publicKey: |\n"
		for _, line := range splitLines(clientPublicKey) {
			adminContent += "  " + line + "\n"
		}
	}
	require.NoError(t, os.WriteFile(adminPath, []byte(adminContent), 0644))
}

func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// testClientPrivateKey is the RSA private key corresponding to the testClientPublicKey
// in auth.go. It is used to sign tokens for testing the test mode authentication.
const testClientPrivateKey = `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDaVLTVPXMkN4Lc
hY5n7hK4Vj4JWCLmJfJK2F0VITHgW8yVTVWaZOKzN9Q0e1r4VjvIJuFLV3drdT1S
VQy6Jq3oZ9k8yJ8XoS7d9wV7nN3qXdH1fM8YqK2pL5rT6vE1uW3sH4vK9jF2dP5t
qX3sL8mV1wK9yP6nH2vT4fL7eR5jG1dP8vW3qH9nK2pX5tR8vL4eK6jF3dP9tY7q
H2nK8vP5tX3rL6dT9vW4eK7jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK6jF2pL9tY
7qH3nK2vP8tX6rL5dT8vW3eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW4eK6jF3pL8
tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pAgMBAAECggEAVKHKl3tVNKxI7z+U3sK9
mB3vL5dT8nK2pX4tR7vW3eK6jF2pL9tY5qH3nK2vP7tX6rL5dT9vW4eK7jF2pL8t
Y6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8
tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL
8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2p
L9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2
pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF
2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6j
F3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6
jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK
7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4e
K7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4
eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW
4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9v
W3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8
vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT
9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9d
T8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9d
T8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8
dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6
dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rLAlternative<br>Key-----END PRIVATE KEY-----`
