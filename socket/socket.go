package main

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

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type subscriber interface {
	subscribe(subject string, cb func([]byte)) (unsubscribe func(), err error)
}

type natsSubscriber struct{ conn *corenats.Conn }

func (n *natsSubscriber) subscribe(subject string, cb func([]byte)) (func(), error) {
	sub, err := n.conn.Subscribe(subject, cb)
	if err != nil {
		return nil, err
	}
	return func() { sub.Unsubscribe() }, nil
}

// New returns an http.Handler for the socket service.
func New(conn *corenats.Conn, authMW func(http.Handler) http.Handler) http.Handler {
	return newHandler(&natsSubscriber{conn: conn}, authMW)
}

func newHandler(sub subscriber, authMW func(http.Handler) http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", serveHealth)
	mux.Handle("/channels/", authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveChannel(sub, w, r)
	})))
	return mux
}

func serveHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"ok"}`)
}

func serveChannel(sub subscriber, w http.ResponseWriter, r *http.Request) {
	if _, err := auth.UserFromContext(r.Context()); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := parseChannelID(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}

	var mu sync.Mutex
	unsubscribe, err := sub.subscribe(channelSubject(id), func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			slog.Error("websocket write failed", "error", err)
		}
	})
	if err != nil {
		slog.Error("nats subscribe failed", "error", err)
		conn.Close()
		return
	}

	go func() {
		defer unsubscribe()
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

func channelSubject(id uint) string {
	return fmt.Sprintf("wurbs.channel.%d.messages", id)
}

func parseChannelID(path string) (uint, error) {
	path = strings.TrimSuffix(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[1] != "channels" {
		return 0, fmt.Errorf("invalid channel path")
	}
	id, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid channel id")
	}
	return uint(id), nil
}
