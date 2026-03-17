package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/assert"
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

// --- UserFromContext tests ---

func TestUserFromContext_WithUser(t *testing.T) {
	u := &User{Email: "test@example.com"}
	ctx := ContextWithUser(context.Background(), u)

	got, err := UserFromContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, "test@example.com", got.Email)
}

func TestUserFromContext_WithoutUser(t *testing.T) {
	_, err := UserFromContext(context.Background())
	assert.ErrorIs(t, err, ErrNoUser)
}

func TestUserFromContext_NilUser(t *testing.T) {
	ctx := context.WithValue(context.Background(), userContextKey, (*User)(nil))
	_, err := UserFromContext(ctx)
	assert.ErrorIs(t, err, ErrNoUser)
}

// --- bearerToken tests ---

func TestBearerToken_Valid(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer mytoken123")

	token, err := bearerToken(r)
	require.NoError(t, err)
	assert.Equal(t, "mytoken123", token)
}

func TestBearerToken_CaseInsensitive(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "bearer mytoken123")

	token, err := bearerToken(r)
	require.NoError(t, err)
	assert.Equal(t, "mytoken123", token)
}

func TestBearerToken_MissingHeader(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	_, err := bearerToken(r)
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestBearerToken_WrongScheme(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Basic abc123")

	_, err := bearerToken(r)
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestBearerToken_EmptyToken(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer ")

	_, err := bearerToken(r)
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestBearerToken_NoSpace(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "BearerNoSpace")

	_, err := bearerToken(r)
	assert.ErrorIs(t, err, ErrUnauthorized)
}

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

// --- ClientMiddleware integration tests ---

func TestClientMiddleware_AcceptsAdminUser(t *testing.T) {
	db := setupTestDB(t)
	key := generateTestKey(t)

	// Create an admin user.
	admin := &User{Email: "admin@example.com", Subject: "admin-sub", IsAdmin: true}
	require.NoError(t, db.Create(admin).Error)

	// Set up config with the public key.
	setupTestConfig(t, "", encodePEM(&key.PublicKey))

	mw, err := ClientMiddleware(db)
	require.NoError(t, err)

	// Sign a valid token.
	stdClaims := jwt.Claims{
		Subject: "admin-sub",
		Expiry:  jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
	}
	token := signToken(t, key, stdClaims, "admin@example.com")

	// Build request.
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := UserFromContext(r.Context())
		assert.NoError(t, err)
		assert.Equal(t, "admin@example.com", u.Email)
		assert.True(t, u.IsAdmin)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestClientMiddleware_AcceptsTestUser(t *testing.T) {
	db := setupTestDB(t)
	key := generateTestKey(t)

	testUser := &User{Email: "test@example.com", Subject: "test-sub", IsTest: true}
	require.NoError(t, db.Create(testUser).Error)

	setupTestConfig(t, "", encodePEM(&key.PublicKey))

	mw, err := ClientMiddleware(db)
	require.NoError(t, err)

	stdClaims := jwt.Claims{
		Subject: "test-sub",
		Expiry:  jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
	}
	token := signToken(t, key, stdClaims, "test@example.com")

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := UserFromContext(r.Context())
		assert.NoError(t, err)
		assert.True(t, u.IsTest)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestClientMiddleware_RejectsRealUser(t *testing.T) {
	db := setupTestDB(t)
	key := generateTestKey(t)

	// Real user (not admin, not test).
	realUser := &User{Email: "real@example.com", Subject: "real-sub"}
	require.NoError(t, db.Create(realUser).Error)

	setupTestConfig(t, "", encodePEM(&key.PublicKey))

	mw, err := ClientMiddleware(db)
	require.NoError(t, err)

	stdClaims := jwt.Claims{
		Subject: "real-sub",
		Expiry:  jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
	}
	token := signToken(t, key, stdClaims, "real@example.com")

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for real user via client credentials")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestClientMiddleware_RejectsNoToken(t *testing.T) {
	db := setupTestDB(t)
	key := generateTestKey(t)

	setupTestConfig(t, "", encodePEM(&key.PublicKey))

	mw, err := ClientMiddleware(db)
	require.NoError(t, err)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called without token")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestClientMiddleware_MissingConfig(t *testing.T) {
	db := setupTestDB(t)

	// Point to empty config dir — no config files.
	config.ResetCache()
	tmpDir := t.TempDir()
	t.Setenv("WURB_CONFIG", tmpDir)

	_, err := ClientMiddleware(db)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load test admin credentials")
}

func TestClientMiddleware_MissingPublicKey(t *testing.T) {
	db := setupTestDB(t)

	// Config with no client_public_key.
	setupTestConfig(t, "", "")

	_, err := ClientMiddleware(db)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "publicKey not configured")
}

// --- OIDCMiddleware integration tests ---

func TestOIDCMiddleware_AcceptsRealUser(t *testing.T) {
	db := setupTestDB(t)
	key := generateTestKey(t)
	issuer := "https://auth.example.com"

	// Create a real user.
	realUser := &User{Email: "real@example.com", Subject: "oidc-real-sub"}
	require.NoError(t, db.Create(realUser).Error)

	setupTestConfig(t, issuer, "")

	// Mock JWKS fetching.
	origFetchJWKS := fetchJWKS
	fetchJWKS = func(issuerURL string) (*jose.JSONWebKeySet, error) {
		return &jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{
				{Key: &key.PublicKey, Algorithm: string(jose.RS256)},
			},
		}, nil
	}
	t.Cleanup(func() { fetchJWKS = origFetchJWKS })

	mw, err := OIDCMiddleware(db)
	require.NoError(t, err)

	// Sign an OIDC token.
	stdClaims := jwt.Claims{
		Subject: "oidc-real-sub",
		Issuer:  issuer,
		Expiry:  jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
	}
	token := signToken(t, key, stdClaims, "real@example.com")

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := UserFromContext(r.Context())
		assert.NoError(t, err)
		assert.Equal(t, "real@example.com", u.Email)
		assert.False(t, u.IsTest)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestOIDCMiddleware_AcceptsAdminUser(t *testing.T) {
	db := setupTestDB(t)
	key := generateTestKey(t)
	issuer := "https://auth.example.com"

	admin := &User{Email: "admin@example.com", Subject: "oidc-admin-sub", IsAdmin: true}
	require.NoError(t, db.Create(admin).Error)

	setupTestConfig(t, issuer, "")

	origFetchJWKS := fetchJWKS
	fetchJWKS = func(issuerURL string) (*jose.JSONWebKeySet, error) {
		return &jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{
				{Key: &key.PublicKey, Algorithm: string(jose.RS256)},
			},
		}, nil
	}
	t.Cleanup(func() { fetchJWKS = origFetchJWKS })

	mw, err := OIDCMiddleware(db)
	require.NoError(t, err)

	stdClaims := jwt.Claims{
		Subject: "oidc-admin-sub",
		Issuer:  issuer,
		Expiry:  jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
	}
	token := signToken(t, key, stdClaims, "admin@example.com")

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := UserFromContext(r.Context())
		assert.NoError(t, err)
		assert.True(t, u.IsAdmin)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestOIDCMiddleware_RejectsTestUser(t *testing.T) {
	db := setupTestDB(t)
	key := generateTestKey(t)
	issuer := "https://auth.example.com"

	testUser := &User{Email: "test@example.com", Subject: "oidc-test-sub", IsTest: true}
	require.NoError(t, db.Create(testUser).Error)

	setupTestConfig(t, issuer, "")

	origFetchJWKS := fetchJWKS
	fetchJWKS = func(issuerURL string) (*jose.JSONWebKeySet, error) {
		return &jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{
				{Key: &key.PublicKey, Algorithm: string(jose.RS256)},
			},
		}, nil
	}
	t.Cleanup(func() { fetchJWKS = origFetchJWKS })

	mw, err := OIDCMiddleware(db)
	require.NoError(t, err)

	stdClaims := jwt.Claims{
		Subject: "oidc-test-sub",
		Issuer:  issuer,
		Expiry:  jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
	}
	token := signToken(t, key, stdClaims, "test@example.com")

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for test user via OIDC")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestOIDCMiddleware_CreatesNewUser(t *testing.T) {
	db := setupTestDB(t)
	key := generateTestKey(t)
	issuer := "https://auth.example.com"

	setupTestConfig(t, issuer, "")

	origFetchJWKS := fetchJWKS
	fetchJWKS = func(issuerURL string) (*jose.JSONWebKeySet, error) {
		return &jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{
				{Key: &key.PublicKey, Algorithm: string(jose.RS256)},
			},
		}, nil
	}
	t.Cleanup(func() { fetchJWKS = origFetchJWKS })

	mw, err := OIDCMiddleware(db)
	require.NoError(t, err)

	stdClaims := jwt.Claims{
		Subject: "new-oidc-sub",
		Issuer:  issuer,
		Expiry:  jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
	}
	token := signToken(t, key, stdClaims, "newuser@example.com")

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := UserFromContext(r.Context())
		assert.NoError(t, err)
		assert.Equal(t, "new-oidc-sub", u.Subject)
		assert.Equal(t, "newuser@example.com", u.Email)
		assert.NotZero(t, u.ID)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify user was persisted.
	var count int64
	db.Model(&User{}).Where("subject = ?", "new-oidc-sub").Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestOIDCMiddleware_MissingConfig(t *testing.T) {
	db := setupTestDB(t)

	// Point to empty config dir.
	config.ResetCache()
	tmpDir := t.TempDir()
	t.Setenv("WURB_CONFIG", tmpDir)

	_, err := OIDCMiddleware(db)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load config")
}

func TestOIDCMiddleware_MissingIssuerURL(t *testing.T) {
	db := setupTestDB(t)

	setupTestConfig(t, "", "")

	_, err := OIDCMiddleware(db)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "oidc_issuer not configured")
}

// --- EnsureAdminUser ---

func TestEnsureAdminUser_UserNotFound(t *testing.T) {
	db := setupTestDB(t)

	_, err := EnsureAdminUser(db, "nonexistent@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestEnsureAdminUser_UpdatesExistingNonAdmin(t *testing.T) {
	db := setupTestDB(t)

	require.NoError(t, db.Create(&User{Email: "user@example.com", IsAdmin: false}).Error)

	user, err := EnsureAdminUser(db, "user@example.com")
	require.NoError(t, err)
	assert.True(t, user.IsAdmin)

	var found User
	require.NoError(t, db.Where("email = ?", "user@example.com").First(&found).Error)
	assert.True(t, found.IsAdmin)
}

func TestEnsureAdminUser_IdempotentForExistingAdmin(t *testing.T) {
	db := setupTestDB(t)

	require.NoError(t, db.Create(&User{Email: "admin@example.com", IsAdmin: true}).Error)

	user, err := EnsureAdminUser(db, "admin@example.com")
	require.NoError(t, err)

	assert.True(t, user.IsAdmin)
}

func TestEnsureAdminUser_RejectsTestUser(t *testing.T) {
	db := setupTestDB(t)

	require.NoError(t, db.Create(&User{Email: "test@example.com", IsTest: true, IsAdmin: false}).Error)

	_, err := EnsureAdminUser(db, "test@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrTestUserAdmin)

	var found User
	require.NoError(t, db.Where("email = ?", "test@example.com").First(&found).Error)
	assert.False(t, found.IsAdmin)
	assert.True(t, found.IsTest)
}

// --- EnsureTestAdminUser ---

func TestEnsureTestAdminUser_CreatesUser(t *testing.T) {
	db := setupTestDB(t)

	user, err := EnsureTestAdminUser(db, "test-admin@example.com")
	require.NoError(t, err)
	assert.Equal(t, "test-admin@example.com", user.Email)
	assert.True(t, user.IsAdmin)
	assert.True(t, user.IsTest)

	var found User
	require.NoError(t, db.Where("email = ?", "test-admin@example.com").First(&found).Error)
	assert.True(t, found.IsAdmin)
	assert.True(t, found.IsTest)
}

func TestEnsureTestAdminUser_UpdatesExistingUser(t *testing.T) {
	db := setupTestDB(t)

	require.NoError(t, db.Create(&User{Email: "test-admin@example.com", IsAdmin: false, IsTest: false}).Error)

	user, err := EnsureTestAdminUser(db, "test-admin@example.com")
	require.NoError(t, err)
	assert.True(t, user.IsAdmin)
	assert.True(t, user.IsTest)

	var found User
	require.NoError(t, db.Where("email = ?", "test-admin@example.com").First(&found).Error)
	assert.True(t, found.IsAdmin)
	assert.True(t, found.IsTest)
}

func TestEnsureTestAdminUser_IdempotentForExistingTestAdmin(t *testing.T) {
	db := setupTestDB(t)

	user1, err := EnsureTestAdminUser(db, "test-admin@example.com")
	require.NoError(t, err)

	user2, err := EnsureTestAdminUser(db, "test-admin@example.com")
	require.NoError(t, err)

	assert.Equal(t, user1.ID, user2.ID)
	assert.True(t, user2.IsAdmin)
	assert.True(t, user2.IsTest)
}

// --- User model tests ---

func TestUserModel_Fields(t *testing.T) {
	db := setupTestDB(t)

	user := &User{
		Email:   "full@example.com",
		Subject: "sub-full",
		IsAdmin: true,
		IsTest:  false,
	}
	require.NoError(t, db.Create(user).Error)

	var loaded User
	require.NoError(t, db.First(&loaded, user.ID).Error)

	assert.Equal(t, "full@example.com", loaded.Email)
	assert.Equal(t, "sub-full", loaded.Subject)
	assert.True(t, loaded.IsAdmin)
	assert.False(t, loaded.IsTest)
}

func TestUserModel_UniqueEmail(t *testing.T) {
	db := setupTestDB(t)

	u1 := &User{Email: "dup@example.com", Subject: "sub-1"}
	require.NoError(t, db.Create(u1).Error)

	u2 := &User{Email: "dup@example.com", Subject: "sub-2"}
	err := db.Create(u2).Error
	assert.Error(t, err)
}

func TestUserModel_UniqueSubject(t *testing.T) {
	db := setupTestDB(t)

	u1 := &User{Email: "a@example.com", Subject: "same-sub"}
	require.NoError(t, db.Create(u1).Error)

	u2 := &User{Email: "b@example.com", Subject: "same-sub"}
	err := db.Create(u2).Error
	assert.Error(t, err)
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
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL88T9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL
9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL
8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rL
6dT8vW3eK7jF2pL9tY5qH2nK3vP6tX5rL9dT8vW4eK6jF3pL8tY7qH2nK8vP5tX3rL
6dT9vW4eK7jF2pL9tY6qH3nK2vP7tX5rL8dT9vW3eK6jF3pL8tY5qH2nK3vP6tX5rL
9dT8vW4eK7jF2pL9tY6qH3nK2vP8tX5rLAlternative<br>Key-----END PRIVATE KEY-----`

// --- Test helper ---

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
