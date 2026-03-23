package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	usermod "github.com/zon/chat/core/user"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

var (
	authMu          sync.RWMutex
	oauth2Config    *oauth2.Config
	issuerURL       string
	oauth2JWKS      *jose.JSONWebKeySet
	clientPublicKey []byte
)

type TokenSet struct {
	AccessToken  string `json:"accessToken"`
	TokenType    string `json:"tokenType"`
	ExpiresIn    int    `json:"expiresIn"`
	RefreshToken string `json:"refreshToken,omitempty"`
	IDToken      string `json:"idToken,omitempty"`
}

type RefreshInput struct {
	RefreshToken string `json:"refreshToken"`
}

type ClientTokenInput struct {
	GrantType           string `json:"grantType"`
	ClientID            string `json:"clientId"`
	ClientAssertion     string `json:"clientAssertion"`
	ClientAssertionType string `json:"clientAssertionType"`
}

type OIDCConfig struct {
	Issuer        string
	ClientID      string
	ClientSecret  string
	RESTPort      int
	SkipJWKSFetch bool
}

func InitOIDC(cfg *OIDCConfig) error {
	if cfg == nil {
		return nil
	}
	authMu.Lock()
	defer authMu.Unlock()
	issuerURL = cfg.Issuer
	oauth2Config = &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  fmt.Sprintf("http://localhost:%d/auth/callback", cfg.RESTPort),
		Endpoint: oauth2.Endpoint{
			AuthURL:  cfg.Issuer + "/authorize",
			TokenURL: cfg.Issuer + "/oauth/token",
		},
		Scopes: []string{"openid", "email", "profile"},
	}

	if cfg.SkipJWKSFetch {
		return nil
	}

	var err error
	oauth2JWKS, err = fetchJWKS(cfg.Issuer)
	if err != nil {
		return fmt.Errorf("auth: failed to fetch JWKS: %w", err)
	}

	return nil
}

func SetJWKS(jwks *jose.JSONWebKeySet) {
	authMu.Lock()
	defer authMu.Unlock()
	oauth2JWKS = jwks
}

func Login(w http.ResponseWriter, r *http.Request) {
	authMu.RLock()
	cfg := oauth2Config
	authMu.RUnlock()
	if cfg == nil {
		http.Error(w, "OIDC not configured", http.StatusInternalServerError)
		return
	}

	state := r.URL.Query().Get("state")
	url := cfg.AuthCodeURL(state, oauth2.AccessTypeOnline, oauth2.ApprovalForce)
	http.Redirect(w, r, url, http.StatusFound)
}

func Callback(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authMu.RLock()
		cfg := oauth2Config
		jwks := oauth2JWKS
		iss := issuerURL
		authMu.RUnlock()
		if cfg == nil {
			http.Error(w, "OIDC not configured", http.StatusInternalServerError)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code parameter", http.StatusBadRequest)
			return
		}

		token, err := cfg.Exchange(r.Context(), code)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to exchange code: %v", err), http.StatusInternalServerError)
			return
		}

		rawIDToken, ok := token.Extra("id_token").(string)
		if !ok {
			http.Error(w, "no id_token in response", http.StatusInternalServerError)
			return
		}

		claims, err := validateOIDCToken(rawIDToken, jwks, iss)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to verify id_token: %v", err), http.StatusInternalServerError)
			return
		}

		user, err := resolveOIDCUser(db, claims)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to resolve user: %v", err), http.StatusInternalServerError)
			return
		}

		storeTokenInSession(w, user.Subject, token.RefreshToken)

		tokenSet := TokenSet{
			AccessToken:  token.AccessToken,
			TokenType:    "Bearer",
			ExpiresIn:    int(token.Expiry.Second()),
			RefreshToken: token.RefreshToken,
			IDToken:      rawIDToken,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokenSet)
	}
}

func Logout(w http.ResponseWriter, r *http.Request) {
	authMu.RLock()
	cfg := oauth2Config
	iss := issuerURL
	authMu.RUnlock()
	if cfg == nil {
		http.Error(w, "OIDC not configured", http.StatusInternalServerError)
		return
	}

	refreshToken := getRefreshTokenFromSession(r)
	if refreshToken != "" {
		tokenSource := cfg.TokenSource(r.Context(), &oauth2.Token{RefreshToken: refreshToken})
		_, _ = tokenSource.Token()
	}

	clearSession(w)

	endSessionURL := iss + "/v2/logout?post_logout_redirect_uri="
	http.Redirect(w, r, endSessionURL, http.StatusFound)
}

