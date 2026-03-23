package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/zon/chat/core/auth"
	"github.com/zon/chat/core/channel"
	corenats "github.com/zon/chat/core/nats"
	"github.com/zon/chat/core/user"
	"gorm.io/gorm"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type subscriber interface {
	Subscribe(subject string, cb func([]byte)) (unsubscribe func(), err error)
}

type natsSubscriber struct{ conn *corenats.Conn }

func (n *natsSubscriber) Subscribe(subject string, cb func([]byte)) (func(), error) {
	sub, err := n.conn.Subscribe(subject, cb)
	if err != nil {
		return nil, err
	}
	return func() { sub.Unsubscribe() }, nil
}

// New returns an http.Handler for the socket service.
func New(conn *corenats.Conn, authMW func(http.Handler) http.Handler, db *gorm.DB) http.Handler {
	return newHandler(&natsSubscriber{conn: conn}, authMW, db)
}

func newHandler(sub subscriber, authMW func(http.Handler) http.Handler, db *gorm.DB) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", serveHealth)
	mux.Handle("/channels/", authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveChannel(sub, db, w, r)
	})))
	return mux
}

func serveHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"ok"}`)
}

func serveChannel(sub subscriber, db *gorm.DB, w http.ResponseWriter, r *http.Request) {
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := parseChannelID(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !isChannelMember(db, id, user) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}

	var mu sync.Mutex

	msgsUnsub, err := sub.Subscribe(messageSubject(id), func(data []byte) {
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

	membersUnsub, err := sub.Subscribe(memberSubject(id), func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			slog.Error("websocket write failed", "error", err)
		}
	})
	if err != nil {
		slog.Error("nats subscribe failed", "error", err)
		msgsUnsub()
		conn.Close()
		return
	}

	usersUnsub, err := sub.Subscribe(userSubject(id), func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			slog.Error("websocket write failed", "error", err)
		}
	})
	if err != nil {
		slog.Error("nats subscribe failed", "error", err)
		msgsUnsub()
		membersUnsub()
		conn.Close()
		return
	}

	unsubscribe := func() {
		msgsUnsub()
		membersUnsub()
		usersUnsub()
	}

	done := make(chan struct{})
	go func() {
		defer unsubscribe()
		defer conn.Close()
		defer close(done)
		for {
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if msgType == websocket.TextMessage {
				var msg map[string]interface{}
				if err := json.Unmarshal(data, &msg); err != nil {
					continue
				}
				if len(msg) == 0 {
					return
				}
			}
		}
	}()
	<-done
}

func isChannelMember(db *gorm.DB, channelID uint, user *user.User) bool {
	if user.IsAdmin {
		return true
	}
	ch, err := channel.Get(db, channelID)
	if err != nil {
		return false
	}
	members, err := channel.Members(db, channelID)
	if err != nil {
		return false
	}
	for _, m := range members {
		if m.ID == user.ID && m.IsTest == ch.IsTest {
			return true
		}
	}
	return false
}

func messageSubject(id uint) string {
	return fmt.Sprintf("wurbs.channel.%d.messages", id)
}

func memberSubject(id uint) string {
	return fmt.Sprintf("wurbs.channel.%d.members", id)
}

func userSubject(id uint) string {
	return fmt.Sprintf("wurbs.channel.%d.users", id)
}

func parseChannelID(path string) (id uint, err error) {
	path = strings.TrimSuffix(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[1] != "channels" {
		return 0, fmt.Errorf("invalid channel path")
	}
	parsedID, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil || parsedID == 0 {
		return 0, fmt.Errorf("invalid channel id")
	}
	id = uint(parsedID)
	return
}
