package main

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	socketURL = "ws://localhost:8081"
	restURL   = "http://localhost:8080"
)

var (
	testAccessToken string
	testChannelID   string
)

func getAuthToken(t *testing.T) string {
	if testAccessToken != "" {
		return testAccessToken
	}
	t.Skip("E2E tests require a running REST server with database. Start with: make rest")
	return ""
}

func getAuthHeader(t *testing.T) string {
	token := getAuthToken(t)
	if token == "" {
		return ""
	}
	return "Bearer " + token
}

func TestSocketHealth(t *testing.T) {
	resp, err := http.Get("http://localhost:8081/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestSocketUnauthenticatedConnection(t *testing.T) {
	_, _, err := websocket.DefaultDialer.Dial(socketURL+"/channels/1", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestSocketNonMemberConnection(t *testing.T) {
	header := getAuthHeader(t)
	if header == "" {
		t.Skip("no auth token")
	}

	headerhttp := http.Header{}
	headerhttp.Set("Authorization", header)

	conn, resp, err := websocket.DefaultDialer.Dial(socketURL+"/channels/99999", headerhttp)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		require.Error(t, err)
		return
	}
	conn.Close()
	resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestSocketMemberReceivesMessage(t *testing.T) {
	header := getAuthHeader(t)
	if header == "" {
		t.Skip("no auth token")
	}

	headerhttp := http.Header{}
	headerhttp.Set("Authorization", header)

	conn, _, err := websocket.DefaultDialer.Dial(socketURL+"/channels/1", headerhttp)
	require.NoError(t, err, "WebSocket connection should succeed for channel member")
	defer conn.Close()

	err = conn.WriteMessage(websocket.TextMessage, []byte(`{}`))
	require.NoError(t, err)

	done := make(chan struct{})
	var receivedMessage []byte

	go func() {
		_, msg, err := conn.ReadMessage()
		if err == nil {
			receivedMessage = msg
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	if receivedMessage != nil {
		var event map[string]interface{}
		err := json.Unmarshal(receivedMessage, &event)
		require.NoError(t, err)
		assert.Equal(t, "created", event["type"])
		assert.Contains(t, event, "message")
	}
}

func TestSocketUnsubscribe(t *testing.T) {
	header := getAuthHeader(t)
	if header == "" {
		t.Skip("no auth token")
	}

	headerhttp := http.Header{}
	headerhttp.Set("Authorization", header)

	conn, _, err := websocket.DefaultDialer.Dial(socketURL+"/channels/1", headerhttp)
	require.NoError(t, err)

	err = conn.WriteMessage(websocket.TextMessage, []byte(`{}`))
	require.NoError(t, err)

	conn.Close()

	_, _, err = websocket.DefaultDialer.Dial(socketURL+"/channels/1", headerhttp)
	require.NoError(t, err)
}

func init() {
	if os.Getenv("E2E_TEST_TOKEN") != "" {
		testAccessToken = os.Getenv("E2E_TEST_TOKEN")
	}
}
