package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zon/chat/core/auth"
	"github.com/zon/chat/core/channel"
	corenats "github.com/zon/chat/core/nats"
	"gorm.io/gorm"
)

type mockNATS struct {
	published []natsMessage
}

type natsMessage struct {
	subject string
	data    any
}

func (m *mockNATS) Publish(subject string, data any) error {
	m.published = append(m.published, natsMessage{subject: subject, data: data})
	return nil
}

func (m *mockNATS) Subscribe(subject string, cb func([]byte)) (*corenats.Subscription, error) {
	return nil, nil
}

func (m *mockNATS) Close() {}

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	t.Skip("skipping test that requires database")
	return nil
}

func createTestUser(t *testing.T, db *gorm.DB, email, subject string, isAdmin, isTest bool) *auth.User {
	t.Helper()
	u := &auth.User{Email: email, Subject: subject, IsAdmin: isAdmin, IsTest: isTest}
	require.NoError(t, db.Create(u).Error)
	return u
}

// fakeAuth returns middleware that injects the given user into the request context.
func fakeAuth(user *auth.User) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if user != nil {
				r = r.WithContext(auth.ContextWithUser(r.Context(), user))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// fakeAuthReject returns middleware that rejects all requests.
func fakeAuthReject() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		})
	}
}

// newTestEngine creates a Gin engine wired to a test DB with the given user authenticated.
func newTestEngine(t *testing.T, db *gorm.DB, user *auth.User) *gin.Engine {
	t.Helper()
	deps := Deps{DB: db, NATS: nil}
	return New(deps, fakeAuth(user))
}

func doJSON(t *testing.T, engine *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody *bytes.Buffer
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewBuffer(data)
	} else {
		reqBody = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func parseJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	return result
}

// --- Health endpoint tests ---

func TestHealth(t *testing.T) {
	db := setupTestDB(t)
	engine := newTestEngine(t, db, nil)

	w := doJSON(t, engine, "GET", "/health", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	body := parseJSON(t, w)
	assert.Equal(t, "ok", body["status"])
}

func TestHealth_NoAuthRequired(t *testing.T) {
	db := setupTestDB(t)
	deps := Deps{DB: db, NATS: nil}
	engine := New(deps, fakeAuthReject())

	w := doJSON(t, engine, "GET", "/health", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

// --- Channel endpoint tests ---

func TestCreateChannel_Success(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{
		"name":      "general",
		"is_public": true,
	})

	assert.Equal(t, http.StatusCreated, w.Code)
	body := parseJSON(t, w)
	assert.Equal(t, "general", body["Name"])
	assert.Equal(t, true, body["IsPublic"])
}

func TestCreateChannel_TestChannel(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{
		"name":    "test-chan",
		"is_test": true,
	})

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCreateChannel_TestChannel_ByTestAdmin(t *testing.T) {
	db := setupTestDB(t)
	testAdmin := createTestUser(t, db, "test-admin@test.com", "sub-test-admin", true, true)
	engine := newTestEngine(t, db, testAdmin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{
		"name":    "test-chan",
		"is_test": true,
	})

	assert.Equal(t, http.StatusCreated, w.Code)
	body := parseJSON(t, w)
	assert.Equal(t, true, body["IsTest"])
}

func TestCreateChannel_NonAdminRejected(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, user)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{
		"name": "general",
	})

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCreateChannel_MissingName(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateChannel_DuplicateName(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	doJSON(t, engine, "POST", "/channels", map[string]any{"name": "dup"})
	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "dup"})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListChannels(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	doJSON(t, engine, "POST", "/channels", map[string]any{"name": "ch1", "is_public": true})
	doJSON(t, engine, "POST", "/channels", map[string]any{"name": "ch2", "is_public": false})

	w := doJSON(t, engine, "GET", "/channels", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var channels []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &channels))
	assert.Len(t, channels, 2)
}

func TestListChannels_Empty(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, user)

	w := doJSON(t, engine, "GET", "/channels", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var channels []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &channels))
	assert.Empty(t, channels)
}

func TestGetChannel_Found(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "findme"})
	created := parseJSON(t, w)

	id := fmt.Sprintf("%.0f", created["ID"])
	w = doJSON(t, engine, "GET", "/channels/"+id, nil)
	assert.Equal(t, http.StatusOK, w.Code)

	body := parseJSON(t, w)
	assert.Equal(t, "findme", body["Name"])
}

