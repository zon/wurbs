package handler

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zon/chat/core/auth"
	"github.com/zon/chat/core/channel"
	"github.com/zon/chat/core/user"
)

type MemberEvent struct {
	Type string     `json:"type"`
	User MemberUser `json:"user"`
}

type MemberUser struct {
	ID        uint      `json:"id"`
	Username  *string   `json:"username"`
	Email     string    `json:"email"`
	Admin     bool      `json:"admin"`
	Inactive  bool      `json:"inactive"`
	CreatedAt time.Time `json:"createdAt"`
}

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
	currentUser, ok := currentUser(c)
	if !ok {
		return
	}
	if !currentUser.IsAdmin {
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
		target, err = user.GetUserByID(h.deps.DB, fmt.Sprintf("%d", *req.UserID))
		if err != nil {
			if errors.Is(err, user.ErrUserNotFound) {
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

	err = channel.AddMemberAsAdmin(h.deps.DB, channelID, currentUser, target)
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
		event := MemberEvent{
			Type: "joined",
			User: MemberUser{
				ID:        target.ID,
				Username:  target.Username,
				Email:     target.Email,
				Admin:     target.IsAdmin,
				Inactive:  !target.IsActive,
				CreatedAt: target.CreatedAt,
			},
		}
		_ = h.deps.NATS.Publish(fmt.Sprintf("wurbs.channel.%d.members", channelID), event)
	}

	c.JSON(http.StatusOK, gin.H{"added": true})
}

func (h *MemberHandler) RemoveMember(c *gin.Context) {
	currentUser, ok := currentUser(c)
	if !ok {
		return
	}
	if !currentUser.IsAdmin {
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

	target, err := user.GetUserByID(h.deps.DB, fmt.Sprintf("%d", userID))
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	err = channel.RemoveMemberAsAdmin(h.deps.DB, channelID, userID, currentUser)
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
		event := MemberEvent{
			Type: "left",
			User: MemberUser{
				ID:        target.ID,
				Username:  target.Username,
				Email:     target.Email,
				Admin:     target.IsAdmin,
				Inactive:  !target.IsActive,
				CreatedAt: target.CreatedAt,
			},
		}
		_ = h.deps.NATS.Publish(fmt.Sprintf("wurbs.channel.%d.members", channelID), event)
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
