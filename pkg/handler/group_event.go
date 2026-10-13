package handler

import (
	"druna_server/pkg/model"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const maxEventPageSize = 100

func parseEventFilter(c *gin.Context) (model.EventFilter, error) {
	filter := model.EventFilter{Limit: 50}
	if limitStr := c.Query("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > maxEventPageSize {
			return filter, errors.New("limit must be between 1 and 100")
		}
		filter.Limit = limit
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			return filter, errors.New("offset must be a non-negative integer")
		}
		filter.Offset = offset
	}
	filter.Type = c.Query("type")
	if from := c.Query("dateFrom"); from != "" {
		t, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return filter, errors.New("dateFrom must be RFC3339")
		}
		filter.DateFrom = &t
	}
	if to := c.Query("dateTo"); to != "" {
		t, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return filter, errors.New("dateTo must be RFC3339")
		}
		filter.DateTo = &t
	}
	return filter, nil
}

// getGroupEventList godoc
// @Summary List events for a group
// @Tags groups
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "Group ID"
// @Param limit query int false "Page size"
// @Param offset query int false "Page offset"
// @Param type query string false "Event type filter"
// @Param dateFrom query string false "RFC3339 lower bound"
// @Param dateTo query string false "RFC3339 upper bound"
// @Success 200 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Failure 403 {object} model.APIResponse
// @Router /api/v1/groups/{id}/events [get]
func (h *Handler) getGroupEventList(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid group id")
		return
	}

	filter, err := parseEventFilter(c)
	if err != nil {
		NewErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.services.Group.ListGroupEvents(groupID, userID, filter)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, result)
}

// addGroupEvent godoc
// @Summary Create a group event (any member)
// @Tags groups
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "Group ID"
// @Param input body model.Event true "Event payload"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Failure 403 {object} model.APIResponse
// @Router /api/v1/groups/{id}/events [post]
func (h *Handler) addGroupEvent(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid group id")
		return
	}

	var input model.Event
	if err := c.BindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	eventID, err := h.services.Group.CreateGroupEvent(groupID, userID, input)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"eventId": eventID})
}

// updateGroupEvent godoc
// @Summary Update a group event (creator or group owner)
// @Tags groups
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "Group ID"
// @Param eventId path int true "Event ID"
// @Param input body model.Event true "Event payload"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Failure 403 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Router /api/v1/groups/{id}/events/{eventId} [patch]
func (h *Handler) updateGroupEvent(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid group id")
		return
	}
	eventID, err := strconv.Atoi(c.Param("eventId"))
	if err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid event id")
		return
	}

	var input model.Event
	if err := c.BindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.services.Group.UpdateGroupEvent(groupID, eventID, userID, input); err != nil {
		writeGroupEventError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"message": "group event updated"})
}

// deleteGroupEvent godoc
// @Summary Delete a group event (creator or group owner)
// @Tags groups
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "Group ID"
// @Param eventId path int true "Event ID"
// @Success 200 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Failure 403 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Router /api/v1/groups/{id}/events/{eventId} [delete]
func (h *Handler) deleteGroupEvent(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid group id")
		return
	}
	eventID, err := strconv.Atoi(c.Param("eventId"))
	if err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid event id")
		return
	}

	if err := h.services.Group.DeleteGroupEvent(groupID, eventID, userID); err != nil {
		writeGroupEventError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"message": "group event deleted"})
}

func writeGroupEventError(c *gin.Context, err error) {
	writeServiceError(c, err)
}
