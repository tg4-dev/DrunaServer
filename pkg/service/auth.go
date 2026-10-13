package service

import (
	"database/sql"
	"druna_server/pkg/model"
	"druna_server/pkg/repository"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

const (
	accessTokenTTL   = 12 * time.Hour
	refreshTokenTTL  = 7 * 24 * time.Hour
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
	minPasswordLen   = 8
)

var ErrPasswordTooShort = errors.New("password must be at least 8 characters")

type tokenClaims struct {
	jwt.RegisteredClaims
	UserID    int    `json:"user_id"`
	Username  string `json:"username"`
	TokenType string `json:"token_type"`
}

type telegramUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	PhotoURL  string `json:"photo_url"`
}

type AuthService struct {
	repo       repository.Authorization
	tokenRepo  repository.Token
	botToken   string
	signingKey []byte
}

func NewAuthService(repo repository.Authorization, tokenRepo repository.Token) *AuthService {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		logrus.Fatal("JWT_SECRET environment variable is required")
	}

	return &AuthService{
		repo:       repo,
		tokenRepo:  tokenRepo,
		botToken:   os.Getenv("BOT_TOKEN"),
		signingKey: []byte(secret),
	}
}

func (s *AuthService) CreateUser(user model.User) (int, error) {
	password := user.Password
	if password == "" {
		return 0, errors.New("password is required")
	}
	if len(password) < minPasswordLen {
		return 0, ErrPasswordTooShort
	}
	hash, err := hashPassword(password)
	if err != nil {
		return 0, err
	}
	user.PasswordHash = hash
	user.Password = ""
	return s.repo.CreateUser(user)
}

func (s *AuthService) GenerateAccessRefreshToken(username, password string) (string, string, error) {
	user, err := s.repo.GetUserByUsername(username)
	if err != nil {
		return "", "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", "", errors.New("invalid credentials")
	}

	return s.generateTokensForUser(user)
}

func (s *AuthService) RenewToken(refreshToken string) (string, string, error) {
	claims, err := s.parseTokenClaims(refreshToken, tokenTypeRefresh)
	if err != nil {
		return "", "", err
	}

	if claims.ID == "" {
		return "", "", errors.New("refresh token missing jti")
	}

	revoked, err := s.tokenRepo.IsTokenRevoked(claims.ID)
	if err != nil {
		return "", "", err
	}
	if revoked {
		return "", "", errors.New("refresh token has been revoked")
	}

	if claims.ExpiresAt != nil {
		if err := s.tokenRepo.RevokeToken(claims.ID, claims.ExpiresAt.Time); err != nil {
			return "", "", err
		}
	}

	user := model.User{ID: claims.UserID, Username: claims.Username}
	return s.generateTokensForUser(user)
}

func (s *AuthService) ParseAccessToken(accessToken string) (int, string, error) {
	claims, err := s.parseTokenClaims(accessToken, tokenTypeAccess)
	if err != nil {
		return 0, "", err
	}
	return claims.UserID, claims.Username, nil
}

// Deprecated: use ParseAccessToken.
func (s *AuthService) ParseToken(accessToken string) (int, string, error) {
	return s.ParseAccessToken(accessToken)
}

func (s *AuthService) GetCurrentUser(userID int) (model.UserProfile, error) {
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return model.UserProfile{}, err
	}
	return userProfileFromUser(user), nil
}

func (s *AuthService) UpdateProfile(userID int, name, avatarURL string) (model.UserProfile, error) {
	if err := s.repo.UpdateUserProfile(userID, name, avatarURL); err != nil {
		return model.UserProfile{}, err
	}
	return s.GetCurrentUser(userID)
}

func userProfileFromUser(user model.User) model.UserProfile {
	profile := model.UserProfile{
		ID:        user.ID,
		Name:      user.Name,
		Username:  user.Username,
		Email:     user.Email,
		AvatarURL: user.AvatarURL,
	}
	if user.TelegramID != nil {
		profile.TelegramID = user.TelegramID
	}
	return profile
}

func (s *AuthService) SearchUsers(prefix string) ([]model.FriendInfo, error) {
	return s.repo.SearchUsers(prefix)
}

func (s *AuthService) TelegramLogin(telegramID int64, name, username, avatarURL string) (string, string, error) {
	user, err := s.repo.GetUserByTelegramID(telegramID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return "", "", err
		}

		telegramUsername := fmt.Sprintf("tg_%d", telegramID)
		if username != "" {
			telegramUsername = username
		}

		randomPassword, err := hashPassword(uuid.New().String())
		if err != nil {
			return "", "", err
		}

		tgID := telegramID
		newUser := model.User{
			Name:         name,
			Username:     telegramUsername,
			PasswordHash: randomPassword,
			Email:        fmt.Sprintf("%s@telegram.local", telegramUsername),
			AvatarURL:    avatarURL,
			TelegramID:   &tgID,
		}

		id, err := s.repo.CreateUser(newUser)
		if err != nil {
			return "", "", err
		}

		user = newUser
		user.ID = id
	} else if avatarURL != "" && user.AvatarURL == "" {
		_ = s.repo.UpdateUserProfile(user.ID, "", avatarURL)
		user.AvatarURL = avatarURL
	}

	return s.generateTokensForUser(user)
}

func (s *AuthService) LoginWithTelegramInitData(initData string) (string, string, error) {
	if s.botToken == "" {
		return "", "", errors.New("BOT_TOKEN is not configured")
	}

	data, err := parseInitData(initData, s.botToken)
	if err != nil {
		return "", "", err
	}

	userJSON, ok := data["user"]
	if !ok || userJSON == "" {
		return "", "", errors.New("missing user in init data")
	}

	var tgUser telegramUser
	if err := json.Unmarshal([]byte(userJSON), &tgUser); err != nil {
		return "", "", fmt.Errorf("invalid user data: %w", err)
	}

	name := tgUser.FirstName
	if tgUser.LastName != "" {
		name = name + " " + tgUser.LastName
	}

	return s.TelegramLogin(tgUser.ID, name, tgUser.Username, tgUser.PhotoURL)
}

func (s *AuthService) generateTokensForUser(user model.User) (string, string, error) {
	accessToken, err := s.generateToken(accessTokenTTL, user, tokenTypeAccess)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := s.generateToken(refreshTokenTTL, user, tokenTypeRefresh)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func (s *AuthService) generateToken(tokenTTL time.Duration, user model.User, tokenType string) (string, error) {
	jti := uuid.New().String()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID:    user.ID,
		Username:  user.Username,
		TokenType: tokenType,
	})
	return token.SignedString(s.signingKey)
}

func (s *AuthService) parseTokenClaims(tokenStr, expectedType string) (*tokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &tokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return s.signingKey, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*tokenClaims)
	if !ok {
		return nil, errors.New("token claims are not of type *tokenClaims")
	}
	if claims.TokenType != expectedType {
		return nil, fmt.Errorf("expected %s token", expectedType)
	}
	return claims, nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
