package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zon/chat/core/auth"
)

type fakeSubscriber struct {
	mu       sync.Mutex
	subjects []string
	cbs      []func([]byte)
	err      error
}

func (f *fakeSubscriber) subscribe(subject string, cb func([]byte)) (func(), error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	f.subjects = append(f.subjects, subject)
	f.cbs = append(f.cbs, cb)
	return func() {}, nil
}

func (f *fakeSubscriber) send(data []byte) {
	f.mu.Lock()
	cbs := append([]func([]byte){}, f.cbs...)
	f.mu.Unlock()
	for _, cb := range cbs {
		cb(data)
	}
}

func fakeAuth(user *auth.User) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if user != nil {
				r = r.WithContext(auth.ContextWithUser(r.Context(), user))
			}
			next.ServeHTTP(w, r)
		})
	}
}

func fakeAuthReject() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		})
	}
}

func dialWS(t *testing.T, server *httptest.Server, path string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + path
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	return conn
}

func TestHealth(t *testing.T) {
	handler := newHandler(&fakeSubscriber{}, fakeAuth(&auth.User{}))
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
}

func TestHealth_NoAuthRequired(t *testing.T) {
	handler := newHandler(&fakeSubscriber{}, fakeAuthReject())
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWebSocket_ConnectToChannel(t *testing.T) {
	fake := &fakeSubscriber{}
	server := httptest.NewServer(newHandler(fake, fakeAuth(&auth.User{Email: "user@test.com"})))
	defer server.Close()

	conn := dialWS(t, server, "/channels/42")
	defer conn.Close()

	fake.mu.Lock()
	defer fake.mu.Unlock()
	require.Len(t, fake.subjects, 1)
	assert.Equal(t, "channel.42.messages", fake.subjects[0])
}

func TestWebSocket_RelayNATSMessage(t *testing.T) {
	fake := &fakeSubscriber{}
	server := httptest.NewServer(newHandler(fake, fakeAuth(&auth.User{Email: "user@test.com"})))
	defer server.Close()

	conn := dialWS(t, server, "/channels/7")
	defer conn.Close()

	msg := `{"Content":"hello world","ChannelID":7}`
	fake.send([]byte(msg))

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, msg, string(data))
}

func TestWebSocket_MultipleMessages(t *testing.T) {
	fake := &fakeSubscriber{}
	server := httptest.NewServer(newHandler(fake, fakeAuth(&auth.User{Email: "user@test.com"})))
	defer server.Close()

	conn := dialWS(t, server, "/channels/1")
	defer conn.Close()

	messages := []string{`{"Content":"first"}`, `{"Content":"second"}`, `{"Content":"third"}`}
	for _, msg := range messages {
		fake.send([]byte(msg))
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for _, expected := range messages {
		_, data, err := conn.ReadMessage()
		require.NoError(t, err)
		assert.Equal(t, expected, string(data))
	}
}

func TestWebSocket_ChannelIsolation(t *testing.T) {
	fake := &fakeSubscriber{}
	server := httptest.NewServer(newHandler(fake, fakeAuth(&auth.User{Email: "user@test.com"})))
	defer server.Close()

	conn1 := dialWS(t, server, "/channels/10")
	defer conn1.Close()
	conn2 := dialWS(t, server, "/channels/20")
	defer conn2.Close()

	fake.mu.Lock()
	subjects := append([]string{}, fake.subjects...)
	fake.mu.Unlock()
	require.Len(t, subjects, 2)
	assert.Contains(t, subjects, "channel.10.messages")
	assert.Contains(t, subjects, "channel.20.messages")
}

func TestWebSocket_InvalidChannelID(t *testing.T) {
	server := httptest.NewServer(newHandler(&fakeSubscriber{}, fakeAuth(&auth.User{})))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/channels/abc"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestWebSocket_ZeroChannelID(t *testing.T) {
	server := httptest.NewServer(newHandler(&fakeSubscriber{}, fakeAuth(&auth.User{})))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/channels/0"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestWebSocket_AuthRejected(t *testing.T) {
	server := httptest.NewServer(newHandler(&fakeSubscriber{}, fakeAuthReject()))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/channels/1"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestWebSocket_NoUserInContext(t *testing.T) {
	passthrough := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
	server := httptest.NewServer(newHandler(&fakeSubscriber{}, passthrough))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/channels/1"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestWebSocket_NATSSubscribeError(t *testing.T) {
	fake := &fakeSubscriber{err: errors.New("nats unavailable")}
	server := httptest.NewServer(newHandler(fake, fakeAuth(&auth.User{Email: "user@test.com"})))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/channels/1"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = conn.ReadMessage()
	assert.Error(t, err)
}

func TestWebSocket_ClientDisconnect(t *testing.T) {
	fake := &fakeSubscriber{}
	server := httptest.NewServer(newHandler(fake, fakeAuth(&auth.User{Email: "user@test.com"})))
	defer server.Close()

	conn := dialWS(t, server, "/channels/1")
	conn.Close()

	time.Sleep(100 * time.Millisecond)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	require.Len(t, fake.subjects, 1)
}

func TestParseChannelID(t *testing.T) {
	tests := []struct {
		path    string
		wantID  uint
		wantErr bool
	}{
		{"/channels/1", 1, false},
		{"/channels/42", 42, false},
		{"/channels/999", 999, false},
		{"/channels/0", 0, true},
		{"/channels/abc", 0, true},
		{"/channels/", 0, true},
		{"/channels", 0, true},
		{"/other/1", 0, true},
		{"/channels/1/extra", 0, true},
		{"", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			id, err := parseChannelID(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantID, id)
			}
		})
	}
}
