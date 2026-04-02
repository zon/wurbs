// Package asyncapi provides primitives to interact with the asyncapi WebSocket API.
//
// Code generated from AsyncAPI specification. DO NOT EDIT.
package asyncapi

import (
	"time"
)

// User defines model for User.
type User struct {
	ID        string    `json:"id"`
	Username  *string   `json:"username,omitempty"`
	Email     string    `json:"email"`
	Admin     bool      `json:"admin"`
	Inactive  bool      `json:"inactive"`
	CreatedAt time.Time `json:"createdAt"`
}

// Message defines model for Message.
type Message struct {
	ID        string     `json:"id"`
	ChannelID string     `json:"channelId"`
	UserID    string     `json:"userId"`
	Body      string     `json:"body"`
	CreatedAt time.Time  `json:"createdAt"`
	EditedAt  *time.Time `json:"editedAt,omitempty"`
}

// MessageEvent defines model for MessageEvent.
type MessageEvent struct {
	// Type enum: created, updated, deleted
	Type    string   `json:"type"`
	Message *Message `json:"message"`
}

// MemberEvent defines model for MemberEvent.
type MemberEvent struct {
	// Type enum: joined, left
	Type string `json:"type"`
	User *User  `json:"user"`
}

// UserEvent defines model for UserEvent.
type UserEvent struct {
	UserID   string  `json:"userId"`
	Username *string `json:"username"`
}

// Unsubscribe defines model for Unsubscribe.
type Unsubscribe struct{}
