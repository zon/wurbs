package handler

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zon/chat/core/auth"
	"github.com/zon/chat/core/user"
)

func parseID(c *gin.Context, param string) (value uint, ok error) {
	raw := c.Param(param)
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("common: invalid %s", param)
	}
	return uint(id), nil
}

func currentUser(c *gin.Context) (user *user.User, ok error) {
	u, err := auth.UserFromContext(c.Request.Context())
	if err != nil {
		return nil, fmt.Errorf("common: unauthorized")
	}
	return u, nil
}

type userResponse struct {
	ID        string  `json:"id"`
	Username  *string `json:"username"`
	Admin     bool    `json:"admin"`
	Inactive  bool    `json:"inactive"`
	CreatedAt string  `json:"createdAt"`
}

func userToResponse(u *user.User) userResponse {
	return userResponse{
		ID:        fmt.Sprintf("%d", u.ID),
		Username:  u.Username,
		Admin:     u.IsAdmin,
		Inactive:  !u.IsActive,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
