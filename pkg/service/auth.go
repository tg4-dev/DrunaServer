package service

import (
	"druna_server/pkg/model"
	"druna_server/pkg/repository"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

const (
	accessTokenTTL   = 15 * time.Minute
	refreshTokenTTL  = 7 * 24 * time.Hour
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
	minPasswordLen   = 8
	maxPasswordLen   = 72
)

var dummyPasswordHash []byte

func init() {
	hash, err := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	dummyPasswordHash = hash
}

type tokenClaims struct {
	jwt.RegisteredClaims
	UserID    int    `json:"user_id"`
	Username  string `json:"username"`
	TokenType string `json:"token_type"`
	FamilyID  string `json:"fid"`
}

type telegramUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	PhotoURL  string `json:"photo_url"`
}

type TelegramWidgetData struct {
	ID        int64
	FirstName string
	LastName  string
	Username  string
	PhotoURL  string
	AuthDate  int64
	Hash      string
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
		return 0, Validation("password is required")
	}
	if len(password) < minPasswordLen {
		return 0, ErrPasswordTooShort
	}
	if len(password) > maxPasswordLen {
		return 0, ErrPasswordTooLong
	}
	hash, err := hashPassword(password)
	if err != nil {
		return 0, err
	}
	user.PasswordHash = hash
	user.Password = ""
	user.ID = 0
	user.TelegramID = nil
	user.AvatarURL = ""
	id, err := s.repo.CreateUser(user)
	if errors.Is(err, repository.ErrDuplicate) {
		return 0, Conflict("username or email already taken")
	}
	return id, err
}

func (s *AuthService) GenerateAccessRefreshToken(username, password string) (string, string, error) {
	user, err := s.repo.GetUserByUsername(username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
			return "", "", ErrInvalidCredentials
		}
		return "", "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", "", ErrInvalidCredentials
	}

	return s.generateTokensForUser(user, "")
}

func (s *AuthService) RenewToken(refreshToken string) (string, string, error) {
	claims, err := s.parseTokenClaims(refreshToken, tokenTypeRefresh)
	if err != nil {
		return "", "", Unauthorized("invalid token")
	}
	if claims.ID == "" || claims.FamilyID == "" {
		return "", "", Unauthorized("invalid token")
	}

	revoked, err := s.tokenRepo.IsFamilyRevoked(claims.FamilyID)
	if err != nil {
		return "", "", err
	}
	if revoked {
		return "", "", Unauthorized("refresh token has been revoked")
	}

	exp := time.Now().Add(refreshTokenTTL)
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Time
	}
	claimed, err := s.tokenRepo.ClaimRefreshToken(claims.ID, claims.FamilyID, exp)
	if err != nil {
		return "", "", err
	}
	if !claimed {
		if err := s.tokenRepo.RevokeFamily(claims.FamilyID, exp); err != nil {
			return "", "", err
		}
		return "", "", Unauthorized("refresh token has been revoked")
	}

	user := model.User{ID: claims.UserID, Username: claims.Username}
	return s.generateTokensForUser(user, claims.FamilyID)
}

func (s *AuthService) Logout(refreshToken string) error {
	claims, err := s.parseTokenClaims(refreshToken, tokenTypeRefresh)
	if err != nil {
		return Unauthorized("invalid token")
	}
	if claims.FamilyID == "" {
		return Unauthorized("invalid token")
	}
	exp := time.Now().Add(refreshTokenTTL)
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Time
	}
	if err := s.tokenRepo.RevokeFamily(claims.FamilyID, exp); err != nil {
		return err
	}
	if claims.ID != "" {
		_, err = s.tokenRepo.ClaimRefreshToken(claims.ID, claims.FamilyID, exp)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *AuthService) ParseAccessToken(accessToken string) (int, string, error) {
	claims, err := s.parseTokenClaims(accessToken, tokenTypeAccess)
	if err != nil {
		return 0, "", Unauthorized("invalid token")
	}
	return claims.UserID, claims.Username, nil
}

// Deprecated: use ParseAccessToken.
func (s *AuthService) ParseToken(accessToken string) (int, string, error) {
	return s.ParseAccessToken(accessToken)
}

