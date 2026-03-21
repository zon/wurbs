package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/require"
	"github.com/zon/chat/core/config"
	"github.com/zon/chat/core/user"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&user.User{}))
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

	tree, err := config.Dir()
	require.NoError(t, err)

	configContent := ""
	if issuerURL != "" {
		configContent = "oidcIssuer: " + issuerURL + "\n"
	}
	require.NoError(t, os.WriteFile(tree.Config, []byte(configContent), 0644))

	ta := TestAdmin{PublicKey: clientPublicKey}
	require.NoError(t, ta.Write(tree.TestAdmin))
}
