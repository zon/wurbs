package pg

import (
	"encoding/json"
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

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

func (s *Secret) Read(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, s)
}

func (s *Secret) Write(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (s *Secret) Open() (*gorm.DB, error) {
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
