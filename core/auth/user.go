package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Email     string  `gorm:"uniqueIndex"`
	Subject   string  `gorm:"uniqueIndex"`
	Username  *string `json:"username"`
	IsAdmin   bool
	IsActive  bool `gorm:"default:true"`
	IsTest    bool
}

var (
	ErrNoUser                  = errors.New("auth: no authenticated user in context")
	ErrUnauthorized            = errors.New("auth: unauthorized")
	ErrUserNotFound            = errors.New("auth: user not found")
	ErrTestUserAdmin           = errors.New("auth: test users cannot become real admins")
	ErrRealAdminModifyTestUser = errors.New("auth: real admins cannot modify test users")
	ErrTestAdminModifyRealUser = errors.New("auth: test admins cannot modify real users")
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

func GetUserByID(db *gorm.DB, id string) (*User, error) {
	var user User
	if err := db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("auth: failed to get user: %w", err)
	}
	return &user, nil
}

func UpdateUser(db *gorm.DB, user *User, input UpdateUserInput, isAdmin bool) error {
	updates := make(map[string]any)

	if input.Username != nil {
		updates["username"] = *input.Username
	}

	if input.Email != nil && *input.Email != "" {
		updates["email"] = *input.Email
	}

	if isAdmin {
		if input.Admin != nil {
			updates["is_admin"] = *input.Admin
		}
		if input.Inactive != nil {
			updates["is_active"] = !*input.Inactive
		}
	}

	if len(updates) == 0 {
		return nil
	}

	if err := db.Model(user).Updates(updates).Error; err != nil {
		return fmt.Errorf("auth: failed to update user: %w", err)
	}

	return nil
}

func UpdateUserAsAdmin(db *gorm.DB, admin, target *User, input UpdateUserInput) error {
	if admin.IsTest && !target.IsTest {
		return ErrTestAdminModifyRealUser
	}
	if !admin.IsTest && target.IsTest {
		return ErrRealAdminModifyTestUser
	}
	return UpdateUser(db, target, input, true)
}

type UpdateUserInput struct {
	Username *string `json:"username"`
	Email    *string `json:"email"`
	Admin    *bool   `json:"admin"`
	Inactive *bool   `json:"inactive"`
}
