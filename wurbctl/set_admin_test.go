package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- adminSecretName ---

func TestAdminSecretName_Basic(t *testing.T) {
	assert.Equal(t, "wurbs-admin-admin-example-com", adminSecretName("admin@example.com"))
}

func TestAdminSecretName_SubdomainEmail(t *testing.T) {
	assert.Equal(t, "wurbs-admin-alice-sub-domain-org", adminSecretName("alice@sub.domain.org"))
}

func TestAdminSecretName_Uppercase(t *testing.T) {
	assert.Equal(t, "wurbs-admin-admin-example-com", adminSecretName("Admin@Example.COM"))
}

func TestAdminSecretName_StripsInvalidChars(t *testing.T) {
	name := adminSecretName("test.user+tag@example.com")
	assert.True(t, len(name) > len("wurbs-admin-"))
	for _, ch := range name[len("wurbs-admin-"):] {
		assert.True(t, (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-')
	}
}

