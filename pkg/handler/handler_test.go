package handler

import (
	"druna_server/pkg/repository"
	"druna_server/pkg/service"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type mockTokenRepo struct{}

func (mockTokenRepo) ClaimRefreshToken(string, string, time.Time) (bool, error) { return true, nil }
func (mockTokenRepo) RevokeFamily(string, time.Time) error                      { return nil }
func (mockTokenRepo) IsFamilyRevoked(string) (bool, error)                      { return false, nil }
func (mockTokenRepo) Ping() error                                               { return nil }
func (mockTokenRepo) PurgeExpiredTokens() (int64, error)                        { return 0, nil }

func TestMain(m *testing.M) {
	os.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-only")
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func TestPingEndpoint(t *testing.T) {
	repos := &repository.Repository{Token: mockTokenRepo{}}
	services := service.NewService(repos)
	h := NewHandler(services)
	router := h.InitRoutes()

	req := httptest.NewRequest(http.MethodGet, "/ping/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}
