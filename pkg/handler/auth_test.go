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
	"strings"
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

func (m *handlerAuthRepo) UpdateUserProfile(userID int, update model.ProfileUpdate) error {
	for username, user := range m.users {
		if user.ID == userID {
			if update.Name != nil {
				user.Name = *update.Name
			}
			if update.AvatarURL != nil {
				user.AvatarURL = *update.AvatarURL
			}
			if update.AvatarColor != nil {
				user.AvatarColor = update.AvatarColor.Value
			}
			if update.AvatarFont != nil {
				user.AvatarFont = update.AvatarFont.Value
			}
			m.users[username] = user
			return nil
		}
	}
	return repository.ErrNotFound
}

func (m *handlerAuthRepo) UsernameExists(username string) (bool, error) {
	for name := range m.users {
		if strings.EqualFold(name, username) {
			return true, nil
		}
	}
	return false, nil
}

func (m *handlerAuthRepo) SearchUsers(userID int, prefix string) ([]model.UserSearchResult, error) {
	return []model.UserSearchResult{}, nil
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

func doJSON(t *testing.T, router *gin.Engine, method, path, token string, body interface{}) (*httptest.ResponseRecorder, model.APIResponse) {
	t.Helper()
	var reader *bytes.Reader
	switch b := body.(type) {
	case nil:
		reader = bytes.NewReader(nil)
	case string:
		reader = bytes.NewReader([]byte(b))
	default:
		payload, _ := json.Marshal(b)
		reader = bytes.NewReader(payload)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	router.ServeHTTP(rec, req)
	var resp model.APIResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return rec, resp
}

func signUpAndIn(t *testing.T, router *gin.Engine, username string) string {
	t.Helper()
	rec, _ := doJSON(t, router, http.MethodPost, "/auth/sign-up", "", map[string]interface{}{
		"name": "Test", "username": username, "email": username + "@test.local", "password": "secret12345",
		"avatarColor": 4, "avatarFont": "oswald",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("sign-up: %d %s", rec.Code, rec.Body.String())
	}
	rec, resp := doJSON(t, router, http.MethodPost, "/auth/sign-in", "", map[string]string{"username": username, "password": "secret12345"})
	if rec.Code != http.StatusOK {
		t.Fatalf("sign-in: %d %s", rec.Code, rec.Body.String())
	}
	return resp.Data.(map[string]interface{})["accessToken"].(string)
}

func TestSignUpDuplicateUsernameReturnsField(t *testing.T) {
	router := setupHandlerRouter(t)
	signUpAndIn(t, router, "dup")

	rec, resp := doJSON(t, router, http.MethodPost, "/auth/sign-up", "", map[string]string{
		"name": "Other", "username": "DUP", "email": "other@test.local", "password": "secret12345",
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
	if resp.Error == nil || resp.Error.Field != "username" || resp.Error.Code != http.StatusConflict {
		t.Fatalf("unexpected error body: %s", rec.Body.String())
	}
}

func TestUsernameAvailableEndpoint(t *testing.T) {
	router := setupHandlerRouter(t)
	signUpAndIn(t, router, "taken")

	cases := []struct {
		query     string
		status    int
		available bool
	}{
		{"Taken", http.StatusOK, false},
		{"free_name", http.StatusOK, true},
		{"ab", http.StatusBadRequest, false},
	}
	for _, tc := range cases {
		rec, resp := doJSON(t, router, http.MethodGet, "/auth/username-available?username="+tc.query, "", nil)
		if rec.Code != tc.status {
			t.Fatalf("%s: expected %d, got %d: %s", tc.query, tc.status, rec.Code, rec.Body.String())
		}
		if tc.status == http.StatusOK && resp.Data.(map[string]interface{})["available"] != tc.available {
			t.Fatalf("%s: unexpected body %s", tc.query, rec.Body.String())
		}
		if tc.status == http.StatusBadRequest && (resp.Error == nil || resp.Error.Field != "username") {
			t.Fatalf("%s: expected field username: %s", tc.query, rec.Body.String())
		}
	}
}

func TestUpdateProfileNullClearsAvatarStyle(t *testing.T) {
	router := setupHandlerRouter(t)
	token := signUpAndIn(t, router, "styler")

	rec, resp := doJSON(t, router, http.MethodGet, "/api/v1/users/me", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get me: %d %s", rec.Code, rec.Body.String())
	}
	me := resp.Data.(map[string]interface{})
	if me["avatarColor"] != float64(4) || me["avatarFont"] != "oswald" {
		t.Fatalf("sign-up style not returned: %s", rec.Body.String())
	}

	rec, resp = doJSON(t, router, http.MethodPatch, "/api/v1/users/me", token, `{"avatarColor": null, "avatarFont": "unbounded"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body.String())
	}
	me = resp.Data.(map[string]interface{})
	if v, ok := me["avatarColor"]; !ok || v != nil {
		t.Fatalf("avatarColor must be present and null: %s", rec.Body.String())
	}
	if me["avatarFont"] != "unbounded" || me["name"] != "Test" {
		t.Fatalf("unexpected profile: %s", rec.Body.String())
	}

	rec, resp = doJSON(t, router, http.MethodPatch, "/api/v1/users/me", token, `{"avatarColor": 9}`)
	if rec.Code != http.StatusBadRequest || resp.Error == nil || resp.Error.Field != "avatarColor" {
		t.Fatalf("expected 400 on avatarColor: %d %s", rec.Code, rec.Body.String())
	}
}
