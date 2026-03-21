package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zon/chat/core/auth"
)

type UserHandler struct {
	deps Deps
}

func NewUserHandler(deps Deps) *UserHandler {
	return &UserHandler{deps: deps}
}

func (h *UserHandler) GetUser(c *gin.Context) {
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

func (h *UserHandler) UpdateUser(c *gin.Context) {
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

	var updateErr error
	if isSelf {
		updateErr = auth.UpdateUser(h.deps.DB, targetUser, input, currentUser.IsAdmin)
	} else {
		updateErr = auth.UpdateUserAsAdmin(h.deps.DB, currentUser, targetUser, input)
	}

	if updateErr != nil {
		if errors.Is(updateErr, auth.ErrTestAdminModifyRealUser) || errors.Is(updateErr, auth.ErrRealAdminModifyTestUser) {
			c.JSON(http.StatusForbidden, gin.H{"error": updateErr.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": updateErr.Error()})
		return
	}

	updatedUser, _ := auth.GetUserByID(h.deps.DB, userID)
	c.JSON(http.StatusOK, userToResponse(updatedUser))
}
