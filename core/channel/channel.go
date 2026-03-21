package channel

import (
	"errors"
	"fmt"

	"github.com/zon/chat/core/auth"
	"gorm.io/gorm"
)

type Channel struct {
	gorm.Model
	Name        string `gorm:"uniqueIndex"`
	Description string
	IsPublic    bool
	IsActive    bool `gorm:"default:true"`
	IsTest      bool
	Members     []auth.User `gorm:"many2many:memberships;"`
}

type Membership struct {
	ChannelID uint `gorm:"primaryKey"`
	UserID    uint `gorm:"primaryKey"`
}

var (
	ErrNotFound                = errors.New("channel: not found")
	ErrTestUserInReal          = errors.New("channel: test users cannot join real channels")
	ErrRealUserInTest          = errors.New("channel: real users cannot join test channels")
	ErrRealAdminInTest         = errors.New("channel: real admins cannot manage test channels")
	ErrTestAdminInReal         = errors.New("channel: test admins cannot manage real channels")
	ErrRealAdminModifyTestUser = errors.New("channel: real admins cannot modify test users")
	ErrTestAdminModifyRealUser = errors.New("channel: test admins cannot modify real users")
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

func CreateAsAdmin(db *gorm.DB, admin *auth.User, name string, isPublic, isTest bool) (*Channel, error) {
	if admin.IsTest && !isTest {
		return nil, ErrTestAdminInReal
	}
	if !admin.IsTest && isTest {
		return nil, ErrRealAdminInTest
	}
	return Create(db, name, isPublic, isTest)
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

type UpdateInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsPublic    *bool   `json:"is_public"`
	IsActive    *bool   `json:"is_active"`
}

func Update(db *gorm.DB, ch *Channel, input UpdateInput) error {
	if input.Name != nil {
		ch.Name = *input.Name
	}
	if input.Description != nil {
		ch.Description = *input.Description
	}
	if input.IsPublic != nil {
		ch.IsPublic = *input.IsPublic
	}
	if input.IsActive != nil {
		ch.IsActive = *input.IsActive
	}

	if err := db.Session(&gorm.Session{FullSaveAssociations: true}).Save(ch).Error; err != nil {
		return fmt.Errorf("channel: failed to update: %w", err)
	}
	return nil
}

func UpdateAsAdmin(db *gorm.DB, ch *Channel, admin *auth.User, input UpdateInput) error {
	if admin.IsTest && !ch.IsTest {
		return ErrTestAdminInReal
	}
	if !admin.IsTest && ch.IsTest {
		return ErrRealAdminInTest
	}
	return Update(db, ch, input)
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

func DeleteAsAdmin(db *gorm.DB, id uint, admin *auth.User) error {
	ch, err := Get(db, id)
	if err != nil {
		return err
	}
	if admin.IsTest && !ch.IsTest {
		return ErrTestAdminInReal
	}
	if !admin.IsTest && ch.IsTest {
		return ErrRealAdminInTest
	}
	return Delete(db, id)
}

func AddMember(db *gorm.DB, channelID uint, user *auth.User) error {
	ch, err := Get(db, channelID)
	if err != nil {
		return err
	}

	if !ch.IsTest && user.IsTest {
		return ErrTestUserInReal
	}
	if ch.IsTest && !user.IsTest {
		return ErrRealUserInTest
	}

	membership := Membership{ChannelID: channelID, UserID: user.ID}
	if err := db.Create(&membership).Error; err != nil {
		return fmt.Errorf("channel: failed to add member: %w", err)
	}
	return nil
}

func AddMemberAsAdmin(db *gorm.DB, channelID uint, admin, user *auth.User) error {
	ch, err := Get(db, channelID)
	if err != nil {
		return err
	}

	if admin.IsTest && !ch.IsTest {
		return ErrTestAdminInReal
	}
	if !admin.IsTest && ch.IsTest {
		return ErrRealAdminInTest
	}

	if admin.IsTest && !user.IsTest {
		return ErrTestAdminModifyRealUser
	}
	if !admin.IsTest && user.IsTest {
		return ErrRealAdminModifyTestUser
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

func RemoveMemberAsAdmin(db *gorm.DB, channelID, userID uint, admin *auth.User) error {
	ch, err := Get(db, channelID)
	if err != nil {
		return err
	}

	if admin.IsTest && !ch.IsTest {
		return ErrTestAdminInReal
	}
	if !admin.IsTest && ch.IsTest {
		return ErrRealAdminInTest
	}

	user, err := auth.GetUserByID(db, fmt.Sprintf("%d", userID))
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return RemoveMember(db, channelID, userID)
		}
		return err
	}

	if admin.IsTest && !user.IsTest {
		return ErrTestAdminModifyRealUser
	}
	if !admin.IsTest && user.IsTest {
		return ErrRealAdminModifyTestUser
	}

	return RemoveMember(db, channelID, userID)
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
