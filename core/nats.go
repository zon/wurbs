package core

import (
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
)

var nc *nats.Conn

func ConnectNATS(cfg *Config, secrets *Secrets) error {
	opts := []nats.Option{nats.Name("wurbs-rest")}

	if secrets.NATS.Token != "" {
		opts = append(opts, nats.Token(secrets.NATS.Token))
	}

	conn, err := nats.Connect(cfg.NATS.URL, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}

	nc = conn
	return nil
}

func GetNATSConn() *nats.Conn {
	return nc
}

func Publish(subject string, data interface{}) error {
	if nc == nil {
		return fmt.Errorf("NATS not connected")
	}

	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	err = nc.Publish(subject, body)
	if err != nil {
		return fmt.Errorf("failed to publish: %w", err)
	}

	return nil
}

func PublishMessage(action string, msg *Message) error {
	event := map[string]interface{}{
		"action":     action,
		"id":         msg.ID,
		"user_id":    msg.UserID,
		"content":    msg.Content,
		"created_at": msg.CreatedAt,
	}
	return Publish("messages", event)
}
