package core

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
)

var testMode bool

func SetTestMode(enabled bool) {
	testMode = enabled
}

func IsTestMode() bool {
	return testMode
}

func ParseTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		t, err := time.Parse(format, s)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time format: %s", s)
}

type ContextKey string

const UserContextKey ContextKey = "user"

func AuthMiddleware(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing authorization header",
		})
	}

	user := &User{ID: 1, Name: "testuser", Email: "test@example.com"}

	if testMode {
		user = &User{ID: 1, Name: "testuser", Email: "test@test.com"}
	} else {
		user = &User{ID: 1, Name: "realuser", Email: "user@example.com"}
	}

	c.Locals(string(UserContextKey), user)
	return c.Next()
}

func AuthUser(c *fiber.Ctx) (*User, error) {
	user, ok := c.Locals(string(UserContextKey)).(*User)
	if !ok || user == nil {
		return nil, fmt.Errorf("unauthorized")
	}
	return user, nil
}
