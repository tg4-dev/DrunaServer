package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestValidRequestID(t *testing.T) {
	cases := map[string]bool{
		"":                          false,
		"abc-123_DEF.4":             true,
		"has space":                 false,
		"line\nbreak":               false,
		strings.Repeat("a", 128):    true,
		strings.Repeat("a", 129):    false,
		"550e8400-e29b-41d4-a716-4": true,
	}
	for id, want := range cases {
		if got := validRequestID(id); got != want {
			t.Errorf("validRequestID(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestRequestIDReplacedWhenInvalid(t *testing.T) {
	router := gin.New()
	router.Use(requestIDMiddleware())
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "bad id\twith junk")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	got := rec.Header().Get("X-Request-ID")
	if got == "" || got == "bad id\twith junk" {
		t.Fatalf("expected a generated request id, got %q", got)
	}
}
