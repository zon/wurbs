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
	"github.com/zon/chat/rest"
)

var cli struct {
	Port string `help:"Port to listen on" default:"8080"`
	Test bool   `help:"Enable test mode (test users and test channels)"`
}

func main() {
	kong.Parse(&cli)

	if cli.Test {
		config.SetTestMode(true)
	}

	db, err := pg.Open()
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}

	// NATS is optional; the service runs without it.
	var nc *corenats.Conn
	nc, err = corenats.Connect()
	if err != nil {
		slog.Warn("NATS connection failed, running without NATS", "error", err)
	}

	// Use client credential middleware for auth. In production, OIDC middleware
	// would also be wired in; for the REST service the client middleware covers
	// both admin and test user flows.
	authMW, err := auth.ClientMiddleware(db)
	if err != nil {
		slog.Warn("client auth middleware unavailable, using passthrough", "error", err)
		authMW = func(next http.Handler) http.Handler { return next }
	}

	deps := rest.Deps{
		DB:   db,
		NATS: nc,
	}

	engine := rest.New(deps, authMW)
	if err := engine.Run(":" + cli.Port); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
