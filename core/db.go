package core

import (
	"path/filepath"

	"github.com/zon/chat/core/config"
	"github.com/zon/chat/core/pg"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() error {
	configDir, err := config.ConfigDir()
	if err != nil {
		return err
	}

	secret := &pg.Secret{}
	err = secret.Read(filepath.Join(configDir, "postgres.json"))
	if err != nil {
		return err
	}

	db, err := secret.Open()
	if err != nil {
		return err
	}

	DB = db
	return nil
}

func AutoMigrate() error {
	return DB.AutoMigrate(
		&Message{},
	)
}