func TestGetChannel_NotFound(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, user)

	w := doJSON(t, engine, "GET", "/channels/999", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetChannel_InvalidID(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, user)

	w := doJSON(t, engine, "GET", "/channels/abc", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteChannel_Success(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "deleteme"})
	created := parseJSON(t, w)
	id := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "DELETE", "/channels/"+id, nil)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify it's gone.
	w = doJSON(t, engine, "GET", "/channels/"+id, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteChannel_NotFound(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "DELETE", "/channels/999", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteChannel_NonAdminRejected(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)

	adminEngine := newTestEngine(t, db, admin)
	w := doJSON(t, adminEngine, "POST", "/channels", map[string]any{"name": "nope"})
	created := parseJSON(t, w)
	id := fmt.Sprintf("%.0f", created["ID"])

	userEngine := newTestEngine(t, db, user)
	w = doJSON(t, userEngine, "DELETE", "/channels/"+id, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateChannel_Success(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "old-name", "public": true})
	created := parseJSON(t, w)
	id := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "PATCH", "/channels/"+id, map[string]any{
		"name":        "new-name",
		"description": "A description",
		"public":      false,
	})
	assert.Equal(t, http.StatusOK, w.Code)

	body := parseJSON(t, w)
	assert.Equal(t, "new-name", body["Name"])
	assert.Equal(t, "A description", body["Description"])
	assert.Equal(t, false, body["IsPublic"])
}

func TestUpdateChannel_SetInactive(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "active-chan"})
	created := parseJSON(t, w)
	id := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "PATCH", "/channels/"+id, map[string]any{
		"inactive": true,
	})
	assert.Equal(t, http.StatusOK, w.Code)

	ch, err := channel.Get(db, 1)
	require.NoError(t, err)
	assert.Equal(t, false, ch.IsActive)
}