func (s *AuthService) GetCurrentUser(userID int) (model.UserProfile, error) {
	user, err := s.repo.GetUserByID(userID)
	if errors.Is(err, repository.ErrNotFound) {
		return model.UserProfile{}, NotFound("user not found")
	}
	if err != nil {
		return model.UserProfile{}, err
	}
	return userProfileFromUser(user), nil
}

func (s *AuthService) UpdateProfile(userID int, name, avatarURL string) (model.UserProfile, error) {
	if avatarURL != "" {
		avatarURL = sanitizeAvatarURL(avatarURL)
		if avatarURL == "" {
			return model.UserProfile{}, Validation("avatarURL must be an https URL")
		}
	}
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
	avatarURL = sanitizeAvatarURL(avatarURL)
	user, err := s.repo.GetUserByTelegramID(telegramID)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
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
		if errors.Is(err, repository.ErrDuplicate) {
			fallbackUser := fmt.Sprintf("tg_%d", telegramID)
			fallbackEmail := fmt.Sprintf("%s@telegram.local", fallbackUser)
			if newUser.Username == fallbackUser && newUser.Email == fallbackEmail {
				return "", "", err
			}
			newUser.Username = fallbackUser
			newUser.Email = fallbackEmail
			id, err = s.repo.CreateUser(newUser)
		}
		if err != nil {
			return "", "", err
		}

		user = newUser
		user.ID = id
	} else if avatarURL != "" && user.AvatarURL == "" {
		if err := s.repo.UpdateUserProfile(user.ID, "", avatarURL); err != nil {
			logrus.WithError(err).WithField("user_id", user.ID).Error("failed to update telegram avatar")
		} else {
			user.AvatarURL = avatarURL
		}
	}

	return s.generateTokensForUser(user, "")
}

func (s *AuthService) LoginWithTelegramInitData(initData string) (string, string, error) {
	if s.botToken == "" {
		return "", "", errors.New("BOT_TOKEN is not configured")
	}

	data, err := verifyWebAppInitData(initData, s.botToken)
	if err != nil {
		return "", "", Unauthorized(err.Error())
	}

	userJSON, ok := data["user"]
	if !ok || userJSON == "" {
		return "", "", Unauthorized("missing user in init data")
	}

	var tgUser telegramUser
	if err := json.Unmarshal([]byte(userJSON), &tgUser); err != nil {
		return "", "", Unauthorized("invalid user data")
	}

	name := tgUser.FirstName
	if tgUser.LastName != "" {
		name = name + " " + tgUser.LastName
	}

	return s.TelegramLogin(tgUser.ID, name, tgUser.Username, tgUser.PhotoURL)
}

func (s *AuthService) LoginWithTelegramWidget(data TelegramWidgetData) (string, string, error) {
	if s.botToken == "" {
		return "", "", errors.New("BOT_TOKEN is not configured")
	}
	fields := map[string]string{
		"id":         fmt.Sprintf("%d", data.ID),
		"auth_date":  fmt.Sprintf("%d", data.AuthDate),
		"first_name": data.FirstName,
	}
	if data.LastName != "" {
		fields["last_name"] = data.LastName
	}
	if data.Username != "" {
		fields["username"] = data.Username
	}
	if data.PhotoURL != "" {
		fields["photo_url"] = data.PhotoURL
	}
	if err := verifyLoginWidget(fields, data.Hash, s.botToken); err != nil {
		return "", "", Unauthorized(err.Error())
	}
	name := data.FirstName
	if data.LastName != "" {
		name = name + " " + data.LastName
	}
	return s.TelegramLogin(data.ID, name, data.Username, data.PhotoURL)
}

func sanitizeAvatarURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return ""
	}
	return raw
}

func (s *AuthService) generateTokensForUser(user model.User, familyID string) (string, string, error) {
	if familyID == "" {
		familyID = uuid.New().String()
	}
	accessToken, err := s.generateToken(accessTokenTTL, user, tokenTypeAccess, familyID)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := s.generateToken(refreshTokenTTL, user, tokenTypeRefresh, familyID)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func (s *AuthService) generateToken(tokenTTL time.Duration, user model.User, tokenType, familyID string) (string, error) {
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
		FamilyID:  familyID,
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
