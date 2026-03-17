package auth

import (
	"encoding/pem"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- parseRSAPublicKey tests ---

func TestParseRSAPublicKey_Valid(t *testing.T) {
	key := generateTestKey(t)
	pemStr := encodePEM(&key.PublicKey)

	parsed, err := parseRSAPublicKey(pemStr)
	require.NoError(t, err)
	assert.Equal(t, key.PublicKey.E, parsed.E)
}

func TestParseRSAPublicKey_NoPEMBlock(t *testing.T) {
	_, err := parseRSAPublicKey("not a pem block")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no PEM block")
}

func TestParseRSAPublicKey_InvalidDER(t *testing.T) {
	block := &pem.Block{Type: "PUBLIC KEY", Bytes: []byte("invalid")}
	pemStr := string(pem.EncodeToMemory(block))

	_, err := parseRSAPublicKey(pemStr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse public key")
}

// --- validateClientToken tests ---

func TestValidateClientToken_Valid(t *testing.T) {
	key := generateTestKey(t)

	stdClaims := jwt.Claims{
		Subject:  "user-123",
		IssuedAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
		Expiry:   jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
	}
	token := signToken(t, key, stdClaims, "admin@example.com")

	claims, err := validateClientToken(token, &key.PublicKey)
	require.NoError(t, err)
	assert.Equal(t, "user-123", claims.Subject)
	assert.Equal(t, "admin@example.com", claims.Email)
}

func TestValidateClientToken_Expired(t *testing.T) {
	key := generateTestKey(t)

	stdClaims := jwt.Claims{
		Subject:  "user-123",
		IssuedAt: jwt.NewNumericDate(time.Now().Add(-10 * time.Minute)),
		Expiry:   jwt.NewNumericDate(time.Now().Add(-5 * time.Minute)),
	}
	token := signToken(t, key, stdClaims, "")

	_, err := validateClientToken(token, &key.PublicKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestValidateClientToken_WrongKey(t *testing.T) {
	signingKey := generateTestKey(t)
	wrongKey := generateTestKey(t)

	stdClaims := jwt.Claims{
		Subject: "user-123",
		Expiry:  jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
	}
	token := signToken(t, signingKey, stdClaims, "")

	_, err := validateClientToken(token, &wrongKey.PublicKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signature invalid")
}

func TestValidateClientToken_MissingSubjectAndEmail(t *testing.T) {
	key := generateTestKey(t)

	stdClaims := jwt.Claims{
		Expiry: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
	}
	token := signToken(t, key, stdClaims, "")

	_, err := validateClientToken(token, &key.PublicKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing subject and email")
}

func TestValidateClientToken_InvalidTokenString(t *testing.T) {
	key := generateTestKey(t)
	_, err := validateClientToken("not-a-jwt", &key.PublicKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

// --- resolveClientUser tests ---

func TestResolveClientUser_FindsByEmail(t *testing.T) {
	db := setupTestDB(t)

	existing := &User{Email: "admin@example.com", Subject: "sub-admin", IsAdmin: true}
	require.NoError(t, db.Create(existing).Error)

	claims := &clientClaims{Email: "admin@example.com"}
	user, err := resolveClientUser(db, claims)

	require.NoError(t, err)
	assert.Equal(t, existing.ID, user.ID)
	assert.True(t, user.IsAdmin)
}

func TestResolveClientUser_FindsBySubject(t *testing.T) {
	db := setupTestDB(t)

	existing := &User{Subject: "sub-test", Email: "test@example.com", IsTest: true}
	require.NoError(t, db.Create(existing).Error)

	claims := &clientClaims{Subject: "sub-test"}
	user, err := resolveClientUser(db, claims)

	require.NoError(t, err)
	assert.Equal(t, existing.ID, user.ID)
}

func TestResolveClientUser_NotFound(t *testing.T) {
	db := setupTestDB(t)

	claims := &clientClaims{Email: "nobody@example.com"}
	_, err := resolveClientUser(db, claims)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}
