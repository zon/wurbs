package handler

import (
	"errors"
	"fmt"
	"log"
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

type Member struct {
	deps Deps
}

func NewMember(deps Deps) *Member {
	return &Member{deps: deps}
}

type addMemberRequest struct {
	UserID *uint  `json:"user_id"`
	Email  string `json:"email"`
}

func (h *Member) AddMember(c *gin.Context) {
	currentUser, err := currentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	if !currentUser.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin required"})
		return
	}

	channelID, err := parseID(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		if err := h.deps.NATS.Publish(fmt.Sprintf("wurbs.channel.%d.members", channelID), event); err != nil {
			log.Printf("failed to publish member event: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"added": true})
}

func (h *Member) RemoveMember(c *gin.Context) {
	currentUser, err := currentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	if !currentUser.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin required"})
		return
	}

	channelID, err := parseID(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := parseID(c, "user_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		if err := h.deps.NATS.Publish(fmt.Sprintf("wurbs.channel.%d.members", channelID), event); err != nil {
			log.Printf("failed to publish member event: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"removed": true})
}

func (h *Member) ListMembers(c *gin.Context) {
	if _, err := currentUser(c); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	channelID, err := parseID(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
