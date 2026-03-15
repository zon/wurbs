package core

import (
	"encoding/json"
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Secret struct {
	Username string `json:"username"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	URI      string `json:"uri"`
	PGPass   string `json:"pgpass"`
	JDBCURI  string `json:"jdbc-uri"`
	FQDNURI  string `json:"fqdn-uri"`
	FQDNJDBC string `json:"fqdn-jdbc-uri"`
}

func ReadSecret(path string) (*Secret, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret file: %w", err)
	}

	var secret Secret
	if err := json.Unmarshal(data, &secret); err != nil {
		return nil, fmt.Errorf("failed to parse secret file: %w", err)
	}

	return &secret, nil
}

func WriteSecret(path string, secret *Secret) error {
	data, err := json.MarshalIndent(secret, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal secret: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write secret file: %w", err)
	}

	return nil
}

func OpenDB(secret *Secret) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		secret.Host, secret.Port, secret.Username, secret.Password, secret.DBName,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	return db, nil
}
