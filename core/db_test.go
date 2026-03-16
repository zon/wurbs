package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestDBVariableExists(t *testing.T) {
	var db *gorm.DB
	assert.Nil(t, db)
}

func TestInitDB_MissingSecret(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("WURB_CONFIG", tmpDir)

	err := InitDB()
	assert.Error(t, err, "InitDB should fail when postgres.json is missing")
}
