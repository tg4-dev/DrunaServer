package handler

import (
	"druna_server/pkg/model"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// getEventList godoc
// @Summary List events for current user
// @Tags events
// @Produce json
// @Security ApiKeyAuth
// @Param limit query int false "Page size"
// @Param offset query int false "Page offset"
// @Param type query string false "Event type filter"
// @Param dateFrom query string false "RFC3339 lower bound"
// @Param dateTo query string false "RFC3339 upper bound"
// @Success 200 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/events/ [get]
func (h *Handler) getEventList(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}

	filter, err := parseEventFilter(c)
	if err != nil {
		NewErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.services.Event.GetEventList(userID, filter)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, result)
}

// getFreeTime godoc
// @Summary Compute personal free time slots for a day
// @Tags events
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param input body model.FreeTimeInputDoc true "Date YYYY-MM-DD"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/events/free-time [post]
func (h *Handler) getFreeTime(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}

	var input model.FreeTimeInputDoc
	if err := c.ShouldBindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.Date == "" {
		NewErrorResponse(c, http.StatusBadRequest, "date is required")
		return
	}

	date, err := parseCalendarDate(input.Date, input.Timezone)
	if err != nil {
		NewErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	slots, err := h.services.Event.GetFreeTime(userID, date)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"freeSlots": slots})
}

// addEvent godoc
// @Summary Create event
// @Tags events
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param input body model.Event true "Event payload"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/events/ [post]
func (h *Handler) addEvent(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}

	var input model.Event
	if err := c.BindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}
	input.UserID = userID

	eventID, err := h.services.Event.CreateEvent(input)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"eventId": eventID})
}

// updateEvent godoc
// @Summary Update event
// @Tags events
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "Event ID"
// @Param input body model.Event true "Event payload"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/events/{id} [patch]
func (h *Handler) updateEvent(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}

	eventID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid event id")
		return
	}

	var input model.Event
	if err := c.BindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}
	input.ID = eventID
	input.UserID = userID

	if err := h.services.Event.UpdateEvent(userID, input); err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"message": "event updated"})
}

// deleteEvent godoc
// @Summary Delete event
// @Tags events
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "Event ID"
// @Success 200 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Failure 403 {object} model.APIResponse
// @Router /api/v1/events/{id} [delete]
func (h *Handler) deleteEvent(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}

	eventID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid event id")
		return
	}

	if err := h.services.Event.DeleteEvent(userID, eventID); err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"message": "event deleted"})
}
