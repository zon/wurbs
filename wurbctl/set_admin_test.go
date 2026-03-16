package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AdminUser{}))
	return db
}

// --- EnsureAdminUser ---

func TestEnsureAdminUser_CreatesUser(t *testing.T) {
	db := openTestDB(t)

	user, err := EnsureAdminUser(db, "new@example.com")
	require.NoError(t, err)
	assert.Equal(t, "new@example.com", user.Email)
	assert.True(t, user.IsAdmin)

	var found AdminUser
	require.NoError(t, db.Where("email = ?", "new@example.com").First(&found).Error)
	assert.True(t, found.IsAdmin)
}

func TestEnsureAdminUser_UpdatesExistingNonAdmin(t *testing.T) {
	db := openTestDB(t)

	require.NoError(t, db.Create(&AdminUser{Email: "user@example.com", IsAdmin: false}).Error)

	user, err := EnsureAdminUser(db, "user@example.com")
	require.NoError(t, err)
	assert.True(t, user.IsAdmin)

	var found AdminUser
	require.NoError(t, db.Where("email = ?", "user@example.com").First(&found).Error)
	assert.True(t, found.IsAdmin)
}

func TestEnsureAdminUser_IdempotentForExistingAdmin(t *testing.T) {
	db := openTestDB(t)

	user1, err := EnsureAdminUser(db, "admin@example.com")
	require.NoError(t, err)

	user2, err := EnsureAdminUser(db, "admin@example.com")
	require.NoError(t, err)

	assert.Equal(t, user1.ID, user2.ID)
	assert.True(t, user2.IsAdmin)
}

// --- adminSecretName ---

func TestAdminSecretName_Basic(t *testing.T) {
	assert.Equal(t, "wurbs-admin-admin-example-com", adminSecretName("admin@example.com"))
}

func TestAdminSecretName_SubdomainEmail(t *testing.T) {
	assert.Equal(t, "wurbs-admin-alice-sub-domain-org", adminSecretName("alice@sub.domain.org"))
}

func TestAdminSecretName_Uppercase(t *testing.T) {
	assert.Equal(t, "wurbs-admin-admin-example-com", adminSecretName("Admin@Example.COM"))
}

func TestAdminSecretName_StripsInvalidChars(t *testing.T) {
	name := adminSecretName("test.user+tag@example.com")
	assert.True(t, len(name) > len("wurbs-admin-"))
	for _, ch := range name[len("wurbs-admin-"):] {
		assert.True(t, (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-')
	}
}

// --- adminDSN ---

func TestAdminDSN_Success(t *testing.T) {
	t.Setenv("PGHOST", "localhost")
	t.Setenv("PGPORT", "5432")
	t.Setenv("PGUSER", "wurbs")
	t.Setenv("PGPASSWORD", "secret")
	t.Setenv("PGDATABASE", "wurbs_db")

	dsn, err := adminDSN()
	require.NoError(t, err)
	assert.Contains(t, dsn, "host=localhost")
	assert.Contains(t, dsn, "port=5432")
	assert.Contains(t, dsn, "user=wurbs")
	assert.Contains(t, dsn, "password=secret")
	assert.Contains(t, dsn, "dbname=wurbs_db")
}

func TestAdminDSN_DefaultPort(t *testing.T) {
	t.Setenv("PGHOST", "localhost")
	t.Setenv("PGPORT", "")
	t.Setenv("PGUSER", "wurbs")
	t.Setenv("PGPASSWORD", "secret")
	t.Setenv("PGDATABASE", "wurbs_db")

	dsn, err := adminDSN()
	require.NoError(t, err)
	assert.Contains(t, dsn, "port=5432")
}

func TestAdminDSN_MissingRequired(t *testing.T) {
	tests := []struct {
		name    string
		missing string
		env     map[string]string
	}{
		{"host", "PGHOST", map[string]string{"PGHOST": "", "PGUSER": "u", "PGDATABASE": "d"}},
		{"user", "PGUSER", map[string]string{"PGHOST": "h", "PGUSER": "", "PGDATABASE": "d"}},
		{"database", "PGDATABASE", map[string]string{"PGHOST": "h", "PGUSER": "u", "PGDATABASE": ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			_, err := adminDSN()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.missing)
		})
	}
}
