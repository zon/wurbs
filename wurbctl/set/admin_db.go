package set

import (
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// OpenAdminDB opens a GORM database connection using standard PG environment variables.
func OpenAdminDB() (*gorm.DB, error) {
	dsn, err := adminDSN()
	if err != nil {
		return nil, err
	}
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

// adminDSN builds a PostgreSQL DSN from standard PG environment variables.
func adminDSN() (string, error) {
	host := os.Getenv("PGHOST")
	port := os.Getenv("PGPORT")
	user := os.Getenv("PGUSER")
	password := os.Getenv("PGPASSWORD")
	database := os.Getenv("PGDATABASE")

	var missing []string
	if host == "" {
		missing = append(missing, "PGHOST")
	}
	if user == "" {
		missing = append(missing, "PGUSER")
	}
	if database == "" {
		missing = append(missing, "PGDATABASE")
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("missing required environment variables: %v", missing)
	}

	if port == "" {
		port = "5432"
	}

	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, database,
	), nil
}
