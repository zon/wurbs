package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zon/chat/core/message"
	"gorm.io/gorm"
)

type Message struct {
	DB   *gorm.DB
	NATS message.Publisher
}

func NewMessage(db *gorm.DB, nats message.Publisher) *Message {
	return &Message{DB: db, NATS: nats}
}

type createMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

func (h *Message) CreateMessage(c *gin.Context) {
	user, err := currentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	channelID, err := parseID(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req createMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	msg, err := message.Create(h.DB, h.NATS, channelID, user.ID, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, msg)
}

func (h *Message) ListMessages(c *gin.Context) {
	if _, err := currentUser(c); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	channelID, err := parseID(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var cursor uint
	if raw := c.Query("cursor"); raw != "" {
		v, err := parseCursor(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "message: invalid cursor"})
			return
		}
		cursor = v
	}

	var limit int
	if raw := c.Query("limit"); raw != "" {
		v, err := parseLimit(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "message: invalid limit"})
			return
		}
		limit = v
	}

	var before, after *time.Time
	if raw := c.Query("before"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "message: invalid before timestamp"})
			return
		}
		before = &t
	}
	if raw := c.Query("after"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "message: invalid after timestamp"})
			return
		}
		after = &t
	}

	page, err := message.List(h.DB, channelID, cursor, limit, before, after)
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
		return 0, errors.New("message: invalid limit")
	}
	return v, nil
}

func (h *Message) UpdateMessage(c *gin.Context) {
	user, err := currentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	messageID, err := parseID(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	msg, err := message.Get(h.DB, messageID)
	if err != nil {
		if errors.Is(err, message.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "message: message not found"})
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

	updated, err := message.Update(h.DB, h.NATS, messageID, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updated)
}

func (h *Message) DeleteMessage(c *gin.Context) {
	user, err := currentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	messageID, err := parseID(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	msg, err := message.Get(h.DB, messageID)
	if err != nil {
		if errors.Is(err, message.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "message: message not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if msg.UserID != user.ID && !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "owner or admin required"})
		return
	}

	if err := message.Delete(h.DB, h.NATS, messageID); err != nil {
		if errors.Is(err, message.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "message: message not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
