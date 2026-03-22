package handler

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zon/chat/core/channel"
	"github.com/zon/chat/core/user"
)

type UserEvent struct {
	UserID   string  `json:"userId"`
	Username *string `json:"username"`
}

type User struct {
	deps Deps
}

func NewUser(deps Deps) *User {
	return &User{deps: deps}
}

func (h *User) GetUser(c *gin.Context) {
	if _, err := currentUser(c); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user: invalid user id"})
		return
	}

	u, err := user.GetUserByID(h.deps.DB, userID)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user: user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, userToResponse(u))
}

type updateUserRequest struct {
	Username *string `json:"username"`
	Email    *string `json:"email"`
	Admin    *bool   `json:"admin"`
	Inactive *bool   `json:"inactive"`
}

func (h *User) UpdateUser(c *gin.Context) {
	currentUser, err := currentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user: invalid user id"})
		return
	}

	targetUser, err := user.GetUserByID(h.deps.DB, userID)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user: user not found"})
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

	if req.Email != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email may not be edited"})
		return
	}

	isSelf := currentUser.ID == targetUser.ID
	if !isSelf && !currentUser.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot edit other users"})
		return
	}

	input := user.UpdateUserInput{
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

	var updateErr error
	if isSelf {
		updateErr = user.UpdateUser(h.deps.DB, targetUser, input, currentUser.IsAdmin)
	} else {
		updateErr = user.UpdateUserAsAdmin(h.deps.DB, currentUser, targetUser, input)
	}

	if updateErr != nil {
		if errors.Is(updateErr, user.ErrTestAdminModifyRealUser) || errors.Is(updateErr, user.ErrRealAdminModifyTestUser) {
			c.JSON(http.StatusForbidden, gin.H{"error": updateErr.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": updateErr.Error()})
		return
	}

	updatedUser, _ := user.GetUserByID(h.deps.DB, userID)

	if req.Username != nil && h.deps.NATS != nil {
		channels, _ := channel.ListForUser(h.deps.DB, targetUser.ID)
		for _, ch := range channels {
			event := UserEvent{
				UserID:   fmt.Sprintf("%d", targetUser.ID),
				Username: req.Username,
			}
			if err := h.deps.NATS.Publish(fmt.Sprintf("wurbs.channel.%d.users", ch.ID), event); err != nil {
				log.Printf("failed to publish user event: %v", err)
			}
		}
	}

	c.JSON(http.StatusOK, userToResponse(updatedUser))
}
