package auth

import (
	"context"
	"errors"

	"github.com/zon/chat/core/user"
)

var (
	ErrNoUser       = errors.New("auth: no authenticated user in context")
	ErrUnauthorized = errors.New("auth: unauthorized")
)

type contextKey int

const (
	_                         = iota
	userContextKey contextKey = iota
)

func UserFromContext(ctx context.Context) (*user.User, error) {
	u, ok := ctx.Value(userContextKey).(*user.User)
	if !ok || u == nil {
		return nil, ErrNoUser
	}
	return u, nil
}

func ContextWithUser(ctx context.Context, u *user.User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}
