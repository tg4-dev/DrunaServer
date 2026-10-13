package handler

import (
	"druna_server/pkg/service"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	_ "druna_server/docs"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Handler struct {
	services *service.Service
}

func NewHandler(services *service.Service) *Handler {
	return &Handler{services: services}
}

func (h *Handler) InitRoutes() *gin.Engine {
	router := gin.New()
	applyTrustedProxies(router)
	router.Use(gin.Recovery())
	router.Use(requestIDMiddleware())
	router.Use(accessLogMiddleware())
	router.Use(metricsMiddleware())
	router.Use(corsMiddleware())

	authLimiter := NewRateLimiter(30)

	ping := router.Group("/ping")
	{
		ping.GET("/", h.ping)
	}

	if swaggerEnabled() {
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	auth := router.Group("/auth", authLimiter.Middleware())
	{
		auth.POST("/sign-up", h.signUp)
		auth.POST("/sign-in", h.signIn)
		auth.POST("/renew-token", h.renewToken)
		auth.POST("/logout", h.logout)
		auth.POST("/telegram", h.telegramAuth)
		auth.POST("/telegram/widget", h.telegramWidgetAuth)
	}

	userLimiter := NewRateLimiter(60)

	apiV1 := router.Group("/api/v1", h.userIdentity)
	h.registerProtectedRoutes(apiV1, userLimiter)

	api := router.Group("/api", h.userIdentity)
	h.registerProtectedRoutes(api, userLimiter)

	return router
}

func (h *Handler) registerProtectedRoutes(api *gin.RouterGroup, userLimiter *RateLimiter) {
	users := api.Group("/users")
	{
		users.GET("/me", h.getCurrentUser)
		users.PATCH("/me", h.updateCurrentUser)
	}

	friends := api.Group("/friends")
	{
		friends.GET("/list", h.getFriendList)
		friends.GET("/search", userLimiter.MiddlewareKey(userRateKey), h.searchUsers)
		friends.GET("/request-list", h.getFriendRequestList)
		friends.GET("/requests/incoming", h.getIncomingFriendRequests)
		friends.GET("/requests/outgoing", h.getOutgoingFriendRequests)
		friends.POST("/request", h.sendFriendRequest)
		friends.POST("/accept", h.acceptFriendRequest)
		friends.POST("/reject", h.rejectFriendRequest)
		friends.DELETE("/", h.deleteFriend)
	}

	events := api.Group("/events")
	{
		events.GET("/", h.getEventList)
		events.POST("/", h.addEvent)
		events.PATCH("/:id", h.updateEvent)
		events.DELETE("/:id", h.deleteEvent)
		events.POST("/free-time", userLimiter.MiddlewareKey(userRateKey), h.getFreeTime)
	}

	groups := api.Group("/groups")
	{
		groups.POST("/create", h.createGroup)
		groups.GET("/list", h.listGroups)
		groups.GET("/:id", h.getGroup)
		groups.DELETE("/:id", h.deleteGroup)
		groups.POST("/:id/leave", h.leaveGroup)
		groups.POST("/:id/members", h.addGroupMember)
		groups.POST("/:id/confirm", h.confirmGroupTime)
		groups.POST("/:id/free-time", userLimiter.MiddlewareKey(userRateKey), h.getGroupFreeTime)
		groups.GET("/:id/events", h.getGroupEventList)
		groups.POST("/:id/events", h.addGroupEvent)
		groups.PATCH("/:id/events/:eventId", h.updateGroupEvent)
		groups.DELETE("/:id/events/:eventId", h.deleteGroupEvent)
	}
}

func corsMiddleware() gin.HandlerFunc {
	origins := os.Getenv("CORS_ORIGINS")
	if origins == "" {
		origins = "*"
	}

	return cors.New(cors.Config{
		AllowOrigins:     strings.Split(origins, ","),
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-ID"},
		ExposeHeaders:    []string{"Content-Length", "X-Request-ID"},
		AllowCredentials: origins != "*",
		MaxAge:           12 * 3600,
	})
}

func userRateKey(c *gin.Context) string {
	id, _ := c.Get(userCtx)
	return fmt.Sprintf("%v:%s", id, c.FullPath())
}

func applyTrustedProxies(router *gin.Engine) {
	raw := strings.TrimSpace(os.Getenv("TRUSTED_PROXIES"))
	if raw == "" {
		if err := router.SetTrustedProxies(nil); err != nil {
			logrus.WithError(err).Warn("failed to disable trusted proxies")
		}
		return
	}
	var proxies []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			proxies = append(proxies, part)
		}
	}
	if err := router.SetTrustedProxies(proxies); err != nil {
		logrus.WithError(err).Warn("failed to set trusted proxies")
	}
}

func swaggerEnabled() bool {
	value := os.Getenv("SWAGGER_ENABLED")
	if value == "" {
		return true
	}
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return true
	}
	return enabled
}
