package core

import "github.com/zon/chat/core/auth"

// User is a type alias for auth.User. The auth module owns the User model;
// this alias keeps existing references to core.User working.
type User = auth.User
