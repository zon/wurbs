package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zon/chat/core/channel"
)

type ChannelHandler struct {
	deps Deps
}

func NewChannelHandler(deps Deps) *ChannelHandler {
	return &ChannelHandler{deps: deps}
}

type createChannelRequest struct {
	Name     string `json:"name" binding:"required"`
	IsPublic bool   `json:"is_public"`
	IsTest   bool   `json:"is_test"`
}

func (h *ChannelHandler) CreateChannel(c *gin.Context) {
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

	ch, err := channel.CreateAsAdmin(h.deps.DB, user, req.Name, req.IsPublic, req.IsTest)
	if err != nil {
		if errors.Is(err, channel.ErrTestAdminInReal) || errors.Is(err, channel.ErrRealAdminInTest) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.deps.NATS != nil {
		_ = h.deps.NATS.Publish(fmt.Sprintf("wurbs.channel.%d.created", ch.ID), ch)
	}

	c.JSON(http.StatusCreated, ch)
}

func (h *ChannelHandler) ListChannels(c *gin.Context) {
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

func (h *ChannelHandler) GetChannel(c *gin.Context) {
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

func (h *ChannelHandler) UpdateChannel(c *gin.Context) {
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

	if err := channel.UpdateAsAdmin(h.deps.DB, ch, user, input); err != nil {
		if errors.Is(err, channel.ErrTestAdminInReal) || errors.Is(err, channel.ErrRealAdminInTest) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	updated, _ := channel.Get(h.deps.DB, id)
	c.JSON(http.StatusOK, updated)
}

func (h *ChannelHandler) DeleteChannel(c *gin.Context) {
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

	err := channel.DeleteAsAdmin(h.deps.DB, id, user)
	if err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
			return
		}
		if errors.Is(err, channel.ErrTestAdminInReal) || errors.Is(err, channel.ErrRealAdminInTest) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.deps.NATS != nil {
		_ = h.deps.NATS.Publish(fmt.Sprintf("wurbs.channel.%d.deleted", id), gin.H{"id": id})
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
