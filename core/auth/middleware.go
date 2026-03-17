package auth

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"strings"

	"github.com/zon/chat/core/config"
	"gorm.io/gorm"
)

// OIDCMiddleware returns HTTP middleware that validates OIDC bearer tokens.
// It loads the issuer URL from config, fetches the provider's JWKS,
// validates the token, and resolves the user by subject. The user is stored
// in the request context for retrieval via UserFromContext.
//
// The db handle is used to look up or create users. Only real and admin
// users may authenticate via OIDC; test users are rejected.
func OIDCMiddleware(db *gorm.DB) (func(http.Handler) http.Handler, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("auth: failed to load config: %w", err)
	}

	if cfg.OIDCIssuer == "" {
		return nil, fmt.Errorf("auth: oidc_issuer not configured in config.yaml")
	}

	jwks, err := fetchJWKS(cfg.OIDCIssuer)
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

			claims, err := validateOIDCToken(token, jwks, cfg.OIDCIssuer)
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
// public key from test admin credentials.
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
		tree, err := config.Dir()
		if err != nil {
			return nil, err
		}

		var ta TestAdmin
		if err := ta.Read(tree.TestAdmin); err != nil {
			return nil, fmt.Errorf("auth: failed to load test admin credentials: %w", err)
		}

		if ta.PublicKey == "" {
			return nil, fmt.Errorf("auth: publicKey not configured in %s", tree.TestAdmin)
		}

		pubKey, err = parseRSAPublicKey(ta.PublicKey)
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
