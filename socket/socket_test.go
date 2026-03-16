package socket

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

// fakeSubscription implements the subscription interface for testing.
type fakeSubscription struct {
	unsubscribed bool
}

func (f *fakeSubscription) Unsubscribe() error {
	f.unsubscribed = true
	return nil
}

// fakeSubscriber implements the subscriber interface for testing. It captures
// the subject and callback, allowing tests to inject messages.
type fakeSubscriber struct {
	mu       sync.Mutex
	subjects []string
	cbs      []func([]byte)
	err      error // if set, Subscribe returns this error
}

func (f *fakeSubscriber) subscribe(subject string, cb func([]byte)) (subscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	f.subjects = append(f.subjects, subject)
	f.cbs = append(f.cbs, cb)
	return &fakeSubscription{}, nil
}

// send publishes data to all captured callbacks (simulates a NATS message).
func (f *fakeSubscriber) send(data []byte) {
	f.mu.Lock()
	cbs := append([]func([]byte){}, f.cbs...)
	f.mu.Unlock()
	for _, cb := range cbs {
		cb(data)
	}
}

// fakeAuth returns middleware that injects the given user into the request context.
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

// fakeAuthReject returns middleware that rejects all requests.
func fakeAuthReject() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		})
	}
}

// dialWS dials a WebSocket connection to the given test server path.
func dialWS(t *testing.T, server *httptest.Server, path string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + path
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	return conn
}

// --- Health endpoint tests ---

