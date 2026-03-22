package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	baseURL = "http://localhost:8080"
)

var (
	testAccessToken string
	testChannelID   string
	testUserID      string
	testMessageID   string
)

func getAuthToken(t *testing.T) string {
	if testAccessToken != "" {
		return testAccessToken
	}

	t.Skip("E2E tests require a running REST server with database. Start with: make rest")
	return ""
}

func getAuthHeaders(t *testing.T) map[string]string {
	token := getAuthToken(t)
	if token == "" {
		return nil
	}
	return map[string]string{
		"Authorization": "Bearer " + token,
	}
}

func createChannel(t *testing.T, name string, isPublic, isTest bool) map[string]interface{} {
	headers := getAuthHeaders(t)
	require.NotNil(t, headers, "need auth token")

	body := map[string]interface{}{
		"name":      name,
		"is_public": isPublic,
		"is_test":   isTest,
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", baseURL+"/channels", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode, "create channel should return 201")

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}

func TestHealth(t *testing.T) {
	resp, err := http.Get(baseURL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCreateChannel(t *testing.T) {
	ch := createChannel(t, "test-channel-e2e", true, true)
	testChannelID = ch["id"].(string)
	assert.NotEmpty(t, testChannelID)
}

func TestGetChannel(t *testing.T) {
	if testChannelID == "" {
		t.Skip("channel not created yet")
	}

	headers := getAuthHeaders(t)
	require.NotNil(t, headers)

	req, err := http.NewRequest("GET", baseURL+"/channels/"+testChannelID, nil)
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var ch map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&ch)
	assert.Equal(t, "test-channel-e2e", ch["name"])
}

func TestUpdateChannel(t *testing.T) {
	if testChannelID == "" {
		t.Skip("channel not created yet")
	}

	headers := getAuthHeaders(t)
	require.NotNil(t, headers)

	body := map[string]interface{}{
		"name": "updated-channel-name",
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequest("PATCH", baseURL+"/channels/"+testChannelID, bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestListChannels(t *testing.T) {
	headers := getAuthHeaders(t)
	require.NotNil(t, headers)

	req, err := http.NewRequest("GET", baseURL+"/channels", nil)
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var channels []interface{}
	json.NewDecoder(resp.Body).Decode(&channels)
	assert.NotEmpty(t, channels)
}

func TestAddMember(t *testing.T) {
	if testChannelID == "" {
		t.Skip("channel not created yet")
	}

	headers := getAuthHeaders(t)
	require.NotNil(t, headers)

	body := map[string]interface{}{
		"email": "newmember@example.com",
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", baseURL+"/channels/"+testChannelID+"/members", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestListMembers(t *testing.T) {
	if testChannelID == "" {
		t.Skip("channel not created yet")
	}

	headers := getAuthHeaders(t)
	require.NotNil(t, headers)

	req, err := http.NewRequest("GET", baseURL+"/channels/"+testChannelID+"/members", nil)
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCreateMessage(t *testing.T) {
	if testChannelID == "" {
		t.Skip("channel not created yet")
	}

	headers := getAuthHeaders(t)
	require.NotNil(t, headers)

	body := map[string]interface{}{
		"body": "Hello from E2E test",
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", baseURL+"/channels/"+testChannelID+"/messages", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var msg map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&msg)
	testMessageID = msg["id"].(string)
	assert.NotEmpty(t, testMessageID)
}

func TestListMessages(t *testing.T) {
	if testChannelID == "" {
		t.Skip("channel not created yet")
	}

	headers := getAuthHeaders(t)
	require.NotNil(t, headers)

	req, err := http.NewRequest("GET", baseURL+"/channels/"+testChannelID+"/messages", nil)
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var page map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&page)
	assert.Contains(t, page, "items")
}

func TestUpdateMessage(t *testing.T) {
	if testMessageID == "" {
		t.Skip("message not created yet")
	}

	headers := getAuthHeaders(t)
	require.NotNil(t, headers)

	body := map[string]interface{}{
		"body": "Updated message body",
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequest("PATCH", baseURL+"/messages/"+testMessageID, bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDeleteMessage(t *testing.T) {
	if testMessageID == "" {
		t.Skip("message not created yet")
	}

	headers := getAuthHeaders(t)
	require.NotNil(t, headers)

	req, err := http.NewRequest("DELETE", baseURL+"/messages/"+testMessageID, nil)
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestGetUser(t *testing.T) {
	headers := getAuthHeaders(t)
	require.NotNil(t, headers)

	userID := "test-user-id"
	req, err := http.NewRequest("GET", baseURL+"/users/"+userID, nil)
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Skip("user not found - test user needs to be created first")
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestUpdateUser(t *testing.T) {
	headers := getAuthHeaders(t)
	require.NotNil(t, headers)

	body := map[string]interface{}{
		"username": "updateduser",
	}
	bodyBytes, _ := json.Marshal(body)

	userID := "test-user-id"
	req, err := http.NewRequest("PATCH", baseURL+"/users/"+userID, bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Skip("user not found - test user needs to be created first")
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRemoveMember(t *testing.T) {
	if testChannelID == "" {
		t.Skip("channel not created yet")
	}

	headers := getAuthHeaders(t)
	require.NotNil(t, headers)

	userID := "some-user-id"
	req, err := http.NewRequest("DELETE", baseURL+"/channels/"+testChannelID+"/members/"+userID, nil)
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Skip("member not found")
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDeleteChannel(t *testing.T) {
	ch := createChannel(t, "channel-to-delete", true, true)
	deleteChannelID := ch["id"].(string)

	headers := getAuthHeaders(t)
	require.NotNil(t, headers)

	req, err := http.NewRequest("DELETE", baseURL+"/channels/"+deleteChannelID, nil)
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func init() {
	if os.Getenv("E2E_TEST_TOKEN") != "" {
		testAccessToken = os.Getenv("E2E_TEST_TOKEN")
	}
}
