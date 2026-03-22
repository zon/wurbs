package channel

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestChannelModel(t *testing.T) {
	c := Channel{}
	val := reflect.TypeOf(c)

	// Check required fields
	_, okID := val.FieldByName("ID")
	assert.True(t, okID, "Channel should have ID field")

	_, okCreatedAt := val.FieldByName("CreatedAt")
	assert.True(t, okCreatedAt, "Channel should have CreatedAt field")

	_, okUpdatedAt := val.FieldByName("UpdatedAt")
	assert.True(t, okUpdatedAt, "Channel should have UpdatedAt field")

	// Check forbidden fields
	_, okDeletedAt := val.FieldByName("DeletedAt")
	assert.False(t, okDeletedAt, "Channel should NOT have DeletedAt field")
}