func TestUpdateChannel_NonAdminRejected(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)

	adminEngine := newTestEngine(t, db, admin)
	w := doJSON(t, adminEngine, "POST", "/channels", map[string]any{"name": "update-test"})
	created := parseJSON(t, w)
	id := fmt.Sprintf("%.0f", created["ID"])

	userEngine := newTestEngine(t, db, user)
	w = doJSON(t, userEngine, "PATCH", "/channels/"+id, map[string]any{
		"name": "should-fail",
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateChannel_NotFound(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "PATCH", "/channels/999", map[string]any{
		"name": "doesnt-exist",
	})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- Member endpoint tests ---

func TestAddMember_Success(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	member := createTestUser(t, db, "member@test.com", "sub-member", false, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "team"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "POST", "/channels/"+channelID+"/members", map[string]any{
		"user_id": member.ID,
	})
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAddMember_TestUserToRealChannelRejected(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	testUser := createTestUser(t, db, "test@test.com", "sub-test", false, true)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "real-chan"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "POST", "/channels/"+channelID+"/members", map[string]any{
		"user_id": testUser.ID,
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAddMember_TestUserToTestChannelAllowed(t *testing.T) {
	db := setupTestDB(t)
	testAdmin := createTestUser(t, db, "test-admin@test.com", "sub-test-admin", true, true)
	testUser := createTestUser(t, db, "test@test.com", "sub-test", false, true)
	engine := newTestEngine(t, db, testAdmin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "test-chan", "is_test": true})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "POST", "/channels/"+channelID+"/members", map[string]any{
		"user_id": testUser.ID,
	})
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAddMember_RealUserToTestChannelRejected(t *testing.T) {
	db := setupTestDB(t)
	testAdmin := createTestUser(t, db, "test-admin@test.com", "sub-test-admin", true, true)
	realUser := createTestUser(t, db, "real@test.com", "sub-real", false, false)
	engine := newTestEngine(t, db, testAdmin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "test-chan", "is_test": true})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "POST", "/channels/"+channelID+"/members", map[string]any{
		"user_id": realUser.ID,
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAddMember_NonAdminRejected(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)

	adminEngine := newTestEngine(t, db, admin)
	w := doJSON(t, adminEngine, "POST", "/channels", map[string]any{"name": "team"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	userEngine := newTestEngine(t, db, user)
	w = doJSON(t, userEngine, "POST", "/channels/"+channelID+"/members", map[string]any{
		"user_id": user.ID,
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAddMember_UserNotFound(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "team"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "POST", "/channels/"+channelID+"/members", map[string]any{
		"user_id": 999,
	})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAddMember_ChannelNotFound(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	member := createTestUser(t, db, "member@test.com", "sub-member", false, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels/999/members", map[string]any{
		"user_id": member.ID,
	})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAddMember_ByEmail_Success(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	member := createTestUser(t, db, "member@test.com", "sub-member", false, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "team"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "POST", "/channels/"+channelID+"/members", map[string]any{
		"email": member.Email,
	})
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAddMember_ByEmail_CreateNewUser(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "team"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "POST", "/channels/"+channelID+"/members", map[string]any{
		"email": "newuser@example.com",
	})
	assert.Equal(t, http.StatusOK, w.Code)

	var newUser auth.User
	result := db.Where("email = ?", "newuser@example.com").First(&newUser)
	require.NoError(t, result.Error)
	assert.NotZero(t, newUser.ID)
}

func TestAddMember_ByEmail_NonAdminRejected(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)

	adminEngine := newTestEngine(t, db, admin)
	w := doJSON(t, adminEngine, "POST", "/channels", map[string]any{"name": "team"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	userEngine := newTestEngine(t, db, user)
	w = doJSON(t, userEngine, "POST", "/channels/"+channelID+"/members", map[string]any{
		"email": "invite@example.com",
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAddMember_NoUserIDOrEmail(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "team"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "POST", "/channels/"+channelID+"/members", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRemoveMember_Success(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	member := createTestUser(t, db, "member@test.com", "sub-member", false, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "team"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	doJSON(t, engine, "POST", "/channels/"+channelID+"/members", map[string]any{
		"user_id": member.ID,
	})

	memberIDStr := fmt.Sprintf("%d", member.ID)
	w = doJSON(t, engine, "DELETE", "/channels/"+channelID+"/members/"+memberIDStr, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRemoveMember_NotFound(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "team"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "DELETE", "/channels/"+channelID+"/members/999", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListMembers(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	m1 := createTestUser(t, db, "m1@test.com", "sub-m1", false, false)
	m2 := createTestUser(t, db, "m2@test.com", "sub-m2", false, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "team"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	doJSON(t, engine, "POST", "/channels/"+channelID+"/members", map[string]any{"user_id": m1.ID})
	doJSON(t, engine, "POST", "/channels/"+channelID+"/members", map[string]any{"user_id": m2.ID})

	w = doJSON(t, engine, "GET", "/channels/"+channelID+"/members", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var members []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &members))
	assert.Len(t, members, 2)
}

func TestListMembers_Empty(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "empty-team"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "GET", "/channels/"+channelID+"/members", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var members []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &members))
	assert.Empty(t, members)
}

func TestListMembers_ChannelNotFound(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, user)

	w := doJSON(t, engine, "GET", "/channels/999/members", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- Message endpoint tests ---

func TestCreateMessage_Success(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "chat"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "POST", "/channels/"+channelID+"/messages", map[string]any{
		"content": "hello world",
	})
	assert.Equal(t, http.StatusCreated, w.Code)

	body := parseJSON(t, w)
	assert.Equal(t, "hello world", body["Content"])
}

func TestCreateMessage_MissingContent(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "chat"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "POST", "/channels/"+channelID+"/messages", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateMessage_InvalidChannelID(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, user)

	w := doJSON(t, engine, "POST", "/channels/abc/messages", map[string]any{
		"content": "hello",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListMessages(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "chat"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	doJSON(t, engine, "POST", "/channels/"+channelID+"/messages", map[string]any{"content": "msg1"})
	doJSON(t, engine, "POST", "/channels/"+channelID+"/messages", map[string]any{"content": "msg2"})

	w = doJSON(t, engine, "GET", "/channels/"+channelID+"/messages", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	body := parseJSON(t, w)
	msgs, ok := body["Messages"].([]any)
	require.True(t, ok)
	assert.Len(t, msgs, 2)
}

func TestListMessages_Empty(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, user)

	w := doJSON(t, engine, "GET", "/channels/1/messages", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	body := parseJSON(t, w)
	msgs, ok := body["Messages"]
	// Messages should be null or empty list when no messages exist.
	if ok && msgs != nil {
		msgList, isList := msgs.([]any)
		if isList {
			assert.Empty(t, msgList)
		}
	}
}

func TestListMessages_WithLimit(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "chat"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	for i := 0; i < 5; i++ {
		doJSON(t, engine, "POST", "/channels/"+channelID+"/messages", map[string]any{
			"content": fmt.Sprintf("msg%d", i),
		})
	}

	w = doJSON(t, engine, "GET", "/channels/"+channelID+"/messages?limit=2", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	body := parseJSON(t, w)
	msgs, ok := body["Messages"].([]any)
	require.True(t, ok)
	assert.Len(t, msgs, 2)
}

func TestListMessages_InvalidLimit(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, user)

	w := doJSON(t, engine, "GET", "/channels/1/messages?limit=abc", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListMessages_InvalidCursor(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, user)

	w := doJSON(t, engine, "GET", "/channels/1/messages?cursor=abc", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListMessages_Pagination(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "chat"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	for i := 0; i < 5; i++ {
		doJSON(t, engine, "POST", "/channels/"+channelID+"/messages", map[string]any{
			"content": fmt.Sprintf("msg%d", i),
		})
	}

	// First page of 3.
	w = doJSON(t, engine, "GET", "/channels/"+channelID+"/messages?limit=3", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	body := parseJSON(t, w)
	msgs := body["Messages"].([]any)
	assert.Len(t, msgs, 3)

	cursor := body["NextCursor"]
	assert.NotNil(t, cursor)
	assert.NotZero(t, cursor)

	// Second page.
	cursorStr := fmt.Sprintf("%.0f", cursor)
	w = doJSON(t, engine, "GET", "/channels/"+channelID+"/messages?limit=3&cursor="+cursorStr, nil)
	assert.Equal(t, http.StatusOK, w.Code)

	body = parseJSON(t, w)
	msgs = body["Messages"].([]any)
	assert.Len(t, msgs, 2)
}

// --- Message edit/delete tests ---

func TestUpdateMessage_Success(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "chat"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	userEngine := newTestEngine(t, db, user)
	w = doJSON(t, userEngine, "POST", "/channels/"+channelID+"/messages", map[string]any{
		"content": "original",
	})
	msgCreated := parseJSON(t, w)
	msgID := fmt.Sprintf("%.0f", msgCreated["ID"])

	w = doJSON(t, userEngine, "PATCH", "/messages/"+msgID, map[string]any{
		"content": "edited",
	})
	assert.Equal(t, http.StatusOK, w.Code)

	body := parseJSON(t, w)
	assert.Equal(t, "edited", body["Content"])
}

func TestUpdateMessage_NotOwner(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	owner := createTestUser(t, db, "owner@test.com", "sub-owner", false, false)
	other := createTestUser(t, db, "other@test.com", "sub-other", false, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "chat"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	ownerEngine := newTestEngine(t, db, owner)
	w = doJSON(t, ownerEngine, "POST", "/channels/"+channelID+"/messages", map[string]any{
		"content": "original",
	})
	msgCreated := parseJSON(t, w)
	msgID := fmt.Sprintf("%.0f", msgCreated["ID"])

	otherEngine := newTestEngine(t, db, other)
	w = doJSON(t, otherEngine, "PATCH", "/messages/"+msgID, map[string]any{
		"content": "hacked",
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateMessage_NotFound(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, user)

	w := doJSON(t, engine, "PATCH", "/messages/999", map[string]any{
		"content": "nope",
	})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteMessage_Owner(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "chat"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	userEngine := newTestEngine(t, db, user)
	w = doJSON(t, userEngine, "POST", "/channels/"+channelID+"/messages", map[string]any{
		"content": "to delete",
	})
	msgCreated := parseJSON(t, w)
	msgID := fmt.Sprintf("%.0f", msgCreated["ID"])

	w = doJSON(t, userEngine, "DELETE", "/messages/"+msgID, nil)
	assert.Equal(t, http.StatusNoContent, w.Code)

	w = doJSON(t, userEngine, "GET", "/channels/"+channelID+"/messages", nil)
	body := parseJSON(t, w)
	msgs := body["Messages"].([]any)
	assert.Len(t, msgs, 0)
}

func TestDeleteMessage_Admin(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	owner := createTestUser(t, db, "owner@test.com", "sub-owner", false, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "chat"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	ownerEngine := newTestEngine(t, db, owner)
	w = doJSON(t, ownerEngine, "POST", "/channels/"+channelID+"/messages", map[string]any{
		"content": "admin delete",
	})
	msgCreated := parseJSON(t, w)
	msgID := fmt.Sprintf("%.0f", msgCreated["ID"])

	w = doJSON(t, engine, "DELETE", "/messages/"+msgID, nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDeleteMessage_NonOwnerNonAdmin(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	owner := createTestUser(t, db, "owner@test.com", "sub-owner", false, false)
	other := createTestUser(t, db, "other@test.com", "sub-other", false, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "chat"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	ownerEngine := newTestEngine(t, db, owner)
	w = doJSON(t, ownerEngine, "POST", "/channels/"+channelID+"/messages", map[string]any{
		"content": "not yours",
	})
	msgCreated := parseJSON(t, w)
	msgID := fmt.Sprintf("%.0f", msgCreated["ID"])

	otherEngine := newTestEngine(t, db, other)
	w = doJSON(t, otherEngine, "DELETE", "/messages/"+msgID, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDeleteMessage_NotFound(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, user)

	w := doJSON(t, engine, "DELETE", "/messages/999", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- Auth middleware integration tests ---

func TestAuthMiddleware_Rejected(t *testing.T) {
	db := setupTestDB(t)
	deps := Deps{DB: db, NATS: nil}
	engine := New(deps, fakeAuthReject())

	w := doJSON(t, engine, "GET", "/channels", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_NoUserInContext(t *testing.T) {
	db := setupTestDB(t)
	// Auth middleware that passes through without setting a user.
	passthrough := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
	deps := Deps{DB: db, NATS: nil}
	engine := New(deps, passthrough)

	w := doJSON(t, engine, "GET", "/channels", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- Test flag related tests ---

func TestCreateChannel_TestFlag(t *testing.T) {
	// Verify that a test admin can create test channels (used when --test flag is enabled).
	db := setupTestDB(t)
	testAdmin := createTestUser(t, db, "test-admin@test.com", "sub-test-admin", true, true)
	engine := newTestEngine(t, db, testAdmin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{
		"name":    "test-channel",
		"is_test": true,
	})
	assert.Equal(t, http.StatusCreated, w.Code)

	body := parseJSON(t, w)
	assert.Equal(t, true, body["IsTest"])
}

func TestTestUsersCanAccessAPI(t *testing.T) {
	// Test users (authenticated via client credentials in test mode) can use the API.
	db := setupTestDB(t)
	testUser := createTestUser(t, db, "test@test.com", "sub-test", false, true)
	engine := newTestEngine(t, db, testUser)

	// Test user can list channels (not an admin-only action).
	w := doJSON(t, engine, "GET", "/channels", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTestUserCannotCreateChannel(t *testing.T) {
	// Non-admin test users cannot create channels (admin required).
	db := setupTestDB(t)
	testUser := createTestUser(t, db, "test@test.com", "sub-test", false, true)
	engine := newTestEngine(t, db, testUser)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "nope"})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminTestUser(t *testing.T) {
	// Admin test users can manage everything.
	db := setupTestDB(t)
	adminTest := createTestUser(t, db, "test-admin@example.com", "sub-admin-test", true, true)
	engine := newTestEngine(t, db, adminTest)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{
		"name":    "admin-test-chan",
		"is_test": true,
	})
	assert.Equal(t, http.StatusCreated, w.Code)
}

// --- Gin framework verification ---

func TestUsesGinFramework(t *testing.T) {
	// Verifies that the REST service uses Gin by checking the engine type.
	db := setupTestDB(t)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	deps := Deps{DB: db, NATS: nil}
	engine := New(deps, fakeAuth(user))

	// The engine is a *gin.Engine which implements http.Handler.
	var _ http.Handler = engine
	assert.NotNil(t, engine)
}

// --- JSON response format ---

func TestResponsesAreJSON(t *testing.T) {
	db := setupTestDB(t)
	engine := newTestEngine(t, db, nil)

	w := doJSON(t, engine, "GET", "/health", nil)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestChannelResponseIsJSON(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "json-test"})
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestMessageResponseIsJSON(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "json-msg"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "POST", "/channels/"+channelID+"/messages", map[string]any{"content": "test"})
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

// --- NATS publishing tests ---

func newTestEngineWithNATS(t *testing.T, db *gorm.DB, user *auth.User, nats *mockNATS) *gin.Engine {
	t.Helper()
	deps := Deps{DB: db, NATS: nats}
	return New(deps, fakeAuth(user))
}

func TestNATS_PublishesOnChannelCreated(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	mn := &mockNATS{}
	engine := newTestEngineWithNATS(t, db, admin, mn)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{
		"name":      "nats-test-chan",
		"is_public": true,
	})

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Len(t, mn.published, 1)
	assert.Contains(t, mn.published[0].subject, "wurbs.channel.")
	assert.Contains(t, mn.published[0].subject, ".created")
}

func TestNATS_PublishesOnChannelDeleted(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	mn := &mockNATS{}
	engine := newTestEngineWithNATS(t, db, admin, mn)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "delete-nats"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "DELETE", "/channels/"+channelID, nil)
	assert.Equal(t, http.StatusOK, w.Code)

	assert.Len(t, mn.published, 2)
	assert.Contains(t, mn.published[1].subject, "wurbs.channel.")
	assert.Contains(t, mn.published[1].subject, ".deleted")
}

func TestNATS_PublishesOnMemberAdded(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	member := createTestUser(t, db, "member@test.com", "sub-member", false, false)
	mn := &mockNATS{}
	engine := newTestEngineWithNATS(t, db, admin, mn)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "member-test"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "POST", "/channels/"+channelID+"/members", map[string]any{
		"user_id": member.ID,
	})
	assert.Equal(t, http.StatusOK, w.Code)

	assert.Len(t, mn.published, 2)
	assert.Contains(t, mn.published[1].subject, "wurbs.channel.")
	assert.Contains(t, mn.published[1].subject, ".members.added")
}

func TestNATS_PublishesOnMemberRemoved(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	member := createTestUser(t, db, "member@test.com", "sub-member", false, false)
	mn := &mockNATS{}
	engine := newTestEngineWithNATS(t, db, admin, mn)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "remove-test"})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	doJSON(t, engine, "POST", "/channels/"+channelID+"/members", map[string]any{
		"user_id": member.ID,
	})

	memberIDStr := fmt.Sprintf("%d", member.ID)
	w = doJSON(t, engine, "DELETE", "/channels/"+channelID+"/members/"+memberIDStr, nil)
	assert.Equal(t, http.StatusOK, w.Code)

	assert.Len(t, mn.published, 3)
	assert.Contains(t, mn.published[2].subject, "wurbs.channel.")
	assert.Contains(t, mn.published[2].subject, ".members.removed")
}

func TestNATS_SkippedWhenNil(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{
		"name": "no-nats",
	})

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestUpdateUser_EmailCannotBeUpdated(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, user)

	email := "new-email@test.com"
	w := doJSON(t, engine, "PATCH", fmt.Sprintf("/users/%d", user.ID), map[string]any{
		"email": email,
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetUser_EmailNotIncluded(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, user)

	w := doJSON(t, engine, "GET", fmt.Sprintf("/users/%d", user.ID), nil)
	assert.Equal(t, http.StatusOK, w.Code)

	body := parseJSON(t, w)
	_, ok := body["email"]
	assert.False(t, ok)
}

func TestGetUser_Success(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "GET", fmt.Sprintf("/users/%d", user.ID), nil)
	assert.Equal(t, http.StatusOK, w.Code)

	body := parseJSON(t, w)
	assert.Equal(t, fmt.Sprintf("%d", user.ID), body["id"])
	assert.Equal(t, false, body["admin"])
	assert.Equal(t, false, body["inactive"])
}

func TestGetUser_NotFound(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "GET", "/users/999", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetUser_NoAuth(t *testing.T) {
	db := setupTestDB(t)
	deps := Deps{DB: db, NATS: nil}
	engine := New(deps, fakeAuthReject())

	w := doJSON(t, engine, "GET", "/users/1", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUpdateUser_SelfUpdate(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, user)

	w := doJSON(t, engine, "PATCH", fmt.Sprintf("/users/%d", user.ID), map[string]any{
		"username": "newname",
	})
	assert.Equal(t, http.StatusOK, w.Code)

	body := parseJSON(t, w)
	assert.Equal(t, "newname", body["username"])
}

func TestUpdateUser_OtherUserRejected(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	otherUser := createTestUser(t, db, "other@test.com", "sub-other", false, false)
	engine := newTestEngine(t, db, user)

	w := doJSON(t, engine, "PATCH", fmt.Sprintf("/users/%d", otherUser.ID), map[string]any{
		"username": "hacked",
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateUser_AdminCanUpdateOther(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "PATCH", fmt.Sprintf("/users/%d", user.ID), map[string]any{
		"username": "adminupdated",
	})
	assert.Equal(t, http.StatusOK, w.Code)

	body := parseJSON(t, w)
	assert.Equal(t, "adminupdated", body["username"])
}

func TestUpdateUser_AdminCanSetAdmin(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "PATCH", fmt.Sprintf("/users/%d", user.ID), map[string]any{
		"admin": true,
	})
	assert.Equal(t, http.StatusOK, w.Code)

	body := parseJSON(t, w)
	assert.Equal(t, true, body["admin"])
}

func TestUpdateUser_NonAdminCannotSetAdmin(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, user)

	w := doJSON(t, engine, "PATCH", fmt.Sprintf("/users/%d", user.ID), map[string]any{
		"admin": true,
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateUser_AdminCanSetInactive(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "PATCH", fmt.Sprintf("/users/%d", user.ID), map[string]any{
		"inactive": true,
	})
	assert.Equal(t, http.StatusOK, w.Code)

	body := parseJSON(t, w)
	assert.Equal(t, true, body["inactive"])
}

func TestUpdateUser_NonAdminCannotSetInactive(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "user@test.com", "sub-user", false, false)
	engine := newTestEngine(t, db, user)

	w := doJSON(t, engine, "PATCH", fmt.Sprintf("/users/%d", user.ID), map[string]any{
		"inactive": true,
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateUser_NotFound(t *testing.T) {
	db := setupTestDB(t)
	admin := createTestUser(t, db, "admin@test.com", "sub-admin", true, false)
	engine := newTestEngine(t, db, admin)

	w := doJSON(t, engine, "PATCH", "/users/999", map[string]any{
		"username": "nonexistent",
	})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreateChannel_RealAdminCannotCreateTestChannel(t *testing.T) {
	db := setupTestDB(t)
	realAdmin := createTestUser(t, db, "real-admin@test.com", "sub-real-admin", true, false)
	engine := newTestEngine(t, db, realAdmin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{
		"name":    "test-chan",
		"is_test": true,
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCreateChannel_TestAdminCannotCreateRealChannel(t *testing.T) {
	db := setupTestDB(t)
	testAdmin := createTestUser(t, db, "test-admin@test.com", "sub-test-admin", true, true)
	engine := newTestEngine(t, db, testAdmin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{
		"name":    "real-chan",
		"is_test": false,
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateChannel_RealAdminCannotUpdateTestChannel(t *testing.T) {
	db := setupTestDB(t)
	realAdmin := createTestUser(t, db, "real-admin@test.com", "sub-real-admin", true, false)
	testAdmin := createTestUser(t, db, "test-admin@test.com", "sub-test-admin", true, true)
	realEngine := newTestEngine(t, db, realAdmin)
	testEngine := newTestEngine(t, db, testAdmin)

	w := doJSON(t, testEngine, "POST", "/channels", map[string]any{"name": "test-chan", "is_test": true})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, realEngine, "PATCH", "/channels/"+channelID, map[string]any{
		"name": "should-fail",
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateChannel_TestAdminCannotUpdateRealChannel(t *testing.T) {
	db := setupTestDB(t)
	realAdmin := createTestUser(t, db, "real-admin@test.com", "sub-real-admin", true, false)
	testAdmin := createTestUser(t, db, "test-admin@test.com", "sub-test-admin", true, true)
	realEngine := newTestEngine(t, db, realAdmin)
	testEngine := newTestEngine(t, db, testAdmin)

	w := doJSON(t, realEngine, "POST", "/channels", map[string]any{"name": "real-chan", "is_test": false})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, testEngine, "PATCH", "/channels/"+channelID, map[string]any{
		"name": "should-fail",
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDeleteChannel_RealAdminCannotDeleteTestChannel(t *testing.T) {
	db := setupTestDB(t)
	realAdmin := createTestUser(t, db, "real-admin@test.com", "sub-real-admin", true, false)
	testAdmin := createTestUser(t, db, "test-admin@test.com", "sub-test-admin", true, true)
	realEngine := newTestEngine(t, db, realAdmin)
	testEngine := newTestEngine(t, db, testAdmin)

	w := doJSON(t, testEngine, "POST", "/channels", map[string]any{"name": "test-chan", "is_test": true})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, realEngine, "DELETE", "/channels/"+channelID, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDeleteChannel_TestAdminCannotDeleteRealChannel(t *testing.T) {
	db := setupTestDB(t)
	realAdmin := createTestUser(t, db, "real-admin@test.com", "sub-real-admin", true, false)
	testAdmin := createTestUser(t, db, "test-admin@test.com", "sub-test-admin", true, true)
	realEngine := newTestEngine(t, db, realAdmin)
	testEngine := newTestEngine(t, db, testAdmin)

	w := doJSON(t, realEngine, "POST", "/channels", map[string]any{"name": "real-chan", "is_test": false})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, testEngine, "DELETE", "/channels/"+channelID, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAddMember_RealAdminCannotAddTestUserToRealChannel(t *testing.T) {
	db := setupTestDB(t)
	realAdmin := createTestUser(t, db, "real-admin@test.com", "sub-real-admin", true, false)
	testUser := createTestUser(t, db, "test@test.com", "sub-test", false, true)
	engine := newTestEngine(t, db, realAdmin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "real-chan", "is_test": false})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "POST", "/channels/"+channelID+"/members", map[string]any{
		"user_id": testUser.ID,
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAddMember_TestAdminCannotAddRealUserToTestChannel(t *testing.T) {
	db := setupTestDB(t)
	testAdmin := createTestUser(t, db, "test-admin@test.com", "sub-test-admin", true, true)
	realUser := createTestUser(t, db, "real@test.com", "sub-real", false, false)
	engine := newTestEngine(t, db, testAdmin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "test-chan", "is_test": true})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "POST", "/channels/"+channelID+"/members", map[string]any{
		"user_id": realUser.ID,
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRemoveMember_RealAdminCannotRemoveTestUserFromRealChannel(t *testing.T) {
	db := setupTestDB(t)
	realAdmin := createTestUser(t, db, "real-admin@test.com", "sub-real-admin", true, false)
	testUser := createTestUser(t, db, "test@test.com", "sub-test", false, true)
	engine := newTestEngine(t, db, realAdmin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "real-chan", "is_test": false})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "POST", "/channels/"+channelID+"/members", map[string]any{
		"user_id": testUser.ID,
	})
	if w.Code != http.StatusForbidden {
		testUserIDStr := fmt.Sprintf("%d", testUser.ID)
		w = doJSON(t, engine, "DELETE", "/channels/"+channelID+"/members/"+testUserIDStr, nil)
		assert.Equal(t, http.StatusForbidden, w.Code)
	}
}

func TestRemoveMember_TestAdminCannotRemoveRealUserFromTestChannel(t *testing.T) {
	db := setupTestDB(t)
	testAdmin := createTestUser(t, db, "test-admin@test.com", "sub-test-admin", true, true)
	realUser := createTestUser(t, db, "real@test.com", "sub-real", false, false)
	engine := newTestEngine(t, db, testAdmin)

	w := doJSON(t, engine, "POST", "/channels", map[string]any{"name": "test-chan", "is_test": true})
	created := parseJSON(t, w)
	channelID := fmt.Sprintf("%.0f", created["ID"])

	w = doJSON(t, engine, "POST", "/channels/"+channelID+"/members", map[string]any{
		"user_id": realUser.ID,
	})
	if w.Code != http.StatusForbidden {
		realUserIDStr := fmt.Sprintf("%d", realUser.ID)
		w = doJSON(t, engine, "DELETE", "/channels/"+channelID+"/members/"+realUserIDStr, nil)
		assert.Equal(t, http.StatusForbidden, w.Code)
	}
}

func TestUpdateUser_RealAdminCannotModifyTestUser(t *testing.T) {
	db := setupTestDB(t)
	realAdmin := createTestUser(t, db, "real-admin@test.com", "sub-real-admin", true, false)
	testUser := createTestUser(t, db, "test@test.com", "sub-test", false, true)
	engine := newTestEngine(t, db, realAdmin)

	w := doJSON(t, engine, "PATCH", fmt.Sprintf("/users/%d", testUser.ID), map[string]any{
		"username": "should-fail",
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateUser_TestAdminCannotModifyRealUser(t *testing.T) {
	db := setupTestDB(t)
	testAdmin := createTestUser(t, db, "test-admin@test.com", "sub-test-admin", true, true)
	realUser := createTestUser(t, db, "real@test.com", "sub-real", false, false)
	engine := newTestEngine(t, db, testAdmin)

	w := doJSON(t, engine, "PATCH", fmt.Sprintf("/users/%d", realUser.ID), map[string]any{
		"username": "should-fail",
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}
