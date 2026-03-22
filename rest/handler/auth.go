package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/zon/chat/core/auth"
)

type Auth struct {
	deps Deps
}

func NewAuth(deps Deps) *Auth {
	return &Auth{deps: deps}
}

func (h *Auth) Login(c *gin.Context) {
	auth.Login(c.Writer, c.Request)
}

func (h *Auth) Callback(c *gin.Context) {
	auth.Callback(h.deps.DB)(c.Writer, c.Request)
}

func (h *Auth) Logout(c *gin.Context) {
	auth.Logout(c.Writer, c.Request)
}

func (h *Auth) Refresh(c *gin.Context) {
	auth.Refresh(c.Writer, c.Request)
}

func (h *Auth) Token(c *gin.Context) {
	auth.ClientToken(h.deps.DB)(c.Writer, c.Request)
}
