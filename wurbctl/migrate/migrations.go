package migrate

import "gorm.io/gorm"

// RunMigrations applies all pending GORM AutoMigrate migrations.
func RunMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&Message{},
	)
}
