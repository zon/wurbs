package handler

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zon/chat/core/channel"
	"github.com/zon/chat/core/message"
	"gorm.io/gorm"
)

type Channel struct {
	DB   *gorm.DB
	NATS message.Publisher
}

func NewChannel(db *gorm.DB, nats message.Publisher) *Channel {
	return &Channel{DB: db, NATS: nats}
}

type createChannelRequest struct {
	Name     string `json:"name" binding:"required"`
	IsPublic bool   `json:"is_public"`
	IsTest   bool   `json:"is_test"`
}

func (h *Channel) CreateChannel(c *gin.Context) {
	user, err := currentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
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

	ch, err := channel.CreateAsAdmin(h.DB, user, req.Name, req.IsPublic, req.IsTest)
	if err != nil {
		if errors.Is(err, channel.ErrTestAdminInReal) || errors.Is(err, channel.ErrRealAdminInTest) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.NATS != nil {
		if err := h.NATS.Publish(fmt.Sprintf("wurbs.channel.%d.created", ch.ID), ch); err != nil {
			log.Printf("failed to publish channel created event: %v", err)
		}
	}

	c.JSON(http.StatusCreated, ch)
}

func (h *Channel) ListChannels(c *gin.Context) {
	if _, err := currentUser(c); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	channels, err := channel.List(h.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, channels)
}

func (h *Channel) GetChannel(c *gin.Context) {
	if _, err := currentUser(c); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	id, err := parseID(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ch, err := channel.Get(h.DB, id)
	if err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "channel: channel not found"})
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

func (h *Channel) UpdateChannel(c *gin.Context) {
	user, err := currentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	if !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin required"})
		return
	}

	id, err := parseID(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ch, err := channel.Get(h.DB, id)
	if err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "channel: channel not found"})
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

	if err := channel.UpdateAsAdmin(h.DB, ch, user, input); err != nil {
		if errors.Is(err, channel.ErrTestAdminInReal) || errors.Is(err, channel.ErrRealAdminInTest) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	updated, _ := channel.Get(h.DB, id)
	c.JSON(http.StatusOK, updated)
}

func (h *Channel) DeleteChannel(c *gin.Context) {
	user, err := currentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	if !user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin required"})
		return
	}

	id, err := parseID(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = channel.DeleteAsAdmin(h.DB, id, user)
	if err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "channel: channel not found"})
			return
		}
		if errors.Is(err, channel.ErrTestAdminInReal) || errors.Is(err, channel.ErrRealAdminInTest) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.NATS != nil {
		if err := h.NATS.Publish(fmt.Sprintf("wurbs.channel.%d.deleted", id), gin.H{"id": id}); err != nil {
			log.Printf("failed to publish channel deleted event: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
