package channel

import (
	"errors"
	"fmt"

	"github.com/zon/chat/core/auth"
	"gorm.io/gorm"
)

type Channel struct {
	gorm.Model
	Name     string `gorm:"uniqueIndex"`
	IsPublic bool
	IsTest   bool
	Members  []auth.User `gorm:"many2many:memberships;"`
}

type Membership struct {
	ChannelID uint `gorm:"primaryKey"`
	UserID    uint `gorm:"primaryKey"`
}

var (
	ErrNotFound       = errors.New("channel: not found")
	ErrTestUserInReal = errors.New("channel: test users cannot join real channels")
)

func Create(db *gorm.DB, name string, isPublic, isTest bool) (*Channel, error) {
	ch := &Channel{
		Name:     name,
		IsPublic: isPublic,
		IsTest:   isTest,
	}
	if err := db.Create(ch).Error; err != nil {
		return nil, fmt.Errorf("channel: failed to create: %w", err)
	}
	return ch, nil
}

func Get(db *gorm.DB, id uint) (*Channel, error) {
	var ch Channel
	if err := db.First(&ch, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("channel: failed to get: %w", err)
	}
	return &ch, nil
}

func List(db *gorm.DB) ([]Channel, error) {
	var channels []Channel
	if err := db.Find(&channels).Error; err != nil {
		return nil, fmt.Errorf("channel: failed to list: %w", err)
	}
	return channels, nil
}

func Delete(db *gorm.DB, id uint) error {
	result := db.Delete(&Channel{}, id)
	if result.Error != nil {
		return fmt.Errorf("channel: failed to delete: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func AddMember(db *gorm.DB, channelID uint, user *auth.User) error {
	ch, err := Get(db, channelID)
	if err != nil {
		return err
	}

	if !ch.IsTest && user.IsTest {
		return ErrTestUserInReal
	}

	membership := Membership{ChannelID: channelID, UserID: user.ID}
	if err := db.Create(&membership).Error; err != nil {
		return fmt.Errorf("channel: failed to add member: %w", err)
	}
	return nil
}

func RemoveMember(db *gorm.DB, channelID, userID uint) error {
	result := db.Where("channel_id = ? AND user_id = ?", channelID, userID).
		Delete(&Membership{})
	if result.Error != nil {
		return fmt.Errorf("channel: failed to remove member: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func Members(db *gorm.DB, channelID uint) ([]auth.User, error) {
	ch, err := Get(db, channelID)
	if err != nil {
		return nil, err
	}

	var users []auth.User
	if err := db.Model(ch).Association("Members").Find(&users); err != nil {
		return nil, fmt.Errorf("channel: failed to list members: %w", err)
	}
	return users, nil
}
