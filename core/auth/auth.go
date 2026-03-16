package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/zon/chat/core/config"
	"gorm.io/gorm"
)

// User is the application user model. The auth module owns this type.
type User struct {
	gorm.Model
	Email   string `gorm:"uniqueIndex"`
	Subject string `gorm:"uniqueIndex"`
	IsAdmin bool
	IsTest  bool
}

// Secret holds auth-related secrets loaded from secret.yaml.
type secret struct {
	Auth struct {
		IssuerURL       string `yaml:"issuer_url"`
		ClientPublicKey string `yaml:"client_public_key"`
	} `yaml:"auth"`
}

type contextKey int

const userContextKey contextKey = iota

var testMode bool

func SetTestMode(enabled bool) {
	testMode = enabled
}

// testClientPublicKey is a shared RSA public key used for client credential
// authentication in test mode. All test users can authenticate using tokens
// signed with the corresponding private key.
const testClientPublicKey = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAu1SU1LfVLPHCozMxH2Mo
4lgOEePzNm0tRgeLezV6ffAt0gunVTLw7onLRnrq0/IzW7yWR7QkrmBL7jTKEn5u
+qKhbwKfBstIs+bMY2Zkp18gnTxKLxoS2tFczGkPLPgizskuemMghRniWaoLcyeh
kd3qqGElvW/VDL5AaWTg0nLVkjRo9z+40RQzuVaE8AkAFmxZzow3x+VJYKdjykkJ
0iT9wCS0DRTXu269V264Vf/3jvredZiKRkgwlL9xNAwxXFg0x/XFw005UWVRIkdg
cKWTjpBP2dPwVZ4WWC+9aGVd+Gyn1o0CLelf4rEjGoXbAAEgAqeGUxrcIlbjXfbcw
IDAQAB
-----END PUBLIC KEY-----`

// Errors returned by the auth module.
var (
	ErrNoUser       = errors.New("auth: no authenticated user in context")
	ErrUnauthorized = errors.New("auth: unauthorized")
)

// UserFromContext extracts the authenticated user from the context.
// Returns ErrNoUser if no user has been set by auth middleware.
func UserFromContext(ctx context.Context) (*User, error) {
	u, ok := ctx.Value(userContextKey).(*User)
	if !ok || u == nil {
		return nil, ErrNoUser
	}
	return u, nil
}

// ContextWithUser returns a new context with the authenticated user set.
// This is used by auth middleware internally and may be used in tests
// to inject a user into the request context.
func ContextWithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

// OIDCMiddleware returns HTTP middleware that validates OIDC bearer tokens.
// It loads the issuer URL from auth secrets, fetches the provider's JWKS,
// validates the token, and resolves the user by subject. The user is stored
// in the request context for retrieval via UserFromContext.
//
// The db handle is used to look up or create users. Only real and admin
// users may authenticate via OIDC; test users are rejected.
func OIDCMiddleware(db *gorm.DB) (func(http.Handler) http.Handler, error) {
	var s secret
	if err := config.LoadSecret(&s); err != nil {
		return nil, fmt.Errorf("auth: failed to load secrets: %w", err)
	}

	if s.Auth.IssuerURL == "" {
		return nil, fmt.Errorf("auth: issuer_url not configured in secrets")
	}

	jwks, err := fetchJWKS(s.Auth.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to fetch JWKS: %w", err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := bearerToken(r)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			claims, err := validateOIDCToken(token, jwks, s.Auth.IssuerURL)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			user, err := resolveOIDCUser(db, claims)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			// Test users cannot authenticate via OIDC.
			if user.IsTest {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			r = r.WithContext(ContextWithUser(r.Context(), user))
			next.ServeHTTP(w, r)
		})
	}, nil
}

// ClientMiddleware returns HTTP middleware that validates client credential
// JWT tokens signed with RSA keys.
//
// In test mode, it uses a shared test public key. Otherwise, it loads the
// public key from auth secrets.
//
// The db handle is used to look up users. Only admin and test users may
// authenticate via client credentials; real users are rejected.
func ClientMiddleware(db *gorm.DB) (func(http.Handler) http.Handler, error) {
	var pubKey *rsa.PublicKey
	var err error

	if testMode {
		pubKey, err = parseRSAPublicKey(testClientPublicKey)
		if err != nil {
			return nil, fmt.Errorf("auth: failed to parse test client public key: %w", err)
		}
	} else {
		var s secret
		if err := config.LoadSecret(&s); err != nil {
			return nil, fmt.Errorf("auth: failed to load secrets: %w", err)
		}

		if s.Auth.ClientPublicKey == "" {
			return nil, fmt.Errorf("auth: client_public_key not configured in secrets")
		}

		pubKey, err = parseRSAPublicKey(s.Auth.ClientPublicKey)
		if err != nil {
			return nil, fmt.Errorf("auth: failed to parse client public key: %w", err)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := bearerToken(r)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			claims, err := validateClientToken(token, pubKey)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			user, err := resolveClientUser(db, claims)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			// Real users (not admin, not test) cannot use client credentials.
			if !user.IsAdmin && !user.IsTest {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			r = r.WithContext(ContextWithUser(r.Context(), user))
			next.ServeHTTP(w, r)
		})
	}, nil
}

// bearerToken extracts the bearer token from the Authorization header.
func bearerToken(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", ErrUnauthorized
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", ErrUnauthorized
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", ErrUnauthorized
	}
	return token, nil
}

// oidcClaims holds the claims extracted from an OIDC token.
type oidcClaims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
}

// clientClaims holds the claims extracted from a client credential token.
type clientClaims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
}

// GenerateRSAKeyPair generates a 2048-bit RSA key pair and returns the
// PEM-encoded private key and public key as strings.
func GenerateRSAKeyPair() (privateKeyPEM, publicKeyPEM string, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("auth: failed to generate RSA key: %w", err)
	}

	privBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	privateKeyPEM = string(pem.EncodeToMemory(privBlock))

	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("auth: failed to marshal public key: %w", err)
	}
	pubBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	}
	publicKeyPEM = string(pem.EncodeToMemory(pubBlock))

	return privateKeyPEM, publicKeyPEM, nil
}

// parseRSAPublicKey parses a PEM-encoded RSA public key.
func parseRSAPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("auth: no PEM block found in public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to parse public key: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("auth: key is not an RSA public key")
	}
	return rsaPub, nil
}

// validateOIDCToken validates a JWT against the OIDC provider's JWKS and
// returns the extracted claims.
func validateOIDCToken(tokenStr string, jwks *jose.JSONWebKeySet, issuer string) (*oidcClaims, error) {
	tok, err := jwt.ParseSigned(tokenStr, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		return nil, fmt.Errorf("auth: failed to parse OIDC token: %w", err)
	}

	// Find the matching key from JWKS.
	var allClaims jwt.Claims
	var custom oidcClaims
	verified := false
	for _, key := range jwks.Keys {
		if err := tok.Claims(key, &allClaims, &custom); err == nil {
			verified = true
			break
		}
	}
	if !verified {
		return nil, fmt.Errorf("auth: no matching key found in JWKS")
	}

	// Validate standard claims.
	expected := jwt.Expected{
		Issuer: issuer,
		Time:   time.Now(),
	}
	if err := allClaims.Validate(expected); err != nil {
		return nil, fmt.Errorf("auth: token validation failed: %w", err)
	}

	custom.Subject = allClaims.Subject
	if custom.Subject == "" {
		return nil, fmt.Errorf("auth: token missing subject claim")
	}

	return &custom, nil
}

// validateClientToken validates a JWT signed with the client RSA key and
// returns the extracted claims.
func validateClientToken(tokenStr string, pubKey *rsa.PublicKey) (*clientClaims, error) {
	tok, err := jwt.ParseSigned(tokenStr, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		return nil, fmt.Errorf("auth: failed to parse client token: %w", err)
	}

	var allClaims jwt.Claims
	var custom clientClaims
	if err := tok.Claims(pubKey, &allClaims, &custom); err != nil {
		return nil, fmt.Errorf("auth: client token signature invalid: %w", err)
	}

	// Validate expiry.
	expected := jwt.Expected{
		Time: time.Now(),
	}
	if err := allClaims.Validate(expected); err != nil {
		return nil, fmt.Errorf("auth: client token validation failed: %w", err)
	}

	custom.Subject = allClaims.Subject
	if custom.Subject == "" && custom.Email == "" {
		return nil, fmt.Errorf("auth: client token missing subject and email claims")
	}

	return &custom, nil
}

// resolveOIDCUser finds or creates a user by OIDC subject.
func resolveOIDCUser(db *gorm.DB, claims *oidcClaims) (*User, error) {
	var user User
	result := db.Where("subject = ?", claims.Subject).First(&user)
	if result.Error == nil {
		// Update email if changed.
		if claims.Email != "" && user.Email != claims.Email {
			db.Model(&user).Update("email", claims.Email)
			user.Email = claims.Email
		}
		return &user, nil
	}
	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, result.Error
	}
	// Create new user.
	user = User{
		Subject: claims.Subject,
		Email:   claims.Email,
	}
	if err := db.Create(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// resolveClientUser finds a user by email from client credential claims.
func resolveClientUser(db *gorm.DB, claims *clientClaims) (*User, error) {
	var user User

	if claims.Email != "" {
		result := db.Where("email = ?", claims.Email).First(&user)
		if result.Error == nil {
			return &user, nil
		}
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}

	if claims.Subject != "" {
		result := db.Where("subject = ?", claims.Subject).First(&user)
		if result.Error == nil {
			return &user, nil
		}
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}

	return nil, fmt.Errorf("auth: user not found for client credentials")
}

// fetchJWKS fetches the JWKS from the OIDC provider's discovery endpoint.
// It is a package-level variable so tests can replace it.
var fetchJWKS = fetchJWKSFromIssuer

func fetchJWKSFromIssuer(issuerURL string) (*jose.JSONWebKeySet, error) {
	// Discover the JWKS URI from the OIDC discovery document.
	wellKnown := strings.TrimRight(issuerURL, "/") + "/.well-known/openid-configuration"

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(wellKnown)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to fetch OIDC discovery: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth: OIDC discovery returned status %d", resp.StatusCode)
	}

	var discovery struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := decodeJSON(resp.Body, &discovery); err != nil {
		return nil, fmt.Errorf("auth: failed to decode OIDC discovery: %w", err)
	}

	if discovery.JWKSURI == "" {
		return nil, fmt.Errorf("auth: jwks_uri not found in OIDC discovery")
	}

	// Fetch the JWKS.
	jwksResp, err := client.Get(discovery.JWKSURI)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to fetch JWKS: %w", err)
	}
	defer jwksResp.Body.Close()

	if jwksResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth: JWKS endpoint returned status %d", jwksResp.StatusCode)
	}

	var jwks jose.JSONWebKeySet
	if err := decodeJSON(jwksResp.Body, &jwks); err != nil {
		return nil, fmt.Errorf("auth: failed to decode JWKS: %w", err)
	}

	return &jwks, nil
}

// decodeJSON decodes JSON from a reader into the target value.
func decodeJSON(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}

// EnsureAdminUser creates or updates a user with the given email, ensuring IsAdmin is true.
func EnsureAdminUser(db *gorm.DB, email string) (*User, error) {
	user := &User{}

	result := db.Where("email = ?", email).First(user)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("auth: failed to find user: %w", result.Error)
	}

	if result.Error == gorm.ErrRecordNotFound {
		user = &User{Email: email, IsAdmin: true}
		if err := db.Create(user).Error; err != nil {
			return nil, fmt.Errorf("auth: failed to create user: %w", err)
		}
	} else if !user.IsAdmin {
		if err := db.Model(user).Update("is_admin", true).Error; err != nil {
			return nil, fmt.Errorf("auth: failed to update admin flag: %w", err)
		}
		user.IsAdmin = true
	}

	return user, nil
}
