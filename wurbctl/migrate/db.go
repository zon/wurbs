package migrate

import (
	"fmt"
	"os"
)

// DSN builds a PostgreSQL DSN from standard PG environment variables.
// Returns an error if any required variable is missing.
func DSN() (string, error) {
	host := os.Getenv("PGHOST")
	port := os.Getenv("PGPORT")
	user := os.Getenv("PGUSER")
	password := os.Getenv("PGPASSWORD")
	database := os.Getenv("PGDATABASE")

	missing := []string{}
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

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, database,
	)
	return dsn, nil
}
