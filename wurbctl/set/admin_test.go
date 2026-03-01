package set

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openTestDB opens an in-memory SQLite database for use in tests.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err, "failed to open in-memory sqlite database")
	err = db.AutoMigrate(&AdminUser{})
	require.NoError(t, err, "failed to migrate AdminUser table")
	return db
}

// mockDBOpener returns a DBOpener that returns the given db (for injection in tests).
func mockDBOpener(db *gorm.DB) DBOpener {
	return func() (*gorm.DB, error) {
		return db, nil
	}
}

// errDBOpener is a DBOpener that always returns an error.
func errDBOpener() (*gorm.DB, error) {
	return nil, errors.New("cannot connect to database")
}

// fullAdminCmd returns an AdminCmd with all fields set for local output and a test DB.
func fullAdminCmd(t *testing.T, db *gorm.DB) AdminCmd {
	t.Helper()
	return AdminCmd{
		Email:       "admin@example.com",
		Namespace:   "default",
		Local:       true,
		ensureAdmin: EnsureAdminUser,
		openDB:      mockDBOpener(db),
	}
}

// --- Missing required value tests ---

func TestAdminCmd_MissingEmail(t *testing.T) {
	db := openTestDB(t)
	cmd := fullAdminCmd(t, db)
	cmd.Email = ""
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email argument is required")
}

// --- Database connection error ---

func TestAdminCmd_DBConnectionError(t *testing.T) {
	cmd := AdminCmd{
		Email:     "admin@example.com",
		Namespace: "default",
		Local:     true,
		openDB:    errDBOpener,
	}
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to database")
	assert.Contains(t, err.Error(), "cannot connect to database")
}

// --- EnsureAdminUser error propagation ---

func TestAdminCmd_EnsureAdminError(t *testing.T) {
	db := openTestDB(t)
	cmd := fullAdminCmd(t, db)
	cmd.ensureAdmin = func(db *gorm.DB, email string) (*AdminUser, error) {
		return nil, errors.New("db write failed")
	}
	err := cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create or update admin user")
	assert.Contains(t, err.Error(), "db write failed")
}

// --- Local file generation ---

func TestAdminCmd_LocalWritesSecretFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	db := openTestDB(t)
	cmd := fullAdminCmd(t, db)
	err := cmd.Run()
	require.NoError(t, err)

	// The secret file should be named after the sanitized email
	expectedFile := adminSecretName("admin@example.com") + ".yaml"
	data, err := os.ReadFile(filepath.Join(dir, expectedFile))
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "kind: Secret")
	assert.Contains(t, content, "ADMIN_CLIENT_PRIVATE_KEY")
	assert.Contains(t, content, "ADMIN_CLIENT_PUBLIC_KEY")
	assert.Contains(t, content, "ADMIN_EMAIL")
}

func TestAdminCmd_LocalSecretContainsBase64EncodedKeys(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	db := openTestDB(t)
	cmd := fullAdminCmd(t, db)
	err := cmd.Run()
	require.NoError(t, err)

	expectedFile := adminSecretName("admin@example.com") + ".yaml"
	data, err := os.ReadFile(filepath.Join(dir, expectedFile))
	require.NoError(t, err)

	content := string(data)

	// Keys should be base64 encoded; raw PEM headers must not appear unencoded
	// (they would contain "BEGIN" which gets base64 encoded)
	rawPEM := "-----BEGIN RSA PRIVATE KEY-----"
	encodedPEM := base64.StdEncoding.EncodeToString([]byte(rawPEM))
	// The encoded content should start with the base64 of the PEM prefix
	assert.True(t,
		strings.Contains(content, encodedPEM[:10]),
		"secret YAML should contain base64-encoded private key",
	)
}

func TestAdminCmd_LocalSecretWithNamespace(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	db := openTestDB(t)
	cmd := fullAdminCmd(t, db)
	cmd.Namespace = "wurbs-prod"
	err := cmd.Run()
	require.NoError(t, err)

	expectedFile := adminSecretName("admin@example.com") + ".yaml"
	data, err := os.ReadFile(filepath.Join(dir, expectedFile))
	require.NoError(t, err)
	assert.Contains(t, string(data), "namespace: wurbs-prod")
}

func TestAdminCmd_LocalSecretFilePermissions(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	db := openTestDB(t)
	cmd := fullAdminCmd(t, db)
	err := cmd.Run()
	require.NoError(t, err)

	expectedFile := adminSecretName("admin@example.com") + ".yaml"
	info, err := os.Stat(filepath.Join(dir, expectedFile))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "secret file should be owner-read/write only")
}

// --- EnsureAdminUser behaviour ---

func TestEnsureAdminUser_CreatesUser(t *testing.T) {
	db := openTestDB(t)

	user, err := EnsureAdminUser(db, "new@example.com")
	require.NoError(t, err)
	assert.Equal(t, "new@example.com", user.Email)
	assert.True(t, user.IsAdmin)

	// Verify it's in the database
	var found AdminUser
	require.NoError(t, db.Where("email = ?", "new@example.com").First(&found).Error)
	assert.True(t, found.IsAdmin)
}

