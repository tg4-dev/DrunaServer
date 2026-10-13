package handler

import (
	"net/http"

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
// @Tags friends
// @Produce json
// @Security ApiKeyAuth
// @Param username query string true "Username prefix"
// @Success 200 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /api/v1/friends/search [get]
func (h *Handler) searchUsers(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}
	prefix := c.Query("username")
	users, err := h.services.Authorization.SearchUsers(prefix)
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
