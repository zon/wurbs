package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zon/chat/rest/handler"
	"gorm.io/gorm"
)

type NATSPublisher interface {
	Publish(subject string, data any) error
}

type Deps struct {
	DB   *gorm.DB
	NATS NATSPublisher
}

func New(deps Deps, authMiddleware func(http.Handler) http.Handler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", health)

	authHandler := handler.NewAuthHandler(handler.Deps{DB: deps.DB, NATS: deps.NATS})
	r.GET("/auth/login", authHandler.Login)
	r.GET("/auth/callback", authHandler.Callback)
	r.POST("/auth/logout", authHandler.Logout)
	r.POST("/auth/refresh", authHandler.Refresh)
	r.POST("/auth/token", authHandler.Token)

	api := r.Group("")
	api.Use(wrapMiddleware(authMiddleware))

	channelHandler := handler.NewChannelHandler(handler.Deps{DB: deps.DB, NATS: deps.NATS})
	api.POST("/channels", channelHandler.CreateChannel)
	api.GET("/channels", channelHandler.ListChannels)
	api.GET("/channels/:id", channelHandler.GetChannel)
	api.PATCH("/channels/:id", channelHandler.UpdateChannel)
	api.DELETE("/channels/:id", channelHandler.DeleteChannel)

	memberHandler := handler.NewMemberHandler(handler.Deps{DB: deps.DB, NATS: deps.NATS})
	api.POST("/channels/:id/members", memberHandler.AddMember)
	api.DELETE("/channels/:id/members/:user_id", memberHandler.RemoveMember)
	api.GET("/channels/:id/members", memberHandler.ListMembers)

	messageHandler := handler.NewMessageHandler(handler.Deps{DB: deps.DB, NATS: deps.NATS})
	api.POST("/channels/:id/messages", messageHandler.CreateMessage)
	api.GET("/channels/:id/messages", messageHandler.ListMessages)

	api.PATCH("/messages/:id", messageHandler.UpdateMessage)
	api.DELETE("/messages/:id", messageHandler.DeleteMessage)

	userHandler := handler.NewUserHandler(handler.Deps{DB: deps.DB, NATS: deps.NATS})
	api.GET("/users/:id", userHandler.GetUser)
	api.PATCH("/users/:id", userHandler.UpdateUser)

	return r
}

func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func wrapMiddleware(mw func(http.Handler) http.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		var called bool
		wrapped := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Request = r
			called = true
		}))
		wrapped.ServeHTTP(c.Writer, c.Request)
		if !called {
			c.Abort()
			return
		}
		c.Next()
	}
}
