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
	"github.com/zon/chat/core/auth"
	"github.com/zon/chat/core/channel"
	"github.com/zon/chat/core/message"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func startTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&auth.User{}, &channel.Channel{}, &channel.Membership{}, &message.Message{}))

	// Need a test admin to allow channel creation etc.
	u := &auth.User{Email: "test-admin@test.com", Subject: "sub-test", IsAdmin: true, IsTest: true}
	require.NoError(t, db.Create(u).Error)

	// Mock auth to inject the user.
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(auth.ContextWithUser(r.Context(), u))
			next.ServeHTTP(w, r)
		})
	}

	engine := New(Deps{DB: db, NATS: nil}, mw)
	return httptest.NewServer(engine)
}

func TestE2E_Health(t *testing.T) {
	server := startTestServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestE2E_CreateChannel(t *testing.T) {
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
