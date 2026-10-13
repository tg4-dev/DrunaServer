package integration

import (
	"bytes"
	"druna_server/pkg/handler"
	"druna_server/pkg/repository"
	"druna_server/pkg/service"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func setupIntegration(t *testing.T) (*gin.Engine, func()) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	os.Setenv("JWT_SECRET", "integration-test-secret")

	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}

	repos := repository.NewRepository(db)
	services := service.NewService(repos)
	router := handler.NewHandler(services).InitRoutes()

	cleanup := func() {
		_, _ = db.Exec(`TRUNCATE notification_outbox, users, events, friends, group_members, groups, revoked_tokens, revoked_token_families RESTART IDENTITY CASCADE`)
		_ = db.Close()
	}

	return router, cleanup
}

func TestSignUpSignInFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, cleanup := setupIntegration(t)
	defer cleanup()

	username := fmt.Sprintf("user_%d", time.Now().UnixNano())
	body := map[string]string{
		"name":     "Integration User",
		"username": username,
		"email":    username + "@test.local",
		"password": "secret123",
	}
	payload, _ := json.Marshal(body)

	signUpReq := httptest.NewRequest(http.MethodPost, "/auth/sign-up", bytes.NewReader(payload))
	signUpReq.Header.Set("Content-Type", "application/json")
	signUpRec := httptest.NewRecorder()
	router.ServeHTTP(signUpRec, signUpReq)
	if signUpRec.Code != http.StatusOK {
		t.Fatalf("sign-up status %d: %s", signUpRec.Code, signUpRec.Body.String())
	}

	signInReq := httptest.NewRequest(http.MethodPost, "/auth/sign-in", bytes.NewReader(payload))
	signInReq.Header.Set("Content-Type", "application/json")
	signInRec := httptest.NewRecorder()
	router.ServeHTTP(signInRec, signInReq)
	if signInRec.Code != http.StatusOK {
		t.Fatalf("sign-in status %d: %s", signInRec.Code, signInRec.Body.String())
	}
}
