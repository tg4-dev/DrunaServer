package handler

import (
	"bytes"
	"druna_server/pkg/model"
	"druna_server/pkg/repository"
	"druna_server/pkg/service"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type handlerAuthRepo struct {
	users map[string]model.User
}

func (m *handlerAuthRepo) CreateUser(user model.User) (int, error) {
	user.ID = len(m.users) + 1
	m.users[user.Username] = user
	return user.ID, nil
}

func (m *handlerAuthRepo) GetUserByUsername(username string) (model.User, error) {
	user, ok := m.users[username]
	if !ok {
		return model.User{}, repository.ErrNotFound
	}
	return user, nil
}

func (m *handlerAuthRepo) GetUserByID(userID int) (model.User, error) {
	for _, user := range m.users {
		if user.ID == userID {
			return user, nil
		}
	}
	return model.User{}, repository.ErrNotFound
}

func (m *handlerAuthRepo) UpdateUserProfile(userID int, name, avatarURL string) error {
	for username, user := range m.users {
		if user.ID == userID {
			if name != "" {
				user.Name = name
			}
			if avatarURL != "" {
				user.AvatarURL = avatarURL
			}
			m.users[username] = user
			return nil
		}
	}
	return repository.ErrNotFound
}

func (m *handlerAuthRepo) SearchUsers(prefix string) ([]model.FriendInfo, error) {
	return nil, nil
}

type handlerTokenRepo struct{}

func (m *handlerTokenRepo) ClaimRefreshToken(jti, familyID string, expiresAt time.Time) (bool, error) {
	return true, nil
}
func (m *handlerTokenRepo) RevokeFamily(familyID string, expiresAt time.Time) error { return nil }
func (m *handlerTokenRepo) IsFamilyRevoked(familyID string) (bool, error)           { return false, nil }
func (m *handlerTokenRepo) PurgeExpiredTokens() (int64, error)                      { return 0, nil }
func (m *handlerTokenRepo) Ping() error                                             { return nil }

type handlerNotificationRepo struct{}

func (m *handlerNotificationRepo) Enqueue(userID int, notificationType string, payload string) error {
	return nil
}

func setupHandlerRouter(t *testing.T) *gin.Engine {
	t.Helper()
	os.Setenv("JWT_SECRET", "handler-test-secret-key")
	gin.SetMode(gin.TestMode)

	repos := &repository.Repository{
		Authorization: &handlerAuthRepo{users: map[string]model.User{}},
		Token:         &handlerTokenRepo{},
		Notification:  &handlerNotificationRepo{},
	}
	return NewHandler(service.NewService(repos)).InitRoutes()
}

func TestSignUpRejectsShortPassword(t *testing.T) {
	router := setupHandlerRouter(t)

	body := map[string]string{
		"name":     "Test",
		"username": "shortpw",
		"email":    "shortpw@test.local",
		"password": "abc",
	}
	payload, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/sign-up", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSignInUnauthorizedWithoutUser(t *testing.T) {
	router := setupHandlerRouter(t)

	body := map[string]string{
		"username": "missing",
		"password": "secret12345",
	}
	payload, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/sign-in", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestProtectedRouteUnauthorized(t *testing.T) {
	router := setupHandlerRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
