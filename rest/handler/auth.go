package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/zon/chat/core/auth"
)

type AuthHandler struct {
	deps Deps
}

func NewAuthHandler(deps Deps) *AuthHandler {
	return &AuthHandler{deps: deps}
}

func (h *AuthHandler) Login(c *gin.Context) {
	auth.Login(c.Writer, c.Request)
}

func (h *AuthHandler) Callback(c *gin.Context) {
	auth.Callback(h.deps.DB)(c.Writer, c.Request)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	auth.Logout(c.Writer, c.Request)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	auth.Refresh(c.Writer, c.Request)
}

func (h *AuthHandler) Token(c *gin.Context) {
	auth.ClientToken(h.deps.DB)(c.Writer, c.Request)
}
