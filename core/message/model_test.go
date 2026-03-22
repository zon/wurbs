package message

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestMessageModel(t *testing.T) {
	m := Message{}
	val := reflect.TypeOf(m)

	// Check required fields
	_, okID := val.FieldByName("ID")
	assert.True(t, okID, "Message should have ID field")

	_, okCreatedAt := val.FieldByName("CreatedAt")
	assert.True(t, okCreatedAt, "Message should have CreatedAt field")

	_, okUpdatedAt := val.FieldByName("UpdatedAt")
	assert.True(t, okUpdatedAt, "Message should have UpdatedAt field")

	// Check forbidden fields
	_, okDeletedAt := val.FieldByName("DeletedAt")
	assert.False(t, okDeletedAt, "Message should NOT have DeletedAt field")
}
