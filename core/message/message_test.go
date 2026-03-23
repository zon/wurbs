package message

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zon/chat/core/user"
	"gorm.io/gorm"
)

func createUser(t *testing.T, db *gorm.DB, email, subject string) *user.User {
	t.Helper()
	u := &user.User{Email: email, Subject: subject}
	require.NoError(t, db.Create(u).Error)
	return u
}

// --- Create tests ---

func TestCreate_Success(t *testing.T) {
	t.Skip("skipping test that requires database")
	var db *gorm.DB
	user := createUser(t, db, "user@example.com", "sub-user")

	msg, err := Create(db, nil, 1, user.ID, "hello world")
	require.NoError(t, err)
	assert.NotZero(t, msg.ID)
	assert.Equal(t, uint(1), msg.ChannelID)
	assert.Equal(t, user.ID, msg.UserID)
	assert.Equal(t, "hello world", msg.Content)
}

func TestCreate_PersistsToDatabase(t *testing.T) {
	t.Skip("skipping test that requires database")
	var db *gorm.DB
	user := createUser(t, db, "user@example.com", "sub-user")

	msg, err := Create(db, nil, 1, user.ID, "persistent message")
	require.NoError(t, err)

	var loaded Message
	require.NoError(t, db.First(&loaded, msg.ID).Error)
	assert.Equal(t, "persistent message", loaded.Content)
	assert.Equal(t, uint(1), loaded.ChannelID)
	assert.Equal(t, user.ID, loaded.UserID)
}

func TestCreate_NilNatsSkipsPublish(t *testing.T) {
	t.Skip("skipping test that requires database")
	var db *gorm.DB
	user := createUser(t, db, "user@example.com", "sub-user")

	// Passing nil NATS connection should not error.
	msg, err := Create(db, nil, 1, user.ID, "no nats")
	require.NoError(t, err)
	assert.NotZero(t, msg.ID)
}

func TestCreate_MultipleMessages(t *testing.T) {
	t.Skip("skipping test that requires database")
	var db *gorm.DB
	user := createUser(t, db, "user@example.com", "sub-user")

	msg1, err := Create(db, nil, 1, user.ID, "first")
	require.NoError(t, err)
	msg2, err := Create(db, nil, 1, user.ID, "second")
	require.NoError(t, err)

	assert.NotEqual(t, msg1.ID, msg2.ID)
	assert.True(t, msg2.ID > msg1.ID)
}

func TestCreate_DifferentChannels(t *testing.T) {
	t.Skip("skipping test that requires database")
	var db *gorm.DB
	user := createUser(t, db, "user@example.com", "sub-user")

	msg1, err := Create(db, nil, 1, user.ID, "channel 1")
	require.NoError(t, err)
	msg2, err := Create(db, nil, 2, user.ID, "channel 2")
	require.NoError(t, err)

	assert.Equal(t, uint(1), msg1.ChannelID)
	assert.Equal(t, uint(2), msg2.ChannelID)
}

// --- Get tests ---

func TestGet_Found(t *testing.T) {
	t.Skip("skipping test that requires database")
	var db *gorm.DB
	user := createUser(t, db, "user@example.com", "sub-user")

	created, err := Create(db, nil, 1, user.ID, "find me")
	require.NoError(t, err)

	msg, err := Get(db, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "find me", msg.Content)
	assert.Equal(t, created.ID, msg.ID)
}

func TestGet_NotFound(t *testing.T) {
	t.Skip("skipping test that requires database")
	var db *gorm.DB

	_, err := Get(db, 999)
	assert.ErrorIs(t, err, ErrNotFound)
}

// --- List tests ---

func TestList_Empty(t *testing.T) {
	t.Skip("skipping test that requires database")
	var db *gorm.DB

	page, err := List(db, 1, 0, 10, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, page.Messages)
	assert.Zero(t, page.NextCursor)
}

func TestList_ReturnsMessagesForChannel(t *testing.T) {
	t.Skip("skipping test that requires database")
	var db *gorm.DB
	user := createUser(t, db, "user@example.com", "sub-user")

	_, err := Create(db, nil, 1, user.ID, "ch1 msg")
	require.NoError(t, err)
	_, err = Create(db, nil, 2, user.ID, "ch2 msg")
	require.NoError(t, err)

	page, err := List(db, 1, 0, 10, nil, nil)
	require.NoError(t, err)
	assert.Len(t, page.Messages, 1)
	assert.Equal(t, "ch1 msg", page.Messages[0].Content)
}

func TestList_OrderNewestFirst(t *testing.T) {
	t.Skip("skipping test that requires database")
	var db *gorm.DB
	user := createUser(t, db, "user@example.com", "sub-user")

	_, err := Create(db, nil, 1, user.ID, "first")
	require.NoError(t, err)
	_, err = Create(db, nil, 1, user.ID, "second")
	require.NoError(t, err)
	_, err = Create(db, nil, 1, user.ID, "third")
	require.NoError(t, err)

	page, err := List(db, 1, 0, 10, nil, nil)
	require.NoError(t, err)
	require.Len(t, page.Messages, 3)
	assert.Equal(t, "third", page.Messages[0].Content)
	assert.Equal(t, "second", page.Messages[1].Content)
	assert.Equal(t, "first", page.Messages[2].Content)
}

