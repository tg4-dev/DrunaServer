package handler

import (
	"druna_server/pkg/model"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// createGroup godoc
// @Summary Create group
// @Tags groups
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param input body model.Group true "Group payload"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/groups/create [post]
func (h *Handler) createGroup(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	var input model.Group
	if err := c.ShouldBindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}
	input.OwnerID = userID

	id, err := h.services.Group.CreateGroup(input)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"message": "group created", "groupId": id})
}

// listGroups godoc
// @Summary List groups for current user
// @Tags groups
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/groups/list [get]
func (h *Handler) listGroups(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	groups, err := h.services.Group.ListGroups(userID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"groups": groups})
}

// getGroup godoc
// @Summary Get group details with members
// @Tags groups
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "Group ID"
// @Success 200 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Router /api/v1/groups/{id} [get]
func (h *Handler) getGroup(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid group id")
		return
	}
	details, err := h.services.Group.GetGroupDetails(groupID, userID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, details)
}

// addGroupMember godoc
// @Summary Add friend to group (owner only)
// @Tags groups
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "Group ID"
// @Param input body model.AddGroupMemberDoc true "Member username"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/groups/{id}/members [post]
func (h *Handler) addGroupMember(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid group id")
		return
	}
	var input model.AddGroupMemberDoc
	if err := c.ShouldBindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.services.Group.AddGroupMember(groupID, userID, input.Username); err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"message": "member added"})
}

// deleteGroup godoc
// @Summary Delete group (owner only)
// @Tags groups
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "Group ID"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/groups/{id} [delete]
func (h *Handler) deleteGroup(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid group id")
		return
	}
	if err := h.services.Group.DeleteGroup(groupID, userID); err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"message": "group deleted"})
}

// leaveGroup godoc
// @Summary Leave group (non-owner members)
// @Tags groups
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "Group ID"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/groups/{id}/leave [post]
func (h *Handler) leaveGroup(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid group id")
		return
	}
	if err := h.services.Group.LeaveGroup(groupID, userID); err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"message": "left group"})
}

// confirmGroupTime godoc
// @Summary Confirm proposed group meeting time
// @Tags groups
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "Group ID"
// @Param input body model.ConfirmGroupTimeDoc true "Confirmed time"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/groups/{id}/confirm [post]
func (h *Handler) confirmGroupTime(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid group id")
		return
	}
	var input model.ConfirmGroupTimeDoc
	if err := c.ShouldBindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.services.Group.ConfirmMemberTime(groupID, userID, input.ConfirmedTime); err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"message": "time confirmed"})
}

// getGroupFreeTime godoc
// @Summary Get intersection of members' free time slots for a day
// @Tags groups
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param id path int true "Group ID"
// @Param input body model.GroupFreeTimeInputDoc true "Date YYYY-MM-DD"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/groups/{id}/free-time [post]
func (h *Handler) getGroupFreeTime(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	groupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid group id")
		return
	}
	var input model.GroupFreeTimeInputDoc
	if err := c.ShouldBindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}
	date, err := parseCalendarDate(input.Date, input.Timezone)
	if err != nil {
		NewErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}
	slots, err := h.services.Group.GetGroupFreeTime(groupID, userID, date)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"freeSlots": slots})
}
