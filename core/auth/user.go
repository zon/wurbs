package auth

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// User is the application user model. The auth module owns this type.
type User struct {
	gorm.Model
	Email   string `gorm:"uniqueIndex"`
	Subject string `gorm:"uniqueIndex"`
	IsAdmin bool
	IsTest  bool
}

// Errors returned by the auth module.
var (
	ErrNoUser        = errors.New("auth: no authenticated user in context")
	ErrUnauthorized  = errors.New("auth: unauthorized")
	ErrUserNotFound  = errors.New("auth: user not found")
	ErrTestUserAdmin = errors.New("auth: test users cannot become real admins")
)

type contextKey int

const userContextKey contextKey = iota

// UserFromContext extracts the authenticated user from the context.
// Returns ErrNoUser if no user has been set by auth middleware.
func UserFromContext(ctx context.Context) (*User, error) {
	u, ok := ctx.Value(userContextKey).(*User)
	if !ok || u == nil {
		return nil, ErrNoUser
	}
	return u, nil
}

// ContextWithUser returns a new context with the authenticated user set.
// This is used by auth middleware internally and may be used in tests
// to inject a user into the request context.
func ContextWithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

// EnsureAdminUser promotes an existing user to admin. It requires the user
// to already exist in the database and rejects promotion of test users.
func EnsureAdminUser(db *gorm.DB, email string) (*User, error) {
	user := &User{}

	result := db.Where("email = ?", email).First(user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("auth: failed to find user: %w", result.Error)
	}

	if user.IsTest {
		return nil, ErrTestUserAdmin
	}

	if !user.IsAdmin {
		if err := db.Model(user).Update("is_admin", true).Error; err != nil {
			return nil, fmt.Errorf("auth: failed to update admin flag: %w", err)
		}
		user.IsAdmin = true
	}

	return user, nil
}

// EnsureTestAdminUser creates or updates a test admin user with the given email,
// ensuring IsAdmin and IsTest are both true.
func EnsureTestAdminUser(db *gorm.DB, email string) (*User, error) {
	user := &User{}

	result := db.Where("email = ?", email).First(user)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("auth: failed to find user: %w", result.Error)
	}

	if result.Error == gorm.ErrRecordNotFound {
		user = &User{Email: email, IsAdmin: true, IsTest: true}
		if err := db.Create(user).Error; err != nil {
			return nil, fmt.Errorf("auth: failed to create test admin user: %w", err)
		}
		fmt.Printf("created test admin user: %s\n", email)
	} else {
		updates := map[string]any{
			"is_admin": true,
			"is_test":  true,
		}
		if err := db.Model(user).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("auth: failed to update test admin user: %w", err)
		}
		user.IsAdmin = true
		user.IsTest = true
		fmt.Printf("test admin user already exists: %s (keys will be rotated)\n", email)
	}

	return user, nil
}
