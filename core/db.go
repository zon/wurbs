package core

import (
	"github.com/zon/chat/core/pg"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() error {
	db, err := pg.Open()
	if err != nil {
		return err
	}

	DB = db
	return nil
}

func AutoMigrate() error {
	return DB.AutoMigrate(&Message{})
}
