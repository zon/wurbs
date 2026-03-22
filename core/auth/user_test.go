package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zon/chat/core/user"
)

// --- UserFromContext tests ---

func TestUserFromContext_WithUser(t *testing.T) {
	u := &user.User{Email: "test@example.com"}
	ctx := ContextWithUser(context.Background(), u)

	got, err := UserFromContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, "test@example.com", got.Email)
}

func TestUserFromContext_WithoutUser(t *testing.T) {
	_, err := UserFromContext(context.Background())
	assert.ErrorIs(t, err, ErrNoUser)
}

func TestUserFromContext_NilUser(t *testing.T) {
	ctx := context.WithValue(context.Background(), userContextKey, (*user.User)(nil))
	_, err := UserFromContext(ctx)
	assert.ErrorIs(t, err, ErrNoUser)
}

// --- EnsureAdminUser ---

func TestEnsureAdminUser_UserNotFound(t *testing.T) {
	db := setupTestDB(t)

	_, err := user.EnsureAdminUser(db, "nonexistent@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, user.ErrUserNotFound)
}

func TestEnsureAdminUser_UpdatesExistingNonAdmin(t *testing.T) {
	db := setupTestDB(t)

	require.NoError(t, db.Create(&user.User{Email: "user@example.com", IsAdmin: false}).Error)

	u, err := user.EnsureAdminUser(db, "user@example.com")
	require.NoError(t, err)
	assert.True(t, u.IsAdmin)

	var found user.User
	require.NoError(t, db.Where("email = ?", "user@example.com").First(&found).Error)
	assert.True(t, found.IsAdmin)
}

func TestEnsureAdminUser_IdempotentForExistingAdmin(t *testing.T) {
	db := setupTestDB(t)

	require.NoError(t, db.Create(&user.User{Email: "admin@example.com", IsAdmin: true}).Error)

	u, err := user.EnsureAdminUser(db, "admin@example.com")
	require.NoError(t, err)

	assert.True(t, u.IsAdmin)
}

func TestEnsureAdminUser_RejectsTestUser(t *testing.T) {
	db := setupTestDB(t)

	require.NoError(t, db.Create(&user.User{Email: "test@example.com", IsTest: true, IsAdmin: false}).Error)

	_, err := user.EnsureAdminUser(db, "test@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, user.ErrTestUserAdmin)

	var found user.User
	require.NoError(t, db.Where("email = ?", "test@example.com").First(&found).Error)
	assert.False(t, found.IsAdmin)
	assert.True(t, found.IsTest)
}

// --- EnsureTestAdminUser ---

func TestEnsureTestAdminUser_CreatesUser(t *testing.T) {
	db := setupTestDB(t)

	u, err := user.EnsureTestAdminUser(db, "test-admin@example.com")
	require.NoError(t, err)
	assert.Equal(t, "test-admin@example.com", u.Email)
	assert.True(t, u.IsAdmin)
	assert.True(t, u.IsTest)

	var found user.User
	require.NoError(t, db.Where("email = ?", "test-admin@example.com").First(&found).Error)
	assert.True(t, found.IsAdmin)
	assert.True(t, found.IsTest)
}

func TestEnsureTestAdminUser_UpdatesExistingUser(t *testing.T) {
	db := setupTestDB(t)

	require.NoError(t, db.Create(&user.User{Email: "test-admin@example.com", IsAdmin: false, IsTest: false}).Error)

	u, err := user.EnsureTestAdminUser(db, "test-admin@example.com")
	require.NoError(t, err)
	assert.True(t, u.IsAdmin)
	assert.True(t, u.IsTest)

	var found user.User
	require.NoError(t, db.Where("email = ?", "test-admin@example.com").First(&found).Error)
	assert.True(t, found.IsAdmin)
	assert.True(t, found.IsTest)
}

func TestEnsureTestAdminUser_IdempotentForExistingTestAdmin(t *testing.T) {
	db := setupTestDB(t)

	u1, err := user.EnsureTestAdminUser(db, "test-admin@example.com")
	require.NoError(t, err)

	u2, err := user.EnsureTestAdminUser(db, "test-admin@example.com")
	require.NoError(t, err)

	assert.Equal(t, u1.ID, u2.ID)
	assert.True(t, u2.IsAdmin)
	assert.True(t, u2.IsTest)
}

// --- User model tests ---

func TestUserModel_Fields(t *testing.T) {
	db := setupTestDB(t)

	u := &user.User{
		Email:   "full@example.com",
		Subject: "sub-full",
		IsAdmin: true,
		IsTest:  false,
	}
	require.NoError(t, db.Create(u).Error)

	var loaded user.User
	require.NoError(t, db.First(&loaded, u.ID).Error)

	assert.Equal(t, "full@example.com", loaded.Email)
	assert.Equal(t, "sub-full", loaded.Subject)
	assert.True(t, loaded.IsAdmin)
	assert.False(t, loaded.IsTest)
}

func TestUserModel_UniqueEmail(t *testing.T) {
	db := setupTestDB(t)

	u1 := &user.User{Email: "dup@example.com", Subject: "sub-1"}
	require.NoError(t, db.Create(u1).Error)

	u2 := &user.User{Email: "dup@example.com", Subject: "sub-2"}
	err := db.Create(u2).Error
	assert.Error(t, err)
}

func TestUserModel_UniqueSubject(t *testing.T) {
	db := setupTestDB(t)

	u1 := &user.User{Email: "a@example.com", Subject: "same-sub"}
	require.NoError(t, db.Create(u1).Error)

	u2 := &user.User{Email: "b@example.com", Subject: "same-sub"}
	err := db.Create(u2).Error
	assert.Error(t, err)
}
