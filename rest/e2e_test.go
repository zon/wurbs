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
)

func startTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	t.Skip("skipping test that requires database")
	return nil
}

func TestE2E_Health(t *testing.T) {
	t.Skip("skipping test that requires database")
	server := startTestServer(t)
	defer server.Close()

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
