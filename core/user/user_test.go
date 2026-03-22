package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetUserByID_Found(t *testing.T) {
	t.Skip("skipping test that requires database")
	var db *gorm.DB

	_, err := EnsureAdminUser(db, "nonexistent@example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUserNotFound)
}
