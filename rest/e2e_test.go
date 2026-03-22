package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestHealthCheck(t *testing.T) {
	deps := Deps{
		DB:   &gorm.DB{}, // Needs a non-nil DB or it will panic
		NATS: nil,        // Should be fine if not used
	}

	// I need a real auth middleware, but maybe I can pass a passthrough one?
	// The rest/main.go uses:
	// func(next http.Handler) http.Handler { return next }

	authMW := func(next http.Handler) http.Handler { return next }

	engine := New(deps, authMW)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	engine.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}
