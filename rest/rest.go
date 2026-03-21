package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zon/chat/core/auth"
	"github.com/zon/chat/core/channel"
	"github.com/zon/chat/core/message"
	"gorm.io/gorm"
)

// NATSPublisher defines the interface for publishing to NATS.
type NATSPublisher interface {
	Publish(subject string, data any) error
}

// Deps holds the dependencies for the REST service.
type Deps struct {
	DB   *gorm.DB
	NATS NATSPublisher // may be nil; NATS publishing is skipped when nil
}

// New creates a Gin engine with all REST API routes registered.
// The auth middleware parameter wraps standard net/http middleware that
// sets the authenticated user in the request context.
func New(deps Deps, authMiddleware func(http.Handler) http.Handler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", health)

	authHandler := &authHandler{deps: deps}
	r.GET("/auth/login", authHandler.login)
	r.GET("/auth/callback", authHandler.callback)
	r.POST("/auth/logout", authHandler.logout)
	r.POST("/auth/refresh", authHandler.refresh)
	r.POST("/auth/token", authHandler.token)

	api := r.Group("")
	api.Use(wrapMiddleware(authMiddleware))

	h := &handler{deps: deps}

	api.POST("/channels", h.createChannel)
	api.GET("/channels", h.listChannels)
	api.GET("/channels/:id", h.getChannel)
	api.PATCH("/channels/:id", h.updateChannel)
	api.DELETE("/channels/:id", h.deleteChannel)

	api.POST("/channels/:id/members", h.addMember)
	api.DELETE("/channels/:id/members/:user_id", h.removeMember)
	api.GET("/channels/:id/members", h.listMembers)

	api.POST("/channels/:id/messages", h.createMessage)
	api.GET("/channels/:id/messages", h.listMessages)

	api.PATCH("/messages/:id", h.updateMessage)
	api.DELETE("/messages/:id", h.deleteMessage)

	api.GET("/users/:id", h.getUser)
	api.PATCH("/users/:id", h.updateUser)

	return r
}

// health is the unauthenticated health check endpoint.
func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// wrapMiddleware adapts a standard net/http middleware to a Gin middleware.
func wrapMiddleware(mw func(http.Handler) http.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		// The net/http middleware calls next.ServeHTTP on success.
		// We wrap the Gin context so the middleware can set context values.
		var called bool
		wrapped := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Replace the Gin request with the one from the middleware
			// (which may have an updated context with the authenticated user).
			c.Request = r
			called = true
		}))
		wrapped.ServeHTTP(c.Writer, c.Request)
		if !called {
			c.Abort()
			return
		}
		c.Next()
	}
}

// handler holds route handler methods and shared dependencies.
type handler struct {
	deps Deps
}

// parseID parses a uint path parameter. Returns 0 and sends a 400 response on failure.
func parseID(c *gin.Context, param string) (uint, bool) {
	raw := c.Param(param)
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + param})
		return 0, false
	}
	return uint(id), true
}

// currentUser extracts the authenticated user from the Gin request context.
func currentUser(c *gin.Context) (*auth.User, bool) {
	u, err := auth.UserFromContext(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	return u, true
}

// --- Channel handlers ---

type createChannelRequest struct {
	Name     string `json:"name" binding:"required"`
	IsPublic bool   `json:"is_public"`
	IsTest   bool   `json:"is_test"`
}

func (h *handler) createChannel(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}
	if !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin required"})
		return
	}

	var req createChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ch, err := channel.Create(h.deps.DB, req.Name, req.IsPublic, req.IsTest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Publish channel creation to NATS.
	if h.deps.NATS != nil {
		_ = h.deps.NATS.Publish(fmt.Sprintf("wurbs.channel.%d.created", ch.ID), ch)
	}

	c.JSON(http.StatusCreated, ch)
}

func (h *handler) listChannels(c *gin.Context) {
	if _, ok := currentUser(c); !ok {
		return
	}

	channels, err := channel.List(h.deps.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, channels)
}

