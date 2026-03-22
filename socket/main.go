package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/alecthomas/kong"
	"github.com/zon/chat/core/auth"
	"github.com/zon/chat/core/config"
	corenats "github.com/zon/chat/core/nats"
	"github.com/zon/chat/core/pg"
)

var cli struct {
	Port      string `help:"Port to listen on" default:"8081"`
	Test      bool   `help:"Enable test mode (test users and test channels)"`
	NatsToken string `help:"Path to NATS auth token file" type:"path"`
}

func Main() {
	kong.Parse(&cli)

	if cli.Test {
		config.SetTestMode(true)
	}

	if cli.NatsToken != "" {
		os.Setenv("WURB_NATS_TOKEN_FILE", cli.NatsToken)
	}

	db, err := pg.Open()
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}

	var nc *corenats.Conn
	nc, err = corenats.Connect()
	if err != nil {
		slog.Warn("NATS connection failed, running without NATS", "error", err)
	}

	authMW, err := auth.ClientMiddleware(db)
	if err != nil {
		slog.Warn("client auth middleware unavailable, using passthrough", "error", err)
		authMW = func(next http.Handler) http.Handler { return next }
	}

	handler := New(nc, authMW, db)
	if err := http.ListenAndServe(":"+cli.Port, handler); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func main() {
	Main()
}
