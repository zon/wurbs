package nats

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nats-io/nats.go"
	"github.com/zon/chat/core/config"
)

const (
	// Default path where k8s mounts the service account token.
	k8sTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

const envNatsTokenFile = "WURB_NATS_TOKEN_FILE"

// secret holds NATS connection details loaded from secret.yaml.
type secret struct {
	NATS struct {
		URL string `yaml:"url"`
	} `yaml:"nats"`
}

// Conn wraps a NATS connection.
type Conn struct {
	nc *nats.Conn
}

// readToken reads the k8s service account token from the standard mount path.
// Returns an empty string if the file does not exist.
var readToken = func() (string, error) {
	data, err := os.ReadFile(k8sTokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("nats: failed to read service token: %w", err)
	}
	return string(data), nil
}

// readTokenFromFile reads a NATS token from a custom file path.
// Returns an empty string if the file does not exist.
var readTokenFromFile = func(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("nats: failed to read token file: %w", err)
	}
	return string(data), nil
}

// dial opens a NATS connection. It is a package-level variable so tests can
// replace it.
var dial = func(url string, opts ...nats.Option) (*nats.Conn, error) {
	return nats.Connect(url, opts...)
}

// Connect loads the NATS URL from secret.yaml, reads a k8s service account
// token for auth callout if available, and returns a connected Conn.
// If WURB_NATS_TOKEN_FILE is set, the token from that file takes precedence
// over the k8s service account token.
func Connect() (*Conn, error) {
	var s secret
	if err := config.LoadSecret(&s); err != nil {
		return nil, fmt.Errorf("nats: failed to load secrets: %w", err)
	}

	if s.NATS.URL == "" {
		return nil, fmt.Errorf("nats: url not configured in secrets")
	}

	var opts []nats.Option

	token, err := func() (string, error) {
		if tokenFile := os.Getenv(envNatsTokenFile); tokenFile != "" {
			return readTokenFromFile(tokenFile)
		}
		return readToken()
	}()
	if err != nil {
		return nil, err
	}
	if token != "" {
		opts = append(opts, nats.Token(token))
	}

	nc, err := dial(s.NATS.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("nats: connection failed: %w", err)
	}

	return &Conn{nc: nc}, nil
}

// Publish JSON-encodes data and publishes it to the given NATS subject.
func (c *Conn) Publish(subject string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("nats: failed to marshal message: %w", err)
	}
	if err := c.nc.Publish(subject, payload); err != nil {
		return fmt.Errorf("nats: publish failed: %w", err)
	}
	return nil
}

// Subscription wraps a NATS subscription.
type Subscription struct {
	sub *nats.Subscription
}

// Subscribe registers a callback for messages on the given NATS subject.
// The callback receives the raw message payload. Returns a Subscription
// that can be unsubscribed.
func (c *Conn) Subscribe(subject string, cb func([]byte)) (*Subscription, error) {
	sub, err := c.nc.Subscribe(subject, func(msg *nats.Msg) {
		cb(msg.Data)
	})
	if err != nil {
		return nil, fmt.Errorf("nats: subscribe failed: %w", err)
	}
	return &Subscription{sub: sub}, nil
}

// Unsubscribe removes the subscription.
func (s *Subscription) Unsubscribe() error {
	return s.sub.Unsubscribe()
}

// Close closes the NATS connection.
func (c *Conn) Close() {
	c.nc.Close()
}