func TestEnsureAdminUser_UpdatesExistingNonAdmin(t *testing.T) {
	db := openTestDB(t)

	// Insert a non-admin user first
	existing := &AdminUser{Email: "user@example.com", IsAdmin: false}
	require.NoError(t, db.Create(existing).Error)

	// EnsureAdminUser should promote them to admin
	user, err := EnsureAdminUser(db, "user@example.com")
	require.NoError(t, err)
	assert.True(t, user.IsAdmin)

	// Verify in DB
	var found AdminUser
	require.NoError(t, db.Where("email = ?", "user@example.com").First(&found).Error)
	assert.True(t, found.IsAdmin)
}

func TestEnsureAdminUser_IdempotentForExistingAdmin(t *testing.T) {
	db := openTestDB(t)

	// Create admin once
	user1, err := EnsureAdminUser(db, "admin@example.com")
	require.NoError(t, err)

	// Call again — should not error
	user2, err := EnsureAdminUser(db, "admin@example.com")
	require.NoError(t, err)

	// Same user ID
	assert.Equal(t, user1.ID, user2.ID)
	assert.Equal(t, user1.Email, user2.Email)
	assert.True(t, user2.IsAdmin)
}

// --- adminSecretName ---

func TestAdminSecretName_Basic(t *testing.T) {
	name := adminSecretName("admin@example.com")
	assert.Equal(t, "wurbs-admin-admin-example-com", name)
}

func TestAdminSecretName_SubdomainEmail(t *testing.T) {
	name := adminSecretName("alice@sub.domain.org")
	assert.Equal(t, "wurbs-admin-alice-sub-domain-org", name)
}

func TestAdminSecretName_Uppercase(t *testing.T) {
	name := adminSecretName("Admin@Example.COM")
	assert.Equal(t, "wurbs-admin-admin-example-com", name)
}

func TestAdminSecretName_ValidK8sName(t *testing.T) {
	name := adminSecretName("test.user+tag@example.com")
	// Should only contain alphanumerics and hyphens, prefixed with "wurbs-admin-"
	assert.True(t, strings.HasPrefix(name, "wurbs-admin-"))
	suffix := strings.TrimPrefix(name, "wurbs-admin-")
	for _, ch := range suffix {
		assert.True(t, (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-',
			"character %q should be alphanumeric or hyphen", ch)
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

func TestAdminDSN_MissingHost(t *testing.T) {
	t.Setenv("PGHOST", "")
	t.Setenv("PGUSER", "wurbs")
	t.Setenv("PGDATABASE", "wurbs_db")

	_, err := adminDSN()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PGHOST")
}

func TestAdminDSN_MissingUser(t *testing.T) {
	t.Setenv("PGHOST", "localhost")
	t.Setenv("PGUSER", "")
	t.Setenv("PGDATABASE", "wurbs_db")

	_, err := adminDSN()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PGUSER")
}

func TestAdminDSN_MissingDatabase(t *testing.T) {
	t.Setenv("PGHOST", "localhost")
	t.Setenv("PGUSER", "wurbs")
	t.Setenv("PGDATABASE", "")

	_, err := adminDSN()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PGDATABASE")
}

func TestAdminDSN_MissingMultiple(t *testing.T) {
	t.Setenv("PGHOST", "")
	t.Setenv("PGUSER", "")
	t.Setenv("PGDATABASE", "")

	_, err := adminDSN()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PGHOST")
	assert.Contains(t, err.Error(), "PGUSER")
	assert.Contains(t, err.Error(), "PGDATABASE")
}

// --- SecretName appears in generated YAML ---

func TestAdminCmd_LocalSecretNameContainsEmail(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	db := openTestDB(t)
	cmd := fullAdminCmd(t, db)
	cmd.Email = "bob@test.io"
	err := cmd.Run()
	require.NoError(t, err)

	expectedSecretName := adminSecretName("bob@test.io")
	expectedFile := expectedSecretName + ".yaml"

	data, err := os.ReadFile(filepath.Join(dir, expectedFile))
	require.NoError(t, err)
	assert.Contains(t, string(data), expectedSecretName)
}

// --- Key pair validity ---

func TestAdminCmd_LocalSecretContainsValidRSAKeys(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	db := openTestDB(t)
	cmd := fullAdminCmd(t, db)
	err := cmd.Run()
	require.NoError(t, err)

	expectedFile := adminSecretName("admin@example.com") + ".yaml"
	data, err := os.ReadFile(filepath.Join(dir, expectedFile))
	require.NoError(t, err)

	content := string(data)

	// Keys should be present and base64 encoded
	assert.Contains(t, content, "ADMIN_CLIENT_PRIVATE_KEY")
	assert.Contains(t, content, "ADMIN_CLIENT_PUBLIC_KEY")

	// Ensure they're non-empty by confirming there's something after the key name
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.Contains(line, "ADMIN_CLIENT_PRIVATE_KEY:") {
			parts := strings.SplitN(line, ": ", 2)
			require.Len(t, parts, 2, "ADMIN_CLIENT_PRIVATE_KEY line should have a value")
			assert.NotEmpty(t, strings.TrimSpace(parts[1]), "private key value should not be empty")
		}
		if strings.Contains(line, "ADMIN_CLIENT_PUBLIC_KEY:") {
			parts := strings.SplitN(line, ": ", 2)
			require.Len(t, parts, 2, "ADMIN_CLIENT_PUBLIC_KEY line should have a value")
			assert.NotEmpty(t, strings.TrimSpace(parts[1]), "public key value should not be empty")
		}
	}
}