func TestHealth(t *testing.T) {
	fake := &fakeSubscriber{}
	user := &auth.User{Email: "test@test.com", IsAdmin: false}
	handler := newHandler(fake, fakeAuth(user))

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
	fake := &fakeSubscriber{}
	handler := newHandler(fake, fakeAuthReject())

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// --- WebSocket connection tests ---

func TestWebSocket_ConnectToChannel(t *testing.T) {
	fake := &fakeSubscriber{}
	user := &auth.User{Email: "user@test.com", IsAdmin: false}
	handler := newHandler(fake, fakeAuth(user))
	server := httptest.NewServer(handler)
	defer server.Close()

	conn := dialWS(t, server, "/channels/42")
	defer conn.Close()

	// Verify NATS subscription was created for the correct subject.
	fake.mu.Lock()
	defer fake.mu.Unlock()
	require.Len(t, fake.subjects, 1)
	assert.Equal(t, "channel.42.messages", fake.subjects[0])
}

func TestWebSocket_RelayNATSMessage(t *testing.T) {
	fake := &fakeSubscriber{}
	user := &auth.User{Email: "user@test.com", IsAdmin: false}
	handler := newHandler(fake, fakeAuth(user))
	server := httptest.NewServer(handler)
	defer server.Close()

	conn := dialWS(t, server, "/channels/7")
	defer conn.Close()

	// Simulate a NATS message being published.
	msg := `{"Content":"hello world","ChannelID":7}`
	fake.send([]byte(msg))

	// Read the message from the WebSocket.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, msg, string(data))
}

func TestWebSocket_MultipleMessages(t *testing.T) {
	fake := &fakeSubscriber{}
	user := &auth.User{Email: "user@test.com", IsAdmin: false}
	handler := newHandler(fake, fakeAuth(user))
	server := httptest.NewServer(handler)
	defer server.Close()

	conn := dialWS(t, server, "/channels/1")
	defer conn.Close()

	messages := []string{
		`{"Content":"first"}`,
		`{"Content":"second"}`,
		`{"Content":"third"}`,
	}

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
	user := &auth.User{Email: "user@test.com", IsAdmin: false}
	handler := newHandler(fake, fakeAuth(user))
	server := httptest.NewServer(handler)
	defer server.Close()

	conn1 := dialWS(t, server, "/channels/10")
	defer conn1.Close()
	conn2 := dialWS(t, server, "/channels/20")
	defer conn2.Close()

	// Verify both subscriptions were created with different subjects.
	fake.mu.Lock()
	subjects := append([]string{}, fake.subjects...)
	fake.mu.Unlock()
	require.Len(t, subjects, 2)
	assert.Contains(t, subjects, "channel.10.messages")
	assert.Contains(t, subjects, "channel.20.messages")
}

func TestWebSocket_InvalidChannelID(t *testing.T) {
	fake := &fakeSubscriber{}
	user := &auth.User{Email: "user@test.com", IsAdmin: false}
	handler := newHandler(fake, fakeAuth(user))
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/channels/abc"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestWebSocket_ZeroChannelID(t *testing.T) {
	fake := &fakeSubscriber{}
	user := &auth.User{Email: "user@test.com", IsAdmin: false}
	handler := newHandler(fake, fakeAuth(user))
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/channels/0"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// --- Auth tests ---

func TestWebSocket_AuthRejected(t *testing.T) {
	fake := &fakeSubscriber{}
	handler := newHandler(fake, fakeAuthReject())
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/channels/1"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestWebSocket_NoUserInContext(t *testing.T) {
	fake := &fakeSubscriber{}
	// Auth middleware that passes through without setting a user.
	passthrough := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
	handler := newHandler(fake, passthrough)
	server := httptest.NewServer(handler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/channels/1"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// --- Test mode tests ---

func TestWebSocket_TestUserCanConnect(t *testing.T) {
	fake := &fakeSubscriber{}
	testUser := &auth.User{Email: "test@test.com", IsTest: true}
	handler := newHandler(fake, fakeAuth(testUser))
	server := httptest.NewServer(handler)
	defer server.Close()

	conn := dialWS(t, server, "/channels/5")
	defer conn.Close()

	fake.mu.Lock()
	defer fake.mu.Unlock()
	require.Len(t, fake.subjects, 1)
	assert.Equal(t, "channel.5.messages", fake.subjects[0])
}

func TestWebSocket_AdminTestUserCanConnect(t *testing.T) {
	fake := &fakeSubscriber{}
	adminTest := &auth.User{Email: "admin-test@test.com", IsAdmin: true, IsTest: true}
	handler := newHandler(fake, fakeAuth(adminTest))
	server := httptest.NewServer(handler)
	defer server.Close()

	conn := dialWS(t, server, "/channels/99")
	defer conn.Close()

	fake.mu.Lock()
	defer fake.mu.Unlock()
	require.Len(t, fake.subjects, 1)
	assert.Equal(t, "channel.99.messages", fake.subjects[0])
}

func TestWebSocket_RealUserCanConnect(t *testing.T) {
	fake := &fakeSubscriber{}
	realUser := &auth.User{Email: "real@test.com", IsTest: false}
	handler := newHandler(fake, fakeAuth(realUser))
	server := httptest.NewServer(handler)
	defer server.Close()

	conn := dialWS(t, server, "/channels/3")
	defer conn.Close()

	fake.mu.Lock()
	defer fake.mu.Unlock()
	require.Len(t, fake.subjects, 1)
}

// --- NATS subscribe error ---

func TestWebSocket_NATSSubscribeError(t *testing.T) {
	fake := &fakeSubscriber{err: errors.New("nats unavailable")}
	user := &auth.User{Email: "user@test.com", IsAdmin: false}
	handler := newHandler(fake, fakeAuth(user))
	server := httptest.NewServer(handler)
	defer server.Close()

	// The server will upgrade the connection but then close it immediately
	// when NATS subscribe fails.
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/channels/1"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		// Connection may fail during upgrade if server is fast enough.
		return
	}
	defer conn.Close()

	// If we got a connection, it should be closed soon.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = conn.ReadMessage()
	assert.Error(t, err, "connection should be closed after subscribe failure")
}

// --- NATS subject format ---

func TestNATSSubject(t *testing.T) {
	assert.Equal(t, "channel.1.messages", NATSSubject(1))
	assert.Equal(t, "channel.42.messages", NATSSubject(42))
	assert.Equal(t, "channel.999.messages", NATSSubject(999))
}

// --- parseChannelID ---

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

// --- Gorilla WebSocket verification ---

func TestUsesGorillaWebSocket(t *testing.T) {
	// Verify the upgrader is a Gorilla WebSocket upgrader.
	assert.NotNil(t, upgrader)
	assert.True(t, upgrader.CheckOrigin(nil))
}

// --- Client disconnect cleanup ---

func TestWebSocket_ClientDisconnect(t *testing.T) {
	fake := &fakeSubscriber{}
	user := &auth.User{Email: "user@test.com", IsAdmin: false}
	handler := newHandler(fake, fakeAuth(user))
	server := httptest.NewServer(handler)
	defer server.Close()

	conn := dialWS(t, server, "/channels/1")

	// Close the connection from the client side.
	conn.Close()

	// Give the server goroutine time to clean up.
	time.Sleep(100 * time.Millisecond)

	// Verify subscribe was called (cleanup happens internally).
	fake.mu.Lock()
	defer fake.mu.Unlock()
	require.Len(t, fake.subjects, 1)
}

// --- JSON response format for health ---

func TestHealthResponseIsJSON(t *testing.T) {
	fake := &fakeSubscriber{}
	handler := newHandler(fake, fakeAuth(nil))

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}
