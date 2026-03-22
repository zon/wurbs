package nats

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zon/chat/core/config"
)

func TestConnect_MissingSecretFile(t *testing.T) {
	config.ResetCache()
	defer config.ResetCache()
	tmpDir := t.TempDir()
	t.Setenv("WURB_CONFIG", tmpDir)

	_, err := Connect()
	assert.Error(t, err, "Connect should fail when config.yaml is missing")
}

func TestPublish_Success(t *testing.T) {
	type testMessage struct {
		ID   int    `json:"id"`
		Text string `json:"text"`
	}

	msg := testMessage{ID: 42, Text: "hello"}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded testMessage
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, msg, decoded)
}

func TestReadToken_FileNotExist(t *testing.T) {
	origReadToken := readToken
	defer func() { readToken = origReadToken }()

	readToken = func() (string, error) {
		return "", nil
	}

	token, err := readToken()
	require.NoError(t, err)
	assert.Empty(t, token)
}
