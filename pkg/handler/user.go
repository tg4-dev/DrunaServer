package handler

import (
	"druna_server/pkg/model"
	"net/http"

	"github.com/gin-gonic/gin"
)

// getCurrentUser godoc
// @Summary Get current user profile
// @Tags users
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Router /api/v1/users/me [get]
func (h *Handler) getCurrentUser(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}

	profile, err := h.services.Authorization.GetCurrentUser(userID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, profile)
}

// updateCurrentUser godoc
// @Summary Update current user profile
// @Tags users
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Description Absent fields are unchanged. `null` clears avatarURL, avatarColor or avatarFont.
// @Description An empty name or avatarURL is ignored (compatibility with older clients).
// @Param input body model.UpdateProfileInput true "Profile fields"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Failure 404 {object} model.APIResponse
// @Router /api/v1/users/me [patch]
func (h *Handler) updateCurrentUser(c *gin.Context) {
	userID := h.getUserIdFromToken(c)
	if userID == 0 {
		return
	}

	var input model.UpdateProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}

	profile, err := h.services.Authorization.UpdateProfile(userID, input)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, profile)
}
