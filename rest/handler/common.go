package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zon/chat/core/auth"
	"gorm.io/gorm"
)

type Deps struct {
	DB   *gorm.DB
	NATS interface {
		Publish(subject string, data any) error
	}
}

type handler struct {
	deps Deps
}

func parseID(c *gin.Context, param string) (uint, bool) {
	raw := c.Param(param)
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + param})
		return 0, false
	}
	return uint(id), true
}

func currentUser(c *gin.Context) (*auth.User, bool) {
	u, err := auth.UserFromContext(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	return u, true
}

type userResponse struct {
	ID        string  `json:"id"`
	Username  *string `json:"username"`
	Admin     bool    `json:"admin"`
	Inactive  bool    `json:"inactive"`
	CreatedAt string  `json:"createdAt"`
}

func userToResponse(u *auth.User) userResponse {
	return userResponse{
		ID:        fmt.Sprintf("%d", u.ID),
		Username:  u.Username,
		Admin:     u.IsAdmin,
		Inactive:  !u.IsActive,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
