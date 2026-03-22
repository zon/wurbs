package auth

import (
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- validateOIDCToken tests ---

func TestValidateOIDCToken_Valid(t *testing.T) {
	key := generateTestKey(t)
	issuer := "https://auth.example.com"

	stdClaims := jwt.Claims{
		Subject:  "oidc-sub-123",
		Issuer:   issuer,
		IssuedAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
		Expiry:   jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
	}
	token := signToken(t, key, stdClaims, "user@example.com")

	jwks := &jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{Key: &key.PublicKey, Algorithm: string(jose.RS256)},
		},
	}

	claims, err := validateOIDCToken(token, jwks, issuer)
	require.NoError(t, err)
	assert.Equal(t, "oidc-sub-123", claims.Subject)
	assert.Equal(t, "user@example.com", claims.Email)
}

func TestValidateOIDCToken_WrongIssuer(t *testing.T) {
	key := generateTestKey(t)

	stdClaims := jwt.Claims{
		Subject: "oidc-sub-123",
		Issuer:  "https://wrong-issuer.com",
		Expiry:  jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
	}
	token := signToken(t, key, stdClaims, "")

	jwks := &jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{Key: &key.PublicKey, Algorithm: string(jose.RS256)},
		},
	}

	_, err := validateOIDCToken(token, jwks, "https://auth.example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestValidateOIDCToken_NoMatchingKey(t *testing.T) {
	signingKey := generateTestKey(t)
	wrongKey := generateTestKey(t)

	stdClaims := jwt.Claims{
		Subject: "oidc-sub-123",
		Issuer:  "https://auth.example.com",
		Expiry:  jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
	}
	token := signToken(t, signingKey, stdClaims, "")

	jwks := &jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{Key: &wrongKey.PublicKey, Algorithm: string(jose.RS256)},
		},
	}

	_, err := validateOIDCToken(token, jwks, "https://auth.example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no matching key")
}

func TestValidateOIDCToken_MissingSubject(t *testing.T) {
	key := generateTestKey(t)
	issuer := "https://auth.example.com"

	stdClaims := jwt.Claims{
		Issuer: issuer,
		Expiry: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
	}
	token := signToken(t, key, stdClaims, "")

	jwks := &jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{Key: &key.PublicKey, Algorithm: string(jose.RS256)},
		},
	}

	_, err := validateOIDCToken(token, jwks, issuer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing subject")
}

// --- resolveOIDCUser tests ---

func TestResolveOIDCUser_CreatesNewUser(t *testing.T) {
	db := setupTestDB(t)

	claims := &oidcClaims{Subject: "sub-new", Email: "new@example.com"}
	user, err := resolveOIDCUser(db, claims)

	require.NoError(t, err)
	assert.Equal(t, "sub-new", user.Subject)
	assert.Equal(t, "new@example.com", user.Email)
	assert.NotZero(t, user.ID)
}

func TestResolveOIDCUser_FindsExisting(t *testing.T) {
	db := setupTestDB(t)

	existing := &User{Subject: "sub-exist", Email: "exist@example.com"}
	require.NoError(t, db.Create(existing).Error)

	claims := &oidcClaims{Subject: "sub-exist", Email: "exist@example.com"}
	user, err := resolveOIDCUser(db, claims)

	require.NoError(t, err)
	assert.Equal(t, existing.ID, user.ID)
}

func TestResolveOIDCUser_UpdatesEmail(t *testing.T) {
	db := setupTestDB(t)

	existing := &User{Subject: "sub-update", Email: "old@example.com"}
	require.NoError(t, db.Create(existing).Error)

	claims := &oidcClaims{Subject: "sub-update", Email: "new@example.com"}
	user, err := resolveOIDCUser(db, claims)

	require.NoError(t, err)
	assert.Equal(t, "new@example.com", user.Email)
}
