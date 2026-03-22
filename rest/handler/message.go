package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zon/chat/core/message"
)

type MessageHandler struct {
	deps Deps
}

func NewMessageHandler(deps Deps) *MessageHandler {
	return &MessageHandler{deps: deps}
}

type createMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

func (h *MessageHandler) CreateMessage(c *gin.Context) {
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

func (h *MessageHandler) ListMessages(c *gin.Context) {
	if _, ok := currentUser(c); !ok {
		return
	}

	channelID, ok := parseID(c, "id")
	if !ok {
		return
	}

	var cursor uint
	if raw := c.Query("cursor"); raw != "" {
		v, err := parseCursor(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cursor"})
			return
		}
		cursor = v
	}

	var limit int
	if raw := c.Query("limit"); raw != "" {
		v, err := parseLimit(raw)
		if err != nil {
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

func parseCursor(raw string) (uint, error) {
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(v), nil
}

func parseLimit(raw string) (int, error) {
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 0, errors.New("invalid limit")
	}
	return v, nil
}

func (h *MessageHandler) UpdateMessage(c *gin.Context) {
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

	var nc interface {
		Publish(subject string, data any) error
	} = h.deps.NATS
	updated, err := message.Update(h.deps.DB, nc, messageID, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updated)
}

func (h *MessageHandler) DeleteMessage(c *gin.Context) {
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

	var nc interface {
		Publish(subject string, data any) error
	} = h.deps.NATS
	if err := message.Delete(h.deps.DB, nc, messageID); err != nil {
		if errors.Is(err, message.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "message not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
