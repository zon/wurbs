package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"gorm.io/gorm"
)

// oidcClaims holds the claims extracted from an OIDC token.
type oidcClaims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
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
