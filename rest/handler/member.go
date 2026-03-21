package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zon/chat/core/auth"
	"github.com/zon/chat/core/channel"
)

type MemberHandler struct {
	deps Deps
}

func NewMemberHandler(deps Deps) *MemberHandler {
	return &MemberHandler{deps: deps}
}

type addMemberRequest struct {
	UserID *uint  `json:"user_id"`
	Email  string `json:"email"`
}

func (h *MemberHandler) AddMember(c *gin.Context) {
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

	err = channel.AddMemberAsAdmin(h.deps.DB, channelID, user, target)
	if err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
			return
		}
		if errors.Is(err, channel.ErrTestUserInReal) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, channel.ErrRealUserInTest) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, channel.ErrTestAdminInReal) || errors.Is(err, channel.ErrRealAdminInTest) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, channel.ErrRealAdminModifyTestUser) || errors.Is(err, channel.ErrTestAdminModifyRealUser) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.deps.NATS != nil {
		_ = h.deps.NATS.Publish(fmt.Sprintf("wurbs.channel.%d.members.added", channelID), gin.H{
			"channel_id": channelID,
			"user_id":    target.ID,
		})
	}

	c.JSON(http.StatusOK, gin.H{"added": true})
}

func (h *MemberHandler) RemoveMember(c *gin.Context) {
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

	err := channel.RemoveMemberAsAdmin(h.deps.DB, channelID, userID, user)
	if err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
			return
		}
		if errors.Is(err, channel.ErrTestAdminInReal) || errors.Is(err, channel.ErrRealAdminInTest) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, channel.ErrRealAdminModifyTestUser) || errors.Is(err, channel.ErrTestAdminModifyRealUser) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if h.deps.NATS != nil {
		_ = h.deps.NATS.Publish(fmt.Sprintf("wurbs.channel.%d.members.removed", channelID), gin.H{
			"channel_id": channelID,
			"user_id":    userID,
		})
	}

	c.JSON(http.StatusOK, gin.H{"removed": true})
}

func (h *MemberHandler) ListMembers(c *gin.Context) {
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
