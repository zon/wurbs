package message

import (
	"errors"
	"fmt"
	"time"

	"github.com/zon/chat/core/auth"
	"gorm.io/gorm"
)

// NATSPublisher defines the interface for publishing to NATS.
type NATSPublisher interface {
	Publish(subject string, data any) error
}

// Message is the chat message model. The message module owns this type.
type Message struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	ChannelID uint
	UserID    uint
	User      auth.User `gorm:"foreignKey:UserID"`
	Content   string
}

type MessageEvent struct {
	Type    string  `json:"type"`
	Message Message `json:"message"`
}

// Page holds a page of messages and a cursor for fetching the next page.
type Page struct {
	Messages []Message
	// NextCursor is the ID of the last message in this page. Zero means no more pages.
	NextCursor uint
}

// Errors returned by the message module.
var (
	ErrNotFound = errors.New("message: not found")
)

// natsSubject returns the NATS subject for a channel's messages.
func natsSubject(channelID uint) string {
	return fmt.Sprintf("wurbs.channel.%d.messages", channelID)
}

// Create persists a new message and publishes it to NATS. The nc parameter
// may be nil, in which case NATS publishing is skipped.
func Create(db *gorm.DB, nc NATSPublisher, channelID, userID uint, content string) (*Message, error) {
	m := &Message{
		ChannelID: channelID,
		UserID:    userID,
		Content:   content,
	}
	if err := db.Create(m).Error; err != nil {
		return nil, fmt.Errorf("message: failed to create: %w", err)
	}

	if nc != nil {
		event := MessageEvent{Type: "created", Message: *m}
		if err := nc.Publish(natsSubject(channelID), event); err != nil {
			return nil, fmt.Errorf("message: failed to publish: %w", err)
		}
	}

	return m, nil
}

// List returns a page of messages for a channel, ordered newest first.
// The before and after parameters are timestamps to filter messages.
// If both are provided, after takes precedence.
// Use cursor=0 to start from the most recent messages. Subsequent pages
// use the NextCursor from the previous Page.
func List(db *gorm.DB, channelID uint, cursor uint, limit int, before, after *time.Time) (*Page, error) {
	if limit <= 0 {
		limit = 50
	}

	q := db.Where("channel_id = ?", channelID).
		Order("id DESC").
		Limit(limit + 1)

	if cursor > 0 {
		q = q.Where("id < ?", cursor)
	}

	if after != nil {
		q = q.Where("created_at > ?", after)
	} else if before != nil {
		q = q.Where("created_at < ?", before)
	}

	var messages []Message
	if err := q.Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("message: failed to list: %w", err)
	}

	page := &Page{}
	if len(messages) > limit {
		messages = messages[:limit]
		page.NextCursor = messages[limit-1].ID
	}
	page.Messages = messages

	return page, nil
}

// Get retrieves a single message by ID.
func Get(db *gorm.DB, id uint) (*Message, error) {
	var m Message
	if err := db.First(&m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("message: failed to get: %w", err)
	}
	return &m, nil
}

// Update modifies an existing message's content and publishes to NATS.
func Update(db *gorm.DB, nc NATSPublisher, id uint, content string) (*Message, error) {
	result := db.Model(&Message{}).Where("id = ?", id).Update("content", content)
	if result.Error != nil {
		return nil, fmt.Errorf("message: failed to update: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	m, err := Get(db, id)
	if err != nil {
		return nil, err
	}

	if nc != nil {
		event := MessageEvent{Type: "updated", Message: *m}
		if err := nc.Publish(natsSubject(m.ChannelID), event); err != nil {
			return nil, fmt.Errorf("message: failed to publish: %w", err)
		}
	}

	return m, nil
}

// Delete removes a message by ID and publishes to NATS.
func Delete(db *gorm.DB, nc NATSPublisher, id uint) error {
	m, err := Get(db, id)
	if err != nil {
		return err
	}

	result := db.Delete(&Message{}, id)
	if result.Error != nil {
		return fmt.Errorf("message: failed to delete: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}

	if nc != nil {
		event := MessageEvent{Type: "deleted", Message: *m}
		if err := nc.Publish(natsSubject(m.ChannelID), event); err != nil {
			return fmt.Errorf("message: failed to publish: %w", err)
		}
	}

	return nil
}
