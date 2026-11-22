package handler

import (
	"druna_server/pkg/model"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func inviteIDParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		NewErrorResponse(c, http.StatusBadRequest, "invalid invite id")
		return 0, false
	}
	return id, true
}

// createInvite godoc
// @Summary Invite friends to an event
// @Description Every invitee must be an accepted friend, otherwise 400 lists them (error.field = "invitees").
// @Tags invites
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param input body model.CreateInviteInput true "Invite"
// @Success 200 {object} model.APIResponse{data=model.Invite}
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/invites [post]
func (h *Handler) createInvite(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	var input model.CreateInviteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}
	invite, err := h.services.Invite.CreateInvite(userID, input)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, invite)
}

// listInvites godoc
// @Summary List invites I organize or am invited to
// @Description pending: I have not answered and it has not ended. upcoming: not ended, and I organize it or have answered. past: ended. Without status: all.
// @Tags invites
// @Produce json
// @Security ApiKeyAuth
// @Param status query string false "pending, upcoming or past"
// @Param limit query int false "Page size, 1-100 (default 50)"
// @Param offset query int false "Page offset"
// @Success 200 {object} model.APIResponse{data=model.InviteListResponse}
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/invites [get]
func (h *Handler) listInvites(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	filter := model.InviteFilter{Status: c.Query("status")}
	var err error
	if v := c.Query("limit"); v != "" {
		if filter.Limit, err = strconv.Atoi(v); err != nil || filter.Limit < 1 || filter.Limit > 100 {
			NewErrorResponse(c, http.StatusBadRequest, "limit must be between 1 and 100")
			return
		}
	}
	if v := c.Query("offset"); v != "" {
		if filter.Offset, err = strconv.Atoi(v); err != nil || filter.Offset < 0 {
			NewErrorResponse(c, http.StatusBadRequest, "offset must be >= 0")
			return
		}
	}
	result, err := h.services.Invite.ListInvites(userID, filter)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, result)
}

// getInvite godoc
// @Summary Get an invite
// @Description 404 unless I organize it or am invited.
// @Tags invites
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "Invite ID"
// @Success 200 {object} model.APIResponse{data=model.Invite}
// @Failure 401 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Router /api/v1/invites/{id} [get]
func (h *Handler) getInvite(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	inviteID, ok := inviteIDParam(c)
	if !ok {
		return
	}
	invite, err := h.services.Invite.GetInvite(inviteID, userID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, invite)
}

// updateInvite godoc
// @Summary Update an invite (organizer only)
// @Description Absent fields are unchanged. Changing startTime or endTime resets every answer to pending.
// @Tags invites
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "Invite ID"
// @Param input body model.UpdateInviteInput true "Fields to change"
// @Success 200 {object} model.APIResponse{data=model.Invite}
// @Failure 400 {object} model.APIResponse
// @Failure 403 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Router /api/v1/invites/{id} [patch]
func (h *Handler) updateInvite(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	inviteID, ok := inviteIDParam(c)
	if !ok {
		return
	}
	var input model.UpdateInviteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}
	invite, err := h.services.Invite.UpdateInvite(inviteID, userID, input)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, invite)
}

// deleteInvite godoc
// @Summary Cancel an invite (organizer only)
// @Tags invites
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "Invite ID"
// @Success 200 {object} model.APIResponse
// @Failure 403 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Router /api/v1/invites/{id} [delete]
func (h *Handler) deleteInvite(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	inviteID, ok := inviteIDParam(c)
	if !ok {
		return
	}
	if err := h.services.Invite.DeleteInvite(inviteID, userID); err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"message": "invite deleted"})
}

// respondInvite godoc
// @Summary Answer an invite
// @Description Invitees only, any number of times. "pending" withdraws the answer. The organizer gets 400.
// @Tags invites
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "Invite ID"
// @Param input body model.RespondInviteInput true "going, not_going or pending"
// @Success 200 {object} model.APIResponse{data=model.Invite}
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Router /api/v1/invites/{id}/respond [post]
func (h *Handler) respondInvite(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	inviteID, ok := inviteIDParam(c)
	if !ok {
		return
	}
	var input model.RespondInviteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}
	invite, err := h.services.Invite.RespondInvite(inviteID, userID, input.Status)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, invite)
}
