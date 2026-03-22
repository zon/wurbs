package user

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestUserModel(t *testing.T) {
	u := User{}
	val := reflect.TypeOf(u)

	// Check required fields
	_, okID := val.FieldByName("ID")
	assert.True(t, okID, "User should have ID field")

	_, okCreatedAt := val.FieldByName("CreatedAt")
	assert.True(t, okCreatedAt, "User should have CreatedAt field")

	_, okUpdatedAt := val.FieldByName("UpdatedAt")
	assert.True(t, okUpdatedAt, "User should have UpdatedAt field")

	// Check forbidden fields
	_, okDeletedAt := val.FieldByName("DeletedAt")
	assert.False(t, okDeletedAt, "User should NOT have DeletedAt field")
}
