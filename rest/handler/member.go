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
	"github.com/zon/chat/core/message"
	"github.com/zon/chat/core/user"
	"gorm.io/gorm"
)

type MemberEvent struct {
	Type string     `json:"type"`
	User MemberUser `json:"user"`
}

type MemberUser struct {
	ID        string    `json:"id"`
	Username  *string   `json:"username"`
	Email     string    `json:"email"`
	Admin     bool      `json:"admin"`
	Inactive  bool      `json:"inactive"`
	CreatedAt time.Time `json:"createdAt"`
}

type Member struct {
	DB   *gorm.DB
	NATS message.Publisher
}

func NewMember(db *gorm.DB, nats message.Publisher) *Member {
	return &Member{DB: db, NATS: nats}
}

type inviteMemberRequest struct {
	UserID *string `json:"userId"`
	Email  string  `json:"email"`
}

type memberResponse struct {
	ChannelID string    `json:"channelId"`
	UserID    string    `json:"userId"`
	JoinedAt  time.Time `json:"joinedAt,omitempty"`
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

	var req inviteMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UserID == nil && req.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userId or email required"})
		return
	}

	var target *user.User

	if req.UserID != nil {
		target, err = user.GetUserByID(h.DB, *req.UserID)
		if err != nil {
			if errors.Is(err, user.ErrUserNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		target, err = auth.FindOrCreateUserByEmail(h.DB, req.Email, "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	err = channel.AddMemberAsAdmin(h.DB, channelID, currentUser, target)
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

	if h.NATS != nil {
		event := MemberEvent{
			Type: "joined",
			User: MemberUser{
				ID:        fmt.Sprintf("%d", target.ID),
				Username:  target.Username,
				Email:     target.Email,
				Admin:     target.IsAdmin,
				Inactive:  !target.IsActive,
				CreatedAt: target.CreatedAt,
			},
		}
		if err := h.NATS.Publish(fmt.Sprintf("wurbs.channel.%d.members", channelID), event); err != nil {
			log.Printf("failed to publish member event: %v", err)
		}
	}

	c.JSON(http.StatusCreated, memberResponse{
		ChannelID: fmt.Sprintf("%d", channelID),
		UserID:    fmt.Sprintf("%d", target.ID),
		JoinedAt:  time.Now(),
	})
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

	target, err := user.GetUserByID(h.DB, fmt.Sprintf("%d", userID))
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	err = channel.RemoveMemberAsAdmin(h.DB, channelID, userID, currentUser)
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

	if h.NATS != nil {
		event := MemberEvent{
			Type: "left",
			User: MemberUser{
				ID:        fmt.Sprintf("%d", target.ID),
				Username:  target.Username,
				Email:     target.Email,
				Admin:     target.IsAdmin,
				Inactive:  !target.IsActive,
				CreatedAt: target.CreatedAt,
			},
		}
		if err := h.NATS.Publish(fmt.Sprintf("wurbs.channel.%d.members", channelID), event); err != nil {
			log.Printf("failed to publish member event: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
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

	members, err := channel.Members(h.DB, channelID)
	if err != nil {
		if errors.Is(err, channel.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responses := make([]userResponse, len(members))
	for i, m := range members {
		responses[i] = userToResponse(&m)
	}
	c.JSON(http.StatusOK, responses)
}
