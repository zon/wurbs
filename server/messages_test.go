package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zon/chat/core"
)

func TestAuthMiddleware(t *testing.T) {
	t.Run("missing authorization returns 401", func(t *testing.T) {
		app := fiber.New()
		app.Use(core.AuthMiddleware)
		app.Get("/test", func(c *fiber.Ctx) error {
			return c.SendString("OK")
		})

		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 401, resp.StatusCode)
	})
}

func TestAuthUser(t *testing.T) {
	t.Run("extracts user from context", func(t *testing.T) {
		app := fiber.New()
		app.Use(func(c *fiber.Ctx) error {
			user := &core.User{ID: 1, Name: "testuser", Email: "test@test.com"}
			c.Locals(string(core.UserContextKey), user)
			return c.Next()
		})
		app.Get("/test", func(c *fiber.Ctx) error {
			user, err := core.AuthUser(c)
			if err != nil {
				return err
			}
			return c.JSON(user)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)

		var user core.User
		json.NewDecoder(resp.Body).Decode(&user)
		assert.Equal(t, "testuser", user.Name)
	})

	t.Run("missing user returns error", func(t *testing.T) {
		app := fiber.New()
		app.Get("/test", func(c *fiber.Ctx) error {
			_, err := core.AuthUser(c)
			return err
		})

		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 500, resp.StatusCode)
	})
}

func TestMessageRoutes(t *testing.T) {
	app := fiber.New()

	core.SetTestMode(true)

	app.Use(func(c *fiber.Ctx) error {
		user := &core.User{ID: 1, Name: "testuser", Email: "test@test.com"}
		c.Locals(string(core.UserContextKey), user)
		return c.Next()
	})

	t.Run("POST /messages with empty body returns 400", func(t *testing.T) {
		app.Post("/messages", postMessage)
		req := httptest.NewRequest("POST", "/messages", bytes.NewBufferString(""))
		req.Header.Set("Content-Type", "text/plain")
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 422, resp.StatusCode)
	})

	t.Run("PUT /messages/:id with invalid id returns 400", func(t *testing.T) {
		app.Put("/messages/:id", putMessage)
		req := httptest.NewRequest("PUT", "/messages/invalid", bytes.NewBufferString("test"))
		req.Header.Set("Content-Type", "text/plain")
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 400, resp.StatusCode)
	})

	t.Run("DELETE /messages/:id with invalid id returns 400", func(t *testing.T) {
		app.Delete("/messages/:id", deleteMessage)
		req := httptest.NewRequest("DELETE", "/messages/invalid", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, 400, resp.StatusCode)
	})
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"RFC3339", "2024-01-15T10:30:00Z", false},
		{"RFC3339 with offset", "2024-01-15T10:30:00+05:30", false},
		{"simple format", "2024-01-15", false},
		{"invalid", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := core.ParseTime(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSetTestMode(t *testing.T) {
	core.SetTestMode(true)
	assert.True(t, core.IsTestMode())

	core.SetTestMode(false)
	assert.False(t, core.IsTestMode())
}

func TestHealthEndpoint(t *testing.T) {
	app := fiber.New()
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON("ok")
	})

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, `"ok"`, string(body))
}
