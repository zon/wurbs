package migrate

import (
	"github.com/zon/chat/core/auth"
	"github.com/zon/chat/core/message"

	"gorm.io/gorm"
)

// RunMigrations applies all pending GORM AutoMigrate migrations.
func RunMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&auth.User{},
		&message.Message{},
	)
}
