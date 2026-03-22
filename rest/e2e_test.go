package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zon/chat/core/config"
	"github.com/zon/chat/core/pg"
)

func startTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	t.Setenv("WURB_CONFIG", "/workspace/repo/config")
	config.ResetCache()
	config.SetTestMode(true)
	db, err := pg.Open()
	require.NoError(t, err)

	// Minimal auth middleware for testing
	authMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// In test mode, we might need a way to mock the user
			// For now, assume some test user is injected or handled by logic
			next.ServeHTTP(w, r)
		})
	}

	// Use TestAdmin to setup the client public key as main does
	// initClientPublicKey(db)

	engine := New(Deps{DB: db, NATS: nil}, authMW)
	return httptest.NewServer(engine)
}

func TestE2E_Health(t *testing.T) {
	// t.Skip("skipping test that requires database")
	server := startTestServer(t)
	if server == nil {
		return
	}
	defer server.Close()
	// ...

	resp, err := http.Get(server.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestE2E_CreateChannel(t *testing.T) {
	t.Skip("skipping test that requires database")
	server := startTestServer(t)
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	body, _ := json.Marshal(map[string]any{
		"name":    "test-chan",
		"is_test": true,
	})

	req, _ := http.NewRequest("POST", server.URL+"/channels", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}
