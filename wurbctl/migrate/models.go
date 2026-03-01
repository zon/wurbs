package migrate

import "gorm.io/gorm"

// Message mirrors the core.Message model for database migration purposes.
// This avoids a dependency on the core package (which currently requires gonf).
type Message struct {
	gorm.Model
	UserID  uint
	Content string
}
