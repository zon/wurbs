package migrate

import (
	"github.com/zon/chat/core"
	"gorm.io/gorm"
)

// RunMigrations applies all pending GORM AutoMigrate migrations.
func RunMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&core.Message{},
		&core.User{},
	)
}
