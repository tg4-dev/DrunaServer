package service

import (
	"druna_server/pkg/model"
	"druna_server/pkg/repository"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

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
	maxNameLen       = 255
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

type AuthService struct {
	repo       repository.Authorization
	tokenRepo  repository.Token
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
	if err := validateAvatarStyle(user.AvatarColor, user.AvatarFont); err != nil {
		return 0, err
	}
	taken, err := s.repo.UsernameExists(user.Username)
	if err != nil {
		return 0, err
	}
	if taken {
		return 0, ErrUsernameTaken
	}

	user.PasswordHash = hash
	user.Password = ""
	user.ID = 0
	user.AvatarURL = ""
	id, err := s.repo.CreateUser(user)
	var dup *repository.DuplicateError
	if errors.As(err, &dup) {
		switch dup.Field {
		case "username":
			return 0, ErrUsernameTaken
		case "email":
			return 0, Conflict("email already taken").WithField("email")
		}
	}
	if errors.Is(err, repository.ErrDuplicate) {
		return 0, Conflict("username or email already taken")
	}
	return id, err
}

var usernamePattern = regexp.MustCompile(`^[a-z0-9_.]{3,32}$`)

// UsernameAvailable reports whether nobody holds the username, ignoring case.
// The username must match the client rules: 3-32 characters of a-z, 0-9, "_" and ".".
func (s *AuthService) UsernameAvailable(username string) (bool, error) {
	if !usernamePattern.MatchString(strings.ToLower(username)) {
		return false, Validation("username must be 3-32 characters: a-z, 0-9, _ and .").WithField("username")
	}
	taken, err := s.repo.UsernameExists(username)
	if err != nil {
		return false, err
	}
	return !taken, nil
}

func validateAvatarStyle(color *int, font *string) error {
	if color != nil && !model.IsAvatarColor(*color) {
		return Validation(fmt.Sprintf("avatarColor must be between 0 and %d", model.AvatarColorCount-1)).WithField("avatarColor")
	}
	if font != nil && !model.IsAvatarFont(*font) {
		return Validation("avatarFont must be one of: " + strings.Join(model.AvatarFonts, ", ")).WithField("avatarFont")
	}
	return nil
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

func (s *AuthService) UpdateProfile(userID int, input model.UpdateProfileInput) (model.UserProfile, error) {
	var update model.ProfileUpdate

	if input.Name.Set && input.Name.Value != nil && *input.Name.Value != "" {
		name := strings.TrimSpace(*input.Name.Value)
		if name == "" {
			return model.UserProfile{}, Validation("name must not be blank").WithField("name")
		}
		if utf8.RuneCountInString(name) > maxNameLen {
			return model.UserProfile{}, Validation(fmt.Sprintf("name must be at most %d characters", maxNameLen)).WithField("name")
		}
		update.Name = &name
	} else if input.Name.Set && input.Name.Value == nil {
		return model.UserProfile{}, Validation("name cannot be removed").WithField("name")
	}

	if input.AvatarURL.Set {
		switch {
		case input.AvatarURL.Value == nil:
			empty := ""
			update.AvatarURL = &empty
		case *input.AvatarURL.Value != "":
			avatarURL := sanitizeAvatarURL(*input.AvatarURL.Value)
			if avatarURL == "" {
				return model.UserProfile{}, Validation("avatarURL must be an https URL").WithField("avatarURL")
			}
			update.AvatarURL = &avatarURL
		}
	}

	if input.AvatarColor.Set {
		if err := validateAvatarStyle(input.AvatarColor.Value, nil); err != nil {
			return model.UserProfile{}, err
		}
		update.AvatarColor = &input.AvatarColor
	}
	if input.AvatarFont.Set {
		if err := validateAvatarStyle(nil, input.AvatarFont.Value); err != nil {
			return model.UserProfile{}, err
		}
		update.AvatarFont = &input.AvatarFont
	}

	if err := s.repo.UpdateUserProfile(userID, update); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.UserProfile{}, NotFound("user not found")
		}
		return model.UserProfile{}, err
	}
	return s.GetCurrentUser(userID)
}

func userProfileFromUser(user model.User) model.UserProfile {
	return model.UserProfile{
		ID:          user.ID,
		Name:        user.Name,
		Username:    user.Username,
		Email:       user.Email,
		AvatarStyle: user.AvatarStyle,
	}
}

func (s *AuthService) SearchUsers(userID int, prefix string) ([]model.UserSearchResult, error) {
	return s.repo.SearchUsers(userID, prefix)
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
