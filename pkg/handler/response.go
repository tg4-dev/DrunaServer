package handler

import (
	"druna_server/pkg/model"
	"druna_server/pkg/repository"
	"druna_server/pkg/service"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type ErrorResponse struct {
	Message string `json:"message"`
}

func Success(c *gin.Context, statusCode int, data interface{}) {
	c.JSON(statusCode, model.APIResponse{Data: data})
}

func NewErrorResponse(c *gin.Context, statusCode int, message string) {
	logrus.WithField("request_id", c.GetString(requestIDKey)).Error(message)
	c.AbortWithStatusJSON(statusCode, model.APIResponse{
		Error: &model.ErrorBody{Message: message, Code: statusCode},
	})
}

func writeServiceError(c *gin.Context, err error) {
	var domain *service.DomainError
	if errors.As(err, &domain) {
		NewErrorResponse(c, statusForKind(domain.Kind), domain.Error())
		return
	}
	if errors.Is(err, repository.ErrNotFound) {
		NewErrorResponse(c, http.StatusNotFound, "not found")
		return
	}
	if errors.Is(err, repository.ErrDuplicate) {
		NewErrorResponse(c, http.StatusConflict, "conflict")
		return
	}
	logrus.WithField("request_id", c.GetString(requestIDKey)).WithError(err).Error("unhandled service error")
	NewErrorResponse(c, http.StatusInternalServerError, "internal error")
}

func statusForKind(kind service.ErrorKind) int {
	switch kind {
	case service.KindValidation:
		return http.StatusBadRequest
	case service.KindUnauthorized:
		return http.StatusUnauthorized
	case service.KindForbidden:
		return http.StatusForbidden
	case service.KindNotFound:
		return http.StatusNotFound
	case service.KindConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
