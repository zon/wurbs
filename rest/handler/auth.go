package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/zon/chat/core/auth"
	"gorm.io/gorm"
)

type Auth struct {
	DB *gorm.DB
}

func NewAuth(db *gorm.DB) *Auth {
	return &Auth{DB: db}
}

func (h *Auth) Login(c *gin.Context) {
	auth.Login(c.Writer, c.Request)
}

func (h *Auth) Callback(c *gin.Context) {
	auth.Callback(h.DB)(c.Writer, c.Request)
}

func (h *Auth) Logout(c *gin.Context) {
	auth.Logout(c.Writer, c.Request)
}

func (h *Auth) Refresh(c *gin.Context) {
	auth.Refresh(c.Writer, c.Request)
}

func (h *Auth) Token(c *gin.Context) {
	auth.ClientToken(h.DB)(c.Writer, c.Request)
}