func (h *handler) getChannel(c *gin.Context) {
	if _, ok := currentUser(c); !ok {
		return
	}

	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	ch, err := channel.Get(h.deps.DB, id)
	if err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ch)
}

type updateChannelRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsPublic    *bool   `json:"public"`
	IsActive    *bool   `json:"inactive"`
}

func (h *handler) updateChannel(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}
	if !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin required"})
		return
	}

	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	ch, err := channel.Get(h.deps.DB, id)
	if err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var req updateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := channel.UpdateInput{
		Name:        req.Name,
		Description: req.Description,
		IsPublic:    req.IsPublic,
	}
	if req.IsActive != nil {
		isActive := !*req.IsActive
		input.IsActive = &isActive
	}

	if err := channel.Update(h.deps.DB, ch, input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	updated, _ := channel.Get(h.deps.DB, id)
	c.JSON(http.StatusOK, updated)
}

func (h *handler) deleteChannel(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}
	if !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin required"})
		return
	}

	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	err := channel.Delete(h.deps.DB, id)
	if err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Publish channel deletion to NATS.
	if h.deps.NATS != nil {
		_ = h.deps.NATS.Publish(fmt.Sprintf("wurbs.channel.%d.deleted", id), gin.H{"id": id})
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// --- Member handlers ---

type addMemberRequest struct {
	UserID *uint  `json:"user_id"`
	Email  string `json:"email"`
}

func (h *handler) addMember(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}
	if !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin required"})
		return
	}

	channelID, ok := parseID(c, "id")
	if !ok {
		return
	}

	var req addMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID == nil && req.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id or email required"})
		return
	}

	var target *auth.User
	var err error

	if req.UserID != nil {
		target, err = auth.GetUserByID(h.deps.DB, fmt.Sprintf("%d", *req.UserID))
		if err != nil {
			if errors.Is(err, auth.ErrUserNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		target, err = auth.FindOrCreateUserByEmail(h.deps.DB, req.Email, "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	err = channel.AddMember(h.deps.DB, channelID, target)
	if err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
			return
		}
		if errors.Is(err, channel.ErrTestUserInReal) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Publish membership change to NATS.
	if h.deps.NATS != nil {
		_ = h.deps.NATS.Publish(fmt.Sprintf("wurbs.channel.%d.members.added", channelID), gin.H{
			"channel_id": channelID,
			"user_id":    target.ID,
		})
	}

	c.JSON(http.StatusOK, gin.H{"added": true})
}

func (h *handler) removeMember(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}
	if !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin required"})
		return
	}

	channelID, ok := parseID(c, "id")
	if !ok {
		return
	}

	userID, ok := parseID(c, "user_id")
	if !ok {
		return
	}

	err := channel.RemoveMember(h.deps.DB, channelID, userID)
	if err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Publish membership change to NATS.
	if h.deps.NATS != nil {
		_ = h.deps.NATS.Publish(fmt.Sprintf("wurbs.channel.%d.members.removed", channelID), gin.H{
			"channel_id": channelID,
			"user_id":    userID,
		})
	}

	c.JSON(http.StatusOK, gin.H{"removed": true})
}

func (h *handler) listMembers(c *gin.Context) {
	if _, ok := currentUser(c); !ok {
		return
	}

	channelID, ok := parseID(c, "id")
	if !ok {
		return
	}

	members, err := channel.Members(h.deps.DB, channelID)
	if err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, members)
}

// --- Message handlers ---

type createMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

func (h *handler) createMessage(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}

	channelID, ok := parseID(c, "id")
	if !ok {
		return
	}

	var req createMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// message.Create publishes to NATS internally when nc is non-nil.
	var nc interface {
		Publish(subject string, data any) error
	} = h.deps.NATS
	msg, err := message.Create(h.deps.DB, nc, channelID, user.ID, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, msg)
}

