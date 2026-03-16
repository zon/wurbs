package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPatchURI(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		newHost  string
		newPort  string
		expected string
	}{
		{
			name:     "simple URI",
			uri:      "postgresql://user:pass@host:5432/db",
			newHost:  "10.96.0.1",
			newPort:  "32432",
			expected: "postgresql://user:pass@10.96.0.1:32432/db",
		},
		{
			name:     "JDBC URI",
			uri:      "jdbc:postgresql://host:5432/db",
			newHost:  "10.96.0.1",
			newPort:  "32432",
			expected: "jdbc:postgresql://10.96.0.1:32432/db",
		},
		{
			name:     "empty URI",
			uri:      "",
			newHost:  "10.96.0.1",
			newPort:  "32432",
			expected: "",
		},
		{
			name:     "port already patched",
			uri:      "postgresql://user:pass@host:32432/db",
			newHost:  "10.96.0.1",
			newPort:  "32432",
			expected: "postgresql://user:pass@10.96.0.1:32432/db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := patchURI(tt.uri, tt.newHost, tt.newPort)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidIP(t *testing.T) {
	tests := []struct {
		ip    string
		valid bool
	}{
		{"10.96.0.1", true},
		{"192.168.1.1", true},
		{"127.0.0.1", true},
		{"invalid", false},
		{"", false},
		{"10.96.0.1.1", false},
		{"0.0.0.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			result := isValidIP(tt.ip)
			assert.Equal(t, tt.valid, result)
		})
	}
}
