package integration

import (
	"druna_server/pkg/model"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEventTimeZonePreserved(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	access, _ := signUpAndSignIn(t, router, fmt.Sprintf("tz_%d", time.Now().UnixNano()), "secret12345")

	rec := doJSON(t, router, http.MethodPost, "/api/v1/events/", map[string]string{
		"title":     "Moscow meeting",
		"startTime": "2026-06-17T10:00:00+03:00",
		"endTime":   "2026-06-17T11:00:00+03:00",
	}, access)
	if rec.Code != http.StatusOK {
		t.Fatalf("create event failed: %d %s", rec.Code, rec.Body.String())
	}

	listRec := doJSON(t, router, http.MethodGet, "/api/v1/events/", nil, access)
	list := decodeData[model.EventListResponse](t, listRec)
	if len(list.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(list.Events))
	}
	want := time.Date(2026, 6, 17, 7, 0, 0, 0, time.UTC)
	if !list.Events[0].StartTime.Equal(want) {
		t.Fatalf("start time shifted: got %s, want %s", list.Events[0].StartTime, want)
	}
}

func TestEmptyListsAreArrays(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	access, _ := signUpAndSignIn(t, router, fmt.Sprintf("empty_%d", time.Now().UnixNano()), "secret12345")

	for _, tc := range []struct{ path, key string }{
		{"/api/v1/friends/list", `"friends":[]`},
		{"/api/v1/friends/requests/incoming", `"friends":[]`},
		{"/api/v1/groups/list", `"groups":[]`},
		{"/api/v1/events/", `"events":[]`},
		{"/api/v1/friends/search?username=zzz_nobody", `"users":[]`},
	} {
		rec := doJSON(t, router, http.MethodGet, tc.path, nil, access)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d %s", tc.path, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), tc.key) {
			t.Fatalf("%s: expected %s in %s", tc.path, tc.key, rec.Body.String())
		}
	}
}

func TestDuplicateSignUpConflict(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	username := fmt.Sprintf("dup_%d", time.Now().UnixNano())
	signUpAndSignIn(t, router, username, "secret12345")

	rec := doJSON(t, router, http.MethodPost, "/auth/sign-up", map[string]string{
		"name":     "Dup",
		"username": username,
		"email":    "other_" + username + "@test.local",
		"password": "secret12345",
	}, "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "pq:") {
		t.Fatalf("raw database error leaked: %s", rec.Body.String())
	}
}

func TestSignInDoesNotRevealUnknownUser(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	username := fmt.Sprintf("enum_%d", time.Now().UnixNano())
	signUpAndSignIn(t, router, username, "secret12345")

	unknown := doJSON(t, router, http.MethodPost, "/auth/sign-in", map[string]string{"username": "nobody_" + username, "password": "secret12345"}, "")
	wrong := doJSON(t, router, http.MethodPost, "/auth/sign-in", map[string]string{"username": username, "password": "wrong-password"}, "")
	if unknown.Code != http.StatusUnauthorized || wrong.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401/401, got %d/%d", unknown.Code, wrong.Code)
	}
	if unknown.Body.String() != wrong.Body.String() {
		t.Fatalf("responses differ: %s vs %s", unknown.Body.String(), wrong.Body.String())
	}
}

func TestSearchEscapesWildcards(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	access, _ := signUpAndSignIn(t, router, fmt.Sprintf("search_%d", time.Now().UnixNano()), "secret12345")

	for _, q := range []string{"%25", "_"} {
		rec := doJSON(t, router, http.MethodGet, "/api/v1/friends/search?username="+q, nil, access)
		users := decodeData[map[string][]model.FriendInfo](t, rec)
		if len(users["users"]) != 0 {
			t.Fatalf("query %q matched %d users; wildcards must be literal", q, len(users["users"]))
		}
	}
}

func TestSenderCannotAcceptOwnRequest(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	userA := fmt.Sprintf("own_a_%d", time.Now().UnixNano())
	userB := fmt.Sprintf("own_b_%d", time.Now().UnixNano())
	accessA, _ := signUpAndSignIn(t, router, userA, "secret12345")
	signUpAndSignIn(t, router, userB, "secret12345")

	doJSON(t, router, http.MethodPost, "/api/v1/friends/request", map[string]string{"username": userB}, accessA)
	rec := doJSON(t, router, http.MethodPost, "/api/v1/friends/accept", map[string]string{"username": userB}, accessA)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "only the recipient") {
		t.Fatalf("expected 403 only the recipient can accept, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestUnknownUsernameIsNotFound(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	access, _ := signUpAndSignIn(t, router, fmt.Sprintf("nf_%d", time.Now().UnixNano()), "secret12345")
	rec := doJSON(t, router, http.MethodPost, "/api/v1/friends/request", map[string]string{"username": "definitely_missing_user"}, access)
	if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "sql:") {
		t.Fatalf("expected clean 404, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestConcurrentRefreshOnlyOneSucceeds(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	_, refresh := signUpAndSignIn(t, router, fmt.Sprintf("race_%d", time.Now().UnixNano()), "secret12345")

	const workers = 8
	codes := make([]int, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = doJSON(t, router, http.MethodPost, "/auth/renew-token", map[string]string{"refreshToken": refresh}, "").Code
		}(i)
	}
	wg.Wait()

	ok := 0
	for _, code := range codes {
		if code == http.StatusOK {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("expected exactly one successful rotation, got %d (%v)", ok, codes)
	}
}

func TestConcurrentOverlappingEventsOnlyOneCreated(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	access, _ := signUpAndSignIn(t, router, fmt.Sprintf("ovl_%d", time.Now().UnixNano()), "secret12345")

	const workers = 8
	codes := make([]int, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = doJSON(t, router, http.MethodPost, "/api/v1/events/", map[string]string{
				"title":     fmt.Sprintf("slot %d", i),
				"startTime": "2026-06-17T10:00:00Z",
				"endTime":   "2026-06-17T11:00:00Z",
			}, access).Code
		}(i)
	}
	wg.Wait()

	ok := 0
	for _, code := range codes {
		if code == http.StatusOK {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("expected exactly one event created, got %d (%v)", ok, codes)
	}
}
