package handler

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

const (
	authorizationHeader = "Authorization"
	userCtx             = "userID"
	requestIDKey        = "requestID"
)

const maxRequestIDLen = 128

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// validRequestID reports whether a client-supplied X-Request-ID is safe to
// echo back and log.
func validRequestID(id string) bool {
	return len(id) <= maxRequestIDLen && requestIDPattern.MatchString(id)
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if !validRequestID(requestID) {
			requestID = uuid.New().String()
		}
		c.Set(requestIDKey, requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func accessLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logrus.WithFields(logrus.Fields{
			"request_id": c.GetString(requestIDKey),
			"method":     c.Request.Method,
			"path":       c.Request.URL.Path,
			"status":     c.Writer.Status(),
			"duration":   time.Since(start).String(),
		}).Info("request completed")
	}
}

func (h *Handler) userIdentity(c *gin.Context) {
	header := c.GetHeader(authorizationHeader)
	if header == "" {
		NewErrorResponse(c, http.StatusUnauthorized, "empty auth header")
		return
	}

	headerParts := strings.Split(header, " ")
	if len(headerParts) != 2 || headerParts[0] != "Bearer" {
		NewErrorResponse(c, http.StatusUnauthorized, "invalid auth header")
		return
	}

	if len(headerParts[1]) == 0 {
		NewErrorResponse(c, http.StatusUnauthorized, "token is empty")
		return
	}

	userId, _, err := h.services.Authorization.ParseAccessToken(headerParts[1])
	if err != nil {
		writeServiceError(c, err)
		return
	}

	c.Set(userCtx, userId)
	c.Next()
}
