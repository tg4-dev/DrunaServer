package handler

import (
	"druna_server/pkg/model"
	"druna_server/pkg/service"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type signUpInput struct {
	Name         string `json:"name" binding:"required"`
	Username     string `json:"username" binding:"required"`
	Email        string `json:"email" binding:"required"`
	Password     string `json:"password"`
	PasswordHash string `json:"passwordHash"`
}

func (in signUpInput) password() string {
	if in.Password != "" {
		return in.Password
	}
	return in.PasswordHash
}

// signUp godoc
// @Summary Register a new user
// @Tags auth
// @Accept json
// @Produce json
// @Param input body model.SignUpDoc true "Sign up payload"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 409 {object} model.APIResponse
// @Failure 500 {object} model.APIResponse
// @Router /auth/sign-up [post]
func (h *Handler) signUp(c *gin.Context) {
	var input signUpInput
	if err := c.BindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.password() == "" {
		NewErrorResponse(c, http.StatusBadRequest, "password is required")
		return
	}

	user := model.User{
		Name:     input.Name,
		Username: input.Username,
		Email:    input.Email,
		Password: input.password(),
	}
	id, err := h.services.Authorization.CreateUser(user)
	if err != nil {
		if errors.Is(err, service.ErrPasswordTooShort) || errors.Is(err, service.ErrPasswordTooLong) {
			NewErrorResponse(c, http.StatusBadRequest, err.Error())
			return
		}
		writeServiceError(c, err)
		return
	}

	Success(c, http.StatusOK, gin.H{"id": id})
}

type signInInput struct {
	Username     string `json:"username" binding:"required"`
	Password     string `json:"password"`
	PasswordHash string `json:"passwordHash"`
}

type renewTokenInput struct {
	RefreshToken string `json:"refreshToken"`
}

func (in signInInput) password() string {
	if in.Password != "" {
		return in.Password
	}
	return in.PasswordHash
}

// signIn godoc
// @Summary Sign in with username and password
// @Tags auth
// @Accept json
// @Produce json
// @Param input body model.SignInDoc true "Sign in payload"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /auth/sign-in [post]
func (h *Handler) signIn(c *gin.Context) {
	var input signInInput
	if err := c.BindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.password() == "" {
		NewErrorResponse(c, http.StatusBadRequest, "password is required")
		return
	}

	accessToken, refreshToken, err := h.services.Authorization.GenerateAccessRefreshToken(input.Username, input.password())
	if err != nil {
		writeServiceError(c, err)
		return
	}

	Success(c, http.StatusOK, gin.H{
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
	})
}

// renewToken godoc
// @Summary Rotate refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param input body model.RenewTokenDoc false "Refresh token in body or Authorization header"
// @Success 200 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /auth/renew-token [post]
func (h *Handler) renewToken(c *gin.Context) {
	token, ok := refreshTokenFromRequest(c)
	if !ok {
		return
	}

	accessToken, refreshToken, err := h.services.Authorization.RenewToken(token)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	Success(c, http.StatusOK, gin.H{
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
	})
}

// logout godoc
// @Summary Revoke a refresh-token family
// @Tags auth
// @Accept json
// @Produce json
// @Param input body model.RenewTokenDoc false "Refresh token in body or Authorization header"
// @Success 200 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /auth/logout [post]
func (h *Handler) logout(c *gin.Context) {
	token, ok := refreshTokenFromRequest(c)
	if !ok {
		return
	}
	if err := h.services.Authorization.Logout(token); err != nil {
		writeServiceError(c, err)
		return
	}
	Success(c, http.StatusOK, gin.H{"message": "logged out"})
}

func refreshTokenFromRequest(c *gin.Context) (string, bool) {
	var input renewTokenInput
	_ = c.ShouldBindJSON(&input)

	token := input.RefreshToken
	if token == "" {
		header := c.GetHeader(authorizationHeader)
		if header == "" {
			NewErrorResponse(c, http.StatusUnauthorized, "empty auth header or refreshToken in body")
			return "", false
		}
		headerParts := strings.Split(header, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			NewErrorResponse(c, http.StatusUnauthorized, "invalid auth header")
			return "", false
		}
		token = headerParts[1]
	}
	if token == "" {
		NewErrorResponse(c, http.StatusUnauthorized, "token is empty")
		return "", false
	}
	return token, true
}

// telegramAuth godoc
// @Summary Authenticate via Telegram Mini App initData
// @Tags auth
// @Accept json
// @Produce json
// @Param input body model.TelegramAuthDoc true "Telegram initData"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /auth/telegram [post]
func (h *Handler) telegramAuth(c *gin.Context) {
	var input struct {
		InitData string `json:"initData" binding:"required"`
	}
	if err := c.BindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}

	accessToken, refreshToken, err := h.services.Authorization.LoginWithTelegramInitData(input.InitData)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	Success(c, http.StatusOK, gin.H{
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
	})
}

// telegramWidgetAuth godoc
// @Summary Authenticate via Telegram Login Widget
// @Tags auth
// @Accept json
// @Produce json
// @Param input body model.TelegramWidgetDoc true "Telegram Login Widget fields"
// @Success 200 {object} model.APIResponse
// @Failure 400 {object} model.APIResponse
// @Failure 401 {object} model.APIResponse
// @Router /auth/telegram/widget [post]
func (h *Handler) telegramWidgetAuth(c *gin.Context) {
	var input model.TelegramWidgetDoc
	if err := c.BindJSON(&input); err != nil {
		NewErrorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}

	accessToken, refreshToken, err := h.services.Authorization.LoginWithTelegramWidget(service.TelegramWidgetData{
		ID:        input.ID,
		FirstName: input.FirstName,
		LastName:  input.LastName,
		Username:  input.Username,
		PhotoURL:  input.PhotoURL,
		AuthDate:  input.AuthDate,
		Hash:      input.Hash,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}

	Success(c, http.StatusOK, gin.H{
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
	})
}