func TestList_Pagination_FirstPage(t *testing.T) {
	t.Skip("skipping test that requires database")
	var db *gorm.DB
	user := createUser(t, db, "user@example.com", "sub-user")

	for i := 0; i < 5; i++ {
		_, err := Create(db, nil, 1, user.ID, fmt.Sprintf("msg-%d", i))
		require.NoError(t, err)
	}

	page, err := List(db, 1, 0, 3, nil, nil)
	require.NoError(t, err)
	assert.Len(t, page.Messages, 3)
	assert.NotZero(t, page.NextCursor, "should have a next cursor when more messages exist")
}

func TestList_Pagination_NextPage(t *testing.T) {
	t.Skip("skipping test that requires database")
	var db *gorm.DB
	user := createUser(t, db, "user@example.com", "sub-user")

	for i := 0; i < 5; i++ {
		_, err := Create(db, nil, 1, user.ID, fmt.Sprintf("msg-%d", i))
		require.NoError(t, err)
	}

	// First page
	page1, err := List(db, 1, 0, 3, nil, nil)
	require.NoError(t, err)
	assert.Len(t, page1.Messages, 3)
	assert.NotZero(t, page1.NextCursor)

	// Second page using cursor
	page2, err := List(db, 1, page1.NextCursor, 3, nil, nil)
	require.NoError(t, err)
	assert.Len(t, page2.Messages, 2)
	assert.Zero(t, page2.NextCursor, "no more pages after this")

	// Ensure no overlap between pages
	page1IDs := make(map[uint]bool)
	for _, m := range page1.Messages {
		page1IDs[m.ID] = true
	}
	for _, m := range page2.Messages {
		assert.False(t, page1IDs[m.ID], "page 2 should not contain messages from page 1")
	}
}

func TestList_Pagination_AllMessages(t *testing.T) {
	t.Skip("skipping test that requires database")
	var db *gorm.DB
	user := createUser(t, db, "user@example.com", "sub-user")

	for i := 0; i < 7; i++ {
		_, err := Create(db, nil, 1, user.ID, fmt.Sprintf("msg-%d", i))
		require.NoError(t, err)
	}

	// Collect all messages across pages
	var all []Message
	cursor := uint(0)
	for {
		page, err := List(db, 1, cursor, 3, nil, nil)
		require.NoError(t, err)
		all = append(all, page.Messages...)
		if page.NextCursor == 0 {
			break
		}
		cursor = page.NextCursor
	}

	assert.Len(t, all, 7, "should collect all 7 messages across pages")
}

func TestList_ExactPageSize(t *testing.T) {
	t.Skip("skipping test that requires database")
	var db *gorm.DB
	user := createUser(t, db, "user@example.com", "sub-user")

	for i := 0; i < 3; i++ {
		_, err := Create(db, nil, 1, user.ID, fmt.Sprintf("msg-%d", i))
		require.NoError(t, err)
	}

	page, err := List(db, 1, 0, 3, nil, nil)
	require.NoError(t, err)
	assert.Len(t, page.Messages, 3)
	assert.Zero(t, page.NextCursor, "no more pages when exactly page size messages exist")
}

func TestList_DefaultLimit(t *testing.T) {
	t.Skip("skipping test that requires database")
	var db *gorm.DB
	user := createUser(t, db, "user@example.com", "sub-user")

	for i := 0; i < 3; i++ {
		_, err := Create(db, nil, 1, user.ID, fmt.Sprintf("msg-%d", i))
		require.NoError(t, err)
	}

	// Passing 0 or negative limit should use default (50)
	page, err := List(db, 1, 0, 0, nil, nil)
	require.NoError(t, err)
	assert.Len(t, page.Messages, 3)

	page, err = List(db, 1, 0, -1, nil, nil)
	require.NoError(t, err)
	assert.Len(t, page.Messages, 3)
}

func TestList_IsolatesChannels(t *testing.T) {
	t.Skip("skipping test that requires database")
	var db *gorm.DB
	user := createUser(t, db, "user@example.com", "sub-user")

	for i := 0; i < 3; i++ {
		_, err := Create(db, nil, 1, user.ID, "ch1")
		require.NoError(t, err)
	}
	for i := 0; i < 2; i++ {
		_, err := Create(db, nil, 2, user.ID, "ch2")
		require.NoError(t, err)
	}

	page1, err := List(db, 1, 0, 10, nil, nil)
	require.NoError(t, err)
	assert.Len(t, page1.Messages, 3)

	page2, err := List(db, 2, 0, 10, nil, nil)
	require.NoError(t, err)
	assert.Len(t, page2.Messages, 2)
}

// --- NATS subject tests ---

func TestNatsSubject(t *testing.T) {
	assert.Equal(t, "wurbs.channel.1.messages", natsSubject(1))
	assert.Equal(t, "wurbs.channel.42.messages", natsSubject(42))
}

// --- Message model field persistence ---

func TestMessageModel_FieldsPersist(t *testing.T) {
	t.Skip("skipping test that requires database")
	var db *gorm.DB
	user := createUser(t, db, "user@example.com", "sub-user")

	msg, err := Create(db, nil, 5, user.ID, "persist test")
	require.NoError(t, err)

	var loaded Message
	require.NoError(t, db.First(&loaded, msg.ID).Error)

	assert.Equal(t, uint(5), loaded.ChannelID)
	assert.Equal(t, user.ID, loaded.UserID)
	assert.Equal(t, "persist test", loaded.Content)
	assert.NotZero(t, loaded.CreatedAt)
	assert.NotZero(t, loaded.UpdatedAt)
}
