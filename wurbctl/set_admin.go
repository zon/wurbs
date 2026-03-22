package main

import (
	"fmt"

	"github.com/zon/chat/core/user"
)

// SetAdminCmd implements `wurbctl set admin`.
type SetAdminCmd struct {
	Email string `arg:"" name:"email" help:"Email address of the user to promote to admin."`
}

// Run executes the set admin command.
func (c *SetAdminCmd) Run(ctx *Context) error {
	db, err := ctx.DB()
	if err != nil {
		return err
	}

	u, err := user.EnsureAdminUser(db, c.Email)
	if err != nil {
		return fmt.Errorf("failed to promote user to admin: %w", err)
	}

	fmt.Printf("user %s promoted to admin\n", u.Email)
	return nil
}