func (h *handler) listMessages(c *gin.Context) {
	if _, ok := currentUser(c); !ok {
		return
	}

	channelID, ok := parseID(c, "id")
	if !ok {
		return
	}

	var cursor uint
	if raw := c.Query("cursor"); raw != "" {
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cursor"})
			return
		}
		cursor = uint(v)
	}

	var limit int
	if raw := c.Query("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = v
	}

	var before, after *time.Time
	if raw := c.Query("before"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid before timestamp"})
			return
		}
		before = &t
	}
	if raw := c.Query("after"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid after timestamp"})
			return
		}
		after = &t
	}

	page, err := message.List(h.deps.DB, channelID, cursor, limit, before, after)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, page)
}

func (h *handler) updateMessage(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}

	messageID, ok := parseID(c, "id")
	if !ok {
		return
	}

	msg, err := message.Get(h.deps.DB, messageID)
	if err != nil {
		if errors.Is(err, message.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "message not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if msg.UserID != user.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the owner can edit this message"})
		return
	}

	var req createMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := message.Update(h.deps.DB, messageID, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updated)
}

func (h *handler) deleteMessage(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		return
	}

	messageID, ok := parseID(c, "id")
	if !ok {
		return
	}

	msg, err := message.Get(h.deps.DB, messageID)
	if err != nil {
		if errors.Is(err, message.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "message not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if msg.UserID != user.ID && !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "owner or admin required"})
		return
	}

	if err := message.Delete(h.deps.DB, messageID); err != nil {
		if errors.Is(err, message.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "message not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

type userResponse struct {
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	Username  *string `json:"username"`
	Admin     bool    `json:"admin"`
	Inactive  bool    `json:"inactive"`
	CreatedAt string  `json:"createdAt"`
}

func userToResponse(u *auth.User) userResponse {
	return userResponse{
		ID:        fmt.Sprintf("%d", u.ID),
		Email:     u.Email,
		Username:  u.Username,
		Admin:     u.IsAdmin,
		Inactive:  !u.IsActive,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (h *handler) getUser(c *gin.Context) {
	if _, ok := currentUser(c); !ok {
		return
	}

	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	user, err := auth.GetUserByID(h.deps.DB, userID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, userToResponse(user))
}

type updateUserRequest struct {
	Username *string `json:"username"`
	Email    *string `json:"email"`
	Admin    *bool   `json:"admin"`
	Inactive *bool   `json:"inactive"`
}

func (h *handler) updateUser(c *gin.Context) {
	currentUser, ok := currentUser(c)
	if !ok {
		return
	}

	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	targetUser, err := auth.GetUserByID(h.deps.DB, userID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	isSelf := currentUser.ID == targetUser.ID
	if !isSelf && !currentUser.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot edit other users"})
		return
	}

	input := auth.UpdateUserInput{
		Username: req.Username,
		Email:    req.Email,
	}

	if currentUser.IsAdmin {
		input.Admin = req.Admin
		input.Inactive = req.Inactive
	} else if req.Admin != nil || req.Inactive != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin field requires admin privileges"})
		return
	}

	if err := auth.UpdateUser(h.deps.DB, targetUser, input, currentUser.IsAdmin); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	updatedUser, _ := auth.GetUserByID(h.deps.DB, userID)
	c.JSON(http.StatusOK, userToResponse(updatedUser))
}

type authHandler struct {
	deps Deps
}

func (h *authHandler) login(c *gin.Context) {
	auth.Login(c.Writer, c.Request)
}

func (h *authHandler) callback(c *gin.Context) {
	auth.Callback(h.deps.DB)(c.Writer, c.Request)
}

func (h *authHandler) logout(c *gin.Context) {
	auth.Logout(c.Writer, c.Request)
}

func (h *authHandler) refresh(c *gin.Context) {
	auth.Refresh(c.Writer, c.Request)
}

func (h *authHandler) token(c *gin.Context) {
	auth.ClientToken(h.deps.DB)(c.Writer, c.Request)
}
