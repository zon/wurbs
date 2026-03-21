package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-jose/go-jose/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zon/chat/core/auth"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestOIDC(t *testing.T) {
	t.Helper()
	err := auth.InitOIDC(&auth.OIDCConfig{
		Issuer:        "https://auth.example.com",
		ClientID:      "test-client",
		ClientSecret:  "test-secret",
		RESTPort:      8080,
		SkipJWKSFetch: true,
	})
	require.NoError(t, err)
	auth.SetJWKS(&jose.JSONWebKeySet{Keys: []jose.JSONWebKey{}})
}

func doAuthJSON(t *testing.T, engine *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody *bytes.Buffer
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewBuffer(data)
	} else {
		reqBody = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func TestAuthRefresh_MissingBody(t *testing.T) {
	setupTestOIDC(t)

	r := gin.New()
	r.POST("/auth/refresh", func(c *gin.Context) {
		auth.Refresh(c.Writer, c.Request)
	})

	w := doAuthJSON(t, r, "POST", "/auth/refresh", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthRefresh_EmptyRefreshToken(t *testing.T) {
	setupTestOIDC(t)

	r := gin.New()
	r.POST("/auth/refresh", func(c *gin.Context) {
		auth.Refresh(c.Writer, c.Request)
	})

	w := doAuthJSON(t, r, "POST", "/auth/refresh", map[string]string{"refreshToken": ""})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthRefresh_InvalidToken(t *testing.T) {
	setupTestOIDC(t)

	r := gin.New()
	r.POST("/auth/refresh", func(c *gin.Context) {
		auth.Refresh(c.Writer, c.Request)
	})

	w := doAuthJSON(t, r, "POST", "/auth/refresh", map[string]string{"refreshToken": "invalid-token"})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAuthLogin_Redirects(t *testing.T) {
	setupTestOIDC(t)

	r := gin.New()
	r.GET("/auth/login", func(c *gin.Context) {
		auth.Login(c.Writer, c.Request)
	})

	req := httptest.NewRequest("GET", "/auth/login", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "https://auth.example.com/authorize")
}

func TestAuthLogin_WithState(t *testing.T) {
	setupTestOIDC(t)

	r := gin.New()
	r.GET("/auth/login", func(c *gin.Context) {
		auth.Login(c.Writer, c.Request)
	})

	req := httptest.NewRequest("GET", "/auth/login?state=mystate", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "mystate")
}

func TestAuthLogout_Redirects(t *testing.T) {
	setupTestOIDC(t)

	r := gin.New()
	r.POST("/auth/logout", func(c *gin.Context) {
		auth.Logout(c.Writer, c.Request)
	})

	req := httptest.NewRequest("POST", "/auth/logout", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "https://auth.example.com/v2/logout")
}

func TestAuthLogout_ClearsSession(t *testing.T) {
	setupTestOIDC(t)

	r := gin.New()
	r.POST("/auth/logout", func(c *gin.Context) {
		auth.Logout(c.Writer, c.Request)
	})

	req := httptest.NewRequest("POST", "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "test-refresh-token"})
	req.AddCookie(&http.Cookie{Name: "subject", Value: "test-subject"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)

	cookies := w.Result().Cookies()
	var refreshCookie, subjectCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
		}
		if c.Name == "subject" {
			subjectCookie = c
		}
	}
	assert.NotNil(t, refreshCookie)
	assert.NotNil(t, subjectCookie)
	pastTime := time.Now().Add(-1 * time.Hour)
	assert.True(t, refreshCookie.Expires.Before(pastTime))
	assert.True(t, subjectCookie.Expires.Before(pastTime))
}

func TestAuthCallback_MissingCode(t *testing.T) {
	setupTestOIDC(t)

	r := gin.New()
	r.GET("/auth/callback", func(c *gin.Context) {
		auth.Callback(nil)(c.Writer, c.Request)
	})

	req := httptest.NewRequest("GET", "/auth/callback", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "missing code")
}

func TestTokenSet_JSONSerialization(t *testing.T) {
	ts := auth.TokenSet{
		AccessToken:  "test-access-token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		RefreshToken: "test-refresh-token",
		IDToken:      "test-id-token",
	}

	data, err := json.Marshal(ts)
	require.NoError(t, err)

	var decoded auth.TokenSet
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, ts.AccessToken, decoded.AccessToken)
	assert.Equal(t, ts.TokenType, decoded.TokenType)
	assert.Equal(t, ts.ExpiresIn, decoded.ExpiresIn)
	assert.Equal(t, ts.RefreshToken, decoded.RefreshToken)
	assert.Equal(t, ts.IDToken, decoded.IDToken)
}

func TestRefreshInput_JSONSerialization(t *testing.T) {
	input := auth.RefreshInput{
		RefreshToken: "test-refresh",
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	var decoded auth.RefreshInput
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, input.RefreshToken, decoded.RefreshToken)
}
