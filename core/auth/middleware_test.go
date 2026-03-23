package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zon/chat/core/config"
	usermod "github.com/zon/chat/core/user"
)

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

// --- ClientMiddleware integration tests ---

func TestClientMiddleware_AcceptsAdminUser(t *testing.T) {
	db := setupTestDB(t)
	key := generateTestKey(t)

	// Create an admin user.
	admin := &usermod.User{Email: "admin@example.com", Subject: "admin-sub", IsAdmin: true}
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

	testUser := &usermod.User{Email: "test@example.com", Subject: "test-sub", IsTest: true}
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
	realUser := &usermod.User{Email: "real@example.com", Subject: "real-sub"}
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
	realUser := &usermod.User{Email: "real@example.com", Subject: "oidc-real-sub"}
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

	admin := &usermod.User{Email: "admin@example.com", Subject: "oidc-admin-sub", IsAdmin: true}
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

	testUser := &usermod.User{Email: "test@example.com", Subject: "oidc-test-sub", IsTest: true}
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
	db.Model(&usermod.User{}).Where("subject = ?", "new-oidc-sub").Count(&count)
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
