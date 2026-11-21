package handler

import (
	"druna_server/pkg/model"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type friendRequestInput struct {
	Username string `json:"username" binding:"required"`
}

func (h *Handler) getUserIdFromToken(c *gin.Context) int {
	id, ok := c.Get(userCtx)
	if !ok {
		NewErrorResponse(c, http.StatusInternalServerError, "user id not found")
		return 0
	}
	userID, ok := id.(int)
	if !ok {
		NewErrorResponse(c, http.StatusInternalServerError, "user id is of invalid type")
		return 0
	}
	return userID
}

// searchUsers godoc
// @Summary Search users by username prefix
// @Description Excludes the caller. Each result has relation: friend, incoming, outgoing or none.
// @Tags friends
// @Produce json
// @Security ApiKeyAuth
// @Param username query string true "Username prefix"
// @Success 200 {object} model.APIResponse{data=[]model.UserSearchResult}
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/friends/search [get]
func (h *Handler) searchUsers(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	prefix := c.Query("username")
	users, err := h.services.Authorization.SearchUsers(userID, prefix)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"users": users})
}

// getFriendList godoc
// @Summary List accepted friends
// @Tags friends
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/friends/list [get]
func (h *Handler) getFriendList(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	friends, err := h.services.Friendship.FriendList(userID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"friends": friends})
}

// getFriendRequestList godoc
// @Summary List all pending friend requests
// @Tags friends
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/friends/request-list [get]
func (h *Handler) getFriendRequestList(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	friends, err := h.services.Friendship.FriendRequestList(userID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"friends": friends})
}

// getIncomingFriendRequests godoc
// @Summary List incoming friend requests
// @Tags friends
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/friends/requests/incoming [get]
func (h *Handler) getIncomingFriendRequests(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	friends, err := h.services.Friendship.IncomingFriendRequests(userID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"friends": friends})
}

// getOutgoingFriendRequests godoc
// @Summary List outgoing friend requests
// @Tags friends
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/friends/requests/outgoing [get]
func (h *Handler) getOutgoingFriendRequests(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	friends, err := h.services.Friendship.OutgoingFriendRequests(userID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"friends": friends})
}

// sendFriendRequest godoc
// @Summary Send friend request
// @Tags friends
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param input body model.FriendRequestDoc true "Target username"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/friends/request [post]
func (h *Handler) sendFriendRequest(c *gin.Context) {
	var input friendRequestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid input")
		return
	}
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	if err := h.services.Friendship.SendFriendRequest(userID, input.Username); err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"message": "friend request sent"})
}

// acceptFriendRequest godoc
// @Summary Accept friend request
// @Tags friends
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param input body model.FriendRequestDoc true "Requester username"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/friends/accept [post]
func (h *Handler) acceptFriendRequest(c *gin.Context) {
	var input friendRequestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid input")
		return
	}
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	if err := h.services.Friendship.AcceptFriendRequest(userID, input.Username); err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"message": "friend request accepted"})
}

// rejectFriendRequest godoc
// @Summary Reject friend request
// @Tags friends
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param input body model.FriendRequestDoc true "Requester username"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/friends/reject [post]
func (h *Handler) rejectFriendRequest(c *gin.Context) {
	var input friendRequestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid input")
		return
	}
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	if err := h.services.Friendship.RejectFriendRequest(userID, input.Username); err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"message": "friend request rejected"})
}

// deleteFriend godoc
// @Summary Remove friend
// @Tags friends
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param input body model.FriendRequestDoc true "Friend username"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/friends/ [delete]
func (h *Handler) deleteFriend(c *gin.Context) {
	var input friendRequestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid input")
		return
	}
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	if err := h.services.Friendship.DeleteFriend(userID, input.Username); err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"message": "friend deleted", "username": input.Username})
}

// parseRangeBound accepts RFC3339 or YYYY-MM-DD (midnight in timezone).
// A date-only upper bound is inclusive, so it moves to the next midnight.
func parseRangeBound(value, timezone string, upper bool) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	t, err := parseCalendarDate(value, timezone)
	if err != nil {
		return time.Time{}, err
	}
	if upper {
		t = t.AddDate(0, 0, 1)
	}
	return t, nil
}

// getFriendBusy godoc
// @Summary When an accepted friend is busy
// @Description Intervals only, no titles: personal and group events, invites the friend organizes or accepted. dateFrom/dateTo are RFC3339 or YYYY-MM-DD (dateTo inclusive, midnight in timezone). At most 366 days. Non-friends get 404.
// @Tags friends
// @Produce json
// @Security ApiKeyAuth
// @Param username path string true "Friend's username"
// @Param dateFrom query string true "RFC3339 or YYYY-MM-DD"
// @Param dateTo query string true "RFC3339 or YYYY-MM-DD (inclusive)"
// @Param timezone query string false "IANA zone for date-only bounds (default UTC)"
// @Success 200 {object} model.APIResponse{data=model.FriendBusyResponse}
// @Failure 400 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Router /api/v1/friends/{username}/busy [get]
func (h *Handler) getFriendBusy(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	fromRaw, toRaw := c.Query("dateFrom"), c.Query("dateTo")
	if fromRaw == "" || toRaw == "" {
		NewErrorResponse(c, http.StatusBadRequest, "dateFrom and dateTo are required")
		return
	}
	timezone := c.Query("timezone")
	from, err := parseRangeBound(fromRaw, timezone, false)
	if err != nil {
		newFieldErrorResponse(c, http.StatusBadRequest, "invalid dateFrom: "+err.Error(), "dateFrom")
		return
	}
	to, err := parseRangeBound(toRaw, timezone, true)
	if err != nil {
		newFieldErrorResponse(c, http.StatusBadRequest, "invalid dateTo: "+err.Error(), "dateTo")
		return
	}

	busy, err := h.services.Busy.FriendBusy(userID, c.Param("username"), from, to)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, model.FriendBusyResponse{Busy: busy})
}
