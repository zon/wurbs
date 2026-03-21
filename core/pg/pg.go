package pg

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/zon/chat/core/config"
	"github.com/zon/chat/core/k8s"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

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

// Patch sets Host and Port and replaces the host:port component in all URI fields.
func (s *Secret) Patch(host, port string) {
	s.Host = host
	s.Port = port
	s.URI = patchURI(s.URI, host, port)
	s.JDBCURI = patchURI(s.JDBCURI, host, port)
	s.FQDNURI = patchURI(s.FQDNURI, host, port)
	s.FQDNJDBCURI = patchURI(s.FQDNJDBCURI, host, port)
}

func patchURI(uri, host, port string) string {
	const jdbcPrefix = "jdbc:"
	prefix := ""
	rest := uri
	if len(uri) > len(jdbcPrefix) && uri[:len(jdbcPrefix)] == jdbcPrefix {
		prefix = jdbcPrefix
		rest = uri[len(jdbcPrefix):]
	}
	u, err := url.Parse(rest)
	if err != nil || u.Host == "" {
		return uri
	}
	u.Host = host + ":" + port
	return prefix + u.String()
}

// ReadK8s populates the secret from a Kubernetes secret.
func (s *Secret) ReadK8s(name, namespace, context string) error {
	data, err := k8s.GetSecret(name, namespace, context)
	if err != nil {
		return err
	}
	s.Username = data["username"]
	s.Password = data["password"]
	s.DBName = data["dbname"]
	s.Host = data["host"]
	s.Port = data["port"]
	s.URI = data["uri"]
	s.PGPass = data["pgpass"]
	s.JDBCURI = data["jdbc-uri"]
	s.FQDNURI = data["fqdn-uri"]
	s.FQDNJDBCURI = data["fqdn-jdbc-uri"]
	return nil
}

// WriteK8s applies the secret to a Kubernetes namespace.
func (s *Secret) WriteK8s(name, namespace, context string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return k8s.ApplySecret(name, namespace, context, map[string]string{
		"postgres.json": string(data),
	})
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
	tree, err := config.Dir()
	if err != nil {
		return nil, err
	}

	return OpenAt(tree.Postgres)
}

// OpenAt reads postgres.json from path and returns a connected *gorm.DB handle.
func OpenAt(path string) (*gorm.DB, error) {
	var s Secret
	if err := s.read(path); err != nil {
		return nil, err
	}

	return s.open()
}
