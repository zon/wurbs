package pg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zon/chat/core/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const secretFile = "postgres.json"

// Secret holds PostgreSQL connection credentials.
type Secret struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DBName      string `json:"dbname"`
	Host        string `json:"host"`
	Port        string `json:"port"`
	URI         string `json:"uri"`
	PGPass      string `json:"pgpass"`
	JDBCURI     string `json:"jdbc-uri"`
	FQDNURI     string `json:"fqdn-uri"`
	FQDNJDBCURI string `json:"fqdn-jdbc-uri"`
}

// Write serializes the secret to a JSON file at path.
func (s *Secret) Write(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (s *Secret) read(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, s)
}

func (s *Secret) open() (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		s.Host, s.Username, s.Password, s.DBName, s.Port)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	return db, nil
}

// Open resolves the config directory, reads postgres.json, and returns a
// connected *gorm.DB handle ready for use.
func Open() (*gorm.DB, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, err
	}

	var s Secret
	if err := s.read(filepath.Join(dir, secretFile)); err != nil {
		return nil, err
	}

	return s.open()
}

// Migrate runs GORM AutoMigrate for the given models.
func Migrate(db *gorm.DB, models ...any) error {
	if db == nil {
		return fmt.Errorf("pg: nil database handle")
	}
	return db.AutoMigrate(models...)
}
