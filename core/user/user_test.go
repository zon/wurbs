package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}))
	return db
}

func TestEnsureAdminUser_UserNotFound(t *testing.T) {
	db := setupTestDB(t)

	_, err := EnsureAdminUser(db, "nonexistent@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUserNotFound)
}
