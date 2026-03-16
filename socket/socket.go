package socket

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/zon/chat/core/auth"
	corenats "github.com/zon/chat/core/nats"
)

// subscriber abstracts NATS subscribe so tests can inject a fake.
type subscriber interface {
	subscribe(subject string, cb func([]byte)) (subscription, error)
}

// subscription represents an active subscription that can be unsubscribed.
type subscription interface {
	Unsubscribe() error
}

// natsSubscriber wraps a *corenats.Conn to satisfy the subscriber interface.
type natsSubscriber struct {
	conn *corenats.Conn
}

func (n *natsSubscriber) subscribe(subject string, cb func([]byte)) (subscription, error) {
	return n.conn.Subscribe(subject, cb)
}

// Deps holds the dependencies for the socket service.
type Deps struct {
	NATS *corenats.Conn
}

// upgrader is the Gorilla WebSocket upgrader. CheckOrigin allows all
// connections; production deployments should restrict this.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// New creates an http.Handler with the WebSocket and health routes.
// The auth middleware parameter wraps standard net/http middleware that
// sets the authenticated user in the request context.
func New(deps Deps, authMiddleware func(http.Handler) http.Handler) http.Handler {
	sub := &natsSubscriber{conn: deps.NATS}
	return newHandler(sub, authMiddleware)
}

// newHandler is the internal constructor used by both New and tests.
func newHandler(sub subscriber, authMiddleware func(http.Handler) http.Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", health)
	mux.Handle("/channels/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleChannel(sub, w, r)
	})))

	return mux
}

// health is the unauthenticated health check endpoint.
func health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// parseChannelID extracts the channel ID from a path like /channels/123.
func parseChannelID(path string) (uint, error) {
	path = strings.TrimSuffix(path, "/")
	parts := strings.Split(path, "/")
	// Expected: ["", "channels", "<id>"]
	if len(parts) != 3 || parts[1] != "channels" {
		return 0, fmt.Errorf("invalid channel path")
	}
	id, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid channel id")
	}
	return uint(id), nil
}

// NATSSubject returns the NATS subject for a channel's messages.
func NATSSubject(channelID uint) string {
	return fmt.Sprintf("channel.%d.messages", channelID)
}

// handleChannel upgrades the HTTP request to a WebSocket connection,
// subscribes to the channel's NATS subject, and relays messages.
func handleChannel(sub subscriber, w http.ResponseWriter, r *http.Request) {
	if _, err := auth.UserFromContext(r.Context()); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	channelID, err := parseChannelID(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}

	var writeMu sync.Mutex
	natsSub, err := sub.subscribe(NATSSubject(channelID), func(data []byte) {
		writeMu.Lock()
		defer writeMu.Unlock()
		if writeErr := conn.WriteMessage(websocket.TextMessage, data); writeErr != nil {
			slog.Error("websocket write failed", "error", writeErr)
		}
	})
	if err != nil {
		slog.Error("nats subscribe failed", "error", err)
		conn.Close()
		return
	}

	// Read loop: keep the connection alive until the client disconnects.
	// Incoming messages are discarded; the WebSocket is write-only from
	// the server's perspective.
	go func() {
		defer natsSub.Unsubscribe()
		defer conn.Close()
		for {
			if _, _, readErr := conn.ReadMessage(); readErr != nil {
				return
			}
		}
	}()
}
