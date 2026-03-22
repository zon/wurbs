package auth

import (
	"context"
	"errors"

	"github.com/zon/chat/core/user"
)

// Re-export User type
type User = user.UserModel

var (
	ErrNoUser       = errors.New("auth: no authenticated user in context")
	ErrUnauthorized = errors.New("auth: unauthorized")
)

type contextKey int

const userContextKey contextKey = iota

func UserFromContext(ctx context.Context) (*User, error) {
	u, ok := ctx.Value(userContextKey).(*User)
	if !ok || u == nil {
		return nil, ErrNoUser
	}
	return u, nil
}

func ContextWithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}