func Refresh(w http.ResponseWriter, r *http.Request) {
	authMu.RLock()
	cfg := oauth2Config
	authMu.RUnlock()
	if cfg == nil {
		http.Error(w, "OIDC not configured", http.StatusInternalServerError)
		return
	}

	var input RefreshInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if input.RefreshToken == "" {
		http.Error(w, "missing refresh token", http.StatusBadRequest)
		return
	}

	token := &oauth2.Token{RefreshToken: input.RefreshToken}
	tokenSource := cfg.TokenSource(r.Context(), token)
	newToken, err := tokenSource.Token()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to refresh token: %v", err), http.StatusInternalServerError)
		return
	}

	rawIDToken, ok := newToken.Extra("id_token").(string)
	if !ok {
		rawIDToken = ""
	}

	tokenSet := TokenSet{
		AccessToken:  newToken.AccessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(newToken.Expiry.Second()),
		RefreshToken: newToken.RefreshToken,
		IDToken:      rawIDToken,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokenSet)
}

func SetClientPublicKey(pem string) {
	authMu.Lock()
	defer authMu.Unlock()
	clientPublicKey = []byte(pem)
}

func ClientToken(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authMu.RLock()
		key := clientPublicKey
		authMu.RUnlock()
		if len(key) == 0 {
			http.Error(w, "client credentials not configured", http.StatusInternalServerError)
			return
		}

		var input ClientTokenInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if input.GrantType != "client_credentials" {
			http.Error(w, "unsupported grant type", http.StatusBadRequest)
			return
		}

		if input.ClientID == "" {
			http.Error(w, "client_id required", http.StatusBadRequest)
			return
		}

		if input.ClientAssertion == "" {
			http.Error(w, "client_assertion required", http.StatusBadRequest)
			return
		}

		pubKey, err := parseRSAPublicKey(string(key))
		if err != nil {
			http.Error(w, "failed to parse client public key", http.StatusInternalServerError)
			return
		}

		claims, err := validateClientToken(input.ClientAssertion, pubKey)
		if err != nil {
			http.Error(w, "invalid client assertion", http.StatusUnauthorized)
			return
		}

		user, err := resolveClientUser(db, claims)
		if err != nil {
			http.Error(w, "user not found", http.StatusUnauthorized)
			return
		}

		if !user.IsTest && !user.IsAdmin {
			http.Error(w, "real users must authenticate via OIDC", http.StatusForbidden)
			return
		}

		accessToken := generateAccessToken(user)

		tokenSet := TokenSet{
			AccessToken: accessToken,
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokenSet)
	}
}

func generateAccessToken(user *usermod.User) string {
	return fmt.Sprintf("client_%d_%s", user.ID, user.Email)
}

func getRefreshTokenFromSession(r *http.Request) string {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		return ""
	}
	return cookie.Value
}

func storeTokenInSession(w http.ResponseWriter, subject, refreshToken string) {
	if refreshToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    refreshToken,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(30 * 24 * time.Hour),
		})
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "subject",
		Value:    subject,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(30 * 24 * time.Hour),
	})
}

func clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(-1 * time.Hour),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "subject",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(-1 * time.Hour),
	})
}

func SessionAuthMiddleware(db *gorm.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("subject")
			if err != nil || cookie.Value == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			var user usermod.User
			result := db.Where("subject = ?", cookie.Value).First(&user)
			if result.Error != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			if user.IsTest {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			r = r.WithContext(ContextWithUser(r.Context(), &user))
			next.ServeHTTP(w, r)
		})
	}
}

func FindUserBySubject(db *gorm.DB, subject string) (*usermod.User, error) {
	var user usermod.User
	result := db.Where("subject = ?", subject).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

func FindOrCreateUserByEmail(db *gorm.DB, email, subject string) (*usermod.User, error) {
	var user usermod.User
	result := db.Where("email = ?", email).First(&user)

	if result.Error == nil {
		if user.Subject == "" && subject != "" {
			user.Subject = subject
			db.Save(&user)
		}
		return &user, nil
	}

	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, result.Error
	}

	user = usermod.User{
		Email:    email,
		Subject:  subject,
		IsActive: true,
	}
	if err := db.Create(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func FindUserByEmail(db *gorm.DB, email string) (*usermod.User, error) {
	var user usermod.User
	result := db.Where("email = ?", email).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}
