package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zon/chat/core/auth"
	"github.com/zon/chat/core/message"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// --- RunMigrations tests ---

func TestRunMigrations(t *testing.T) {
	t.Skip("skipping sqlite test")
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err, "failed to open in-memory sqlite database")

	err = RunMigrations(db)
	require.NoError(t, err, "RunMigrations should not return an error")

	assert.True(t, db.Migrator().HasTable(&auth.User{}), "users table should exist after migration")
	assert.True(t, db.Migrator().HasTable(&message.Message{}), "messages table should exist after migration")
}

func TestRunMigrations_Idempotent(t *testing.T) {
	t.Skip("skipping sqlite test")
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = RunMigrations(db)
	require.NoError(t, err, "first RunMigrations should succeed")

	err = RunMigrations(db)
	require.NoError(t, err, "second RunMigrations should also succeed (idempotent)")
}

func TestRunMigrations_CreatesColumns(t *testing.T) {
	t.Skip("skipping sqlite test")
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = RunMigrations(db)
	require.NoError(t, err)

	migrator := db.Migrator()
	assert.True(t, migrator.HasColumn(&message.Message{}, "id"), "messages table should have id column")
	assert.True(t, migrator.HasColumn(&message.Message{}, "created_at"), "messages table should have created_at column")
	assert.True(t, migrator.HasColumn(&message.Message{}, "updated_at"), "messages table should have updated_at column")
	assert.True(t, migrator.HasColumn(&message.Message{}, "deleted_at"), "messages table should have deleted_at column")
	assert.True(t, migrator.HasColumn(&message.Message{}, "user_id"), "messages table should have user_id column")
	assert.True(t, migrator.HasColumn(&message.Message{}, "content"), "messages table should have content column")

	assert.True(t, migrator.HasColumn(&auth.User{}, "id"), "users table should have id column")
	assert.True(t, migrator.HasColumn(&auth.User{}, "created_at"), "users table should have created_at column")
	assert.True(t, migrator.HasColumn(&auth.User{}, "updated_at"), "users table should have updated_at column")
	assert.True(t, migrator.HasColumn(&auth.User{}, "deleted_at"), "users table should have deleted_at column")
}
