package integration

import (
	"bytes"
	"druna_server/pkg/model"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type apiEnvelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

func doJSON(t *testing.T, router http.Handler, method, path string, body interface{}, authToken string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decodeData[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var envelope apiEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v body=%s", err, rec.Body.String())
	}
	var out T
	if err := json.Unmarshal(envelope.Data, &out); err != nil {
		t.Fatalf("decode data: %v body=%s", err, rec.Body.String())
	}
	return out
}

func signUpAndSignIn(t *testing.T, router http.Handler, username, password string) (accessToken, refreshToken string) {
	t.Helper()
	body := map[string]string{
		"name":     "Test User",
		"username": username,
		"email":    username + "@test.local",
		"password": password,
	}
	signUpRec := doJSON(t, router, http.MethodPost, "/auth/sign-up", body, "")
	if signUpRec.Code != http.StatusOK {
		t.Fatalf("sign-up status %d: %s", signUpRec.Code, signUpRec.Body.String())
	}

	signInRec := doJSON(t, router, http.MethodPost, "/auth/sign-in", body, "")
	if signInRec.Code != http.StatusOK {
		t.Fatalf("sign-in status %d: %s", signInRec.Code, signInRec.Body.String())
	}
	tokens := decodeData[map[string]string](t, signInRec)
	return tokens["accessToken"], tokens["refreshToken"]
}

func TestProtectedRouteWithoutJWT(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	rec := doJSON(t, router, http.MethodGet, "/api/v1/events/", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRefreshTokenRotation(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	username := fmt.Sprintf("refresh_%d", time.Now().UnixNano())
	access, refresh := signUpAndSignIn(t, router, username, "secret12345")
	if access == "" || refresh == "" {
		t.Fatal("expected tokens")
	}

	renewRec := doJSON(t, router, http.MethodPost, "/auth/renew-token", map[string]string{
		"refreshToken": refresh,
	}, "")
	if renewRec.Code != http.StatusOK {
		t.Fatalf("renew status %d: %s", renewRec.Code, renewRec.Body.String())
	}
	newTokens := decodeData[map[string]string](t, renewRec)

	reuseRec := doJSON(t, router, http.MethodPost, "/auth/renew-token", map[string]string{
		"refreshToken": refresh,
	}, "")
	if reuseRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected old refresh to fail, got %d", reuseRec.Code)
	}

	eventsRec := doJSON(t, router, http.MethodGet, "/api/v1/events/", nil, newTokens["accessToken"])
	if eventsRec.Code != http.StatusOK {
		t.Fatalf("events with new access token failed: %d", eventsRec.Code)
	}
}

func TestRefreshTokenRejectedOnAPI(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	username := fmt.Sprintf("refresh_api_%d", time.Now().UnixNano())
	_, refresh := signUpAndSignIn(t, router, username, "secret12345")

	rec := doJSON(t, router, http.MethodGet, "/api/v1/events/", nil, refresh)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected refresh token rejected on API, got %d", rec.Code)
	}
}

func TestFriendLifecycle(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	userA := fmt.Sprintf("friend_a_%d", time.Now().UnixNano())
	userB := fmt.Sprintf("friend_b_%d", time.Now().UnixNano())
	accessA, _ := signUpAndSignIn(t, router, userA, "secret12345")
	accessB, _ := signUpAndSignIn(t, router, userB, "secret12345")

	requestRec := doJSON(t, router, http.MethodPost, "/api/v1/friends/request", map[string]string{
		"username": userB,
	}, accessA)
	if requestRec.Code != http.StatusOK {
		t.Fatalf("friend request failed: %d %s", requestRec.Code, requestRec.Body.String())
	}

	acceptRec := doJSON(t, router, http.MethodPost, "/api/v1/friends/accept", map[string]string{
		"username": userA,
	}, accessB)
	if acceptRec.Code != http.StatusOK {
		t.Fatalf("accept failed: %d %s", acceptRec.Code, acceptRec.Body.String())
	}

	listRec := doJSON(t, router, http.MethodGet, "/api/v1/friends/list", nil, accessA)
	if listRec.Code != http.StatusOK {
		t.Fatalf("friend list failed: %d", listRec.Code)
	}
	friends := decodeData[map[string][]model.FriendInfo](t, listRec)
	if len(friends["friends"]) != 1 {
		t.Fatalf("expected 1 friend, got %d", len(friends["friends"]))
	}

	deleteRec := doJSON(t, router, http.MethodDelete, "/api/v1/friends/", map[string]string{
		"username": userB,
	}, accessA)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete friend failed: %d", deleteRec.Code)
	}
}

func TestEventOverlapRejected(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	username := fmt.Sprintf("event_%d", time.Now().UnixNano())
	access, _ := signUpAndSignIn(t, router, username, "secret12345")

	firstRec := doJSON(t, router, http.MethodPost, "/api/v1/events/", map[string]string{
		"title":     "Busy",
		"startTime": "2026-06-17T10:00:00Z",
		"endTime":   "2026-06-17T11:00:00Z",
		"type":      "work",
	}, access)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("create event failed: %d %s", firstRec.Code, firstRec.Body.String())
	}

	secondRec := doJSON(t, router, http.MethodPost, "/api/v1/events/", map[string]string{
		"title":     "Overlap",
		"startTime": "2026-06-17T10:30:00Z",
		"endTime":   "2026-06-17T11:30:00Z",
		"type":      "work",
	}, access)
	if secondRec.Code == http.StatusOK {
		t.Fatal("expected overlapping event to be rejected")
	}
}

func TestGroupOwnerRulesAndFriendPolicy(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	ownerName := fmt.Sprintf("owner_%d", time.Now().UnixNano())
	memberName := fmt.Sprintf("member_%d", time.Now().UnixNano())
	strangerName := fmt.Sprintf("stranger_%d", time.Now().UnixNano())

	ownerAccess, _ := signUpAndSignIn(t, router, ownerName, "secret12345")
	memberAccess, _ := signUpAndSignIn(t, router, memberName, "secret12345")
	strangerAccess, _ := signUpAndSignIn(t, router, strangerName, "secret12345")

	doJSON(t, router, http.MethodPost, "/api/v1/friends/request", map[string]string{"username": memberName}, ownerAccess)
	doJSON(t, router, http.MethodPost, "/api/v1/friends/accept", map[string]string{"username": ownerName}, memberAccess)

	createRec := doJSON(t, router, http.MethodPost, "/api/v1/groups/create", map[string]string{
		"name": "Test Group",
	}, ownerAccess)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create group failed: %d %s", createRec.Code, createRec.Body.String())
	}
	groupData := decodeData[map[string]interface{}](t, createRec)
	groupID, ok := groupData["groupId"].(float64)
	if !ok {
		t.Fatalf("expected groupId in response: %+v", groupData)
	}

	addStrangerRec := doJSON(t, router, http.MethodPost, fmt.Sprintf("/api/v1/groups/%d/members", int(groupID)), map[string]string{
		"username": strangerName,
	}, ownerAccess)
	if addStrangerRec.Code == http.StatusOK {
		t.Fatal("expected non-friend add to group to fail")
	}

	addMemberRec := doJSON(t, router, http.MethodPost, fmt.Sprintf("/api/v1/groups/%d/members", int(groupID)), map[string]string{
		"username": memberName,
	}, ownerAccess)
	if addMemberRec.Code != http.StatusOK {
		t.Fatalf("add friend member failed: %d %s", addMemberRec.Code, addMemberRec.Body.String())
	}

	deleteRec := doJSON(t, router, http.MethodDelete, fmt.Sprintf("/api/v1/groups/%d", int(groupID)), nil, memberAccess)
	if deleteRec.Code == http.StatusOK {
		t.Fatal("non-owner should not delete group")
	}

	deleteOwnerRec := doJSON(t, router, http.MethodDelete, fmt.Sprintf("/api/v1/groups/%d", int(groupID)), nil, ownerAccess)
	if deleteOwnerRec.Code != http.StatusOK {
		t.Fatalf("owner delete failed: %d", deleteOwnerRec.Code)
	}

	_ = strangerAccess
}

func TestUserProfile(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	username := fmt.Sprintf("profile_%d", time.Now().UnixNano())
	access, _ := signUpAndSignIn(t, router, username, "secret12345")

	meRec := doJSON(t, router, http.MethodGet, "/api/v1/users/me", nil, access)
	if meRec.Code != http.StatusOK {
		t.Fatalf("get profile failed: %d %s", meRec.Code, meRec.Body.String())
	}
	profile := decodeData[model.UserProfile](t, meRec)
	if profile.Username != username {
		t.Fatalf("expected username %s, got %s", username, profile.Username)
	}

	patchRec := doJSON(t, router, http.MethodPatch, "/api/v1/users/me", map[string]string{
		"name":      "Updated Name",
		"avatarURL": "https://example.com/avatar.png",
	}, access)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch profile failed: %d %s", patchRec.Code, patchRec.Body.String())
	}
	updated := decodeData[model.UserProfile](t, patchRec)
	if updated.Name != "Updated Name" || updated.AvatarURL != "https://example.com/avatar.png" {
		t.Fatalf("unexpected profile update: %+v", updated)
	}
}

func TestFriendCounterRequestAndResendAfterReject(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	userA := fmt.Sprintf("pair_a_%d", time.Now().UnixNano())
	userB := fmt.Sprintf("pair_b_%d", time.Now().UnixNano())
	accessA, _ := signUpAndSignIn(t, router, userA, "secret12345")
	accessB, _ := signUpAndSignIn(t, router, userB, "secret12345")

	first := doJSON(t, router, http.MethodPost, "/api/v1/friends/request", map[string]string{"username": userB}, accessA)
	if first.Code != http.StatusOK {
		t.Fatalf("first request: %d %s", first.Code, first.Body.String())
	}
	counter := doJSON(t, router, http.MethodPost, "/api/v1/friends/request", map[string]string{"username": userA}, accessB)
	if counter.Code != http.StatusConflict {
		t.Fatalf("expected 409 for counter request, got %d %s", counter.Code, counter.Body.String())
	}

	reject := doJSON(t, router, http.MethodPost, "/api/v1/friends/reject", map[string]string{"username": userA}, accessB)
	if reject.Code != http.StatusOK {
		t.Fatalf("reject: %d %s", reject.Code, reject.Body.String())
	}
	again := doJSON(t, router, http.MethodPost, "/api/v1/friends/request", map[string]string{"username": userB}, accessA)
	if again.Code != http.StatusOK {
		t.Fatalf("resend after reject: %d %s", again.Code, again.Body.String())
	}
}

func TestParallelOverlappingEvents(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	username := fmt.Sprintf("race_%d", time.Now().UnixNano())
	access, _ := signUpAndSignIn(t, router, username, "secret12345")

	body := map[string]string{
		"title":     "Race",
		"startTime": "2026-07-01T10:00:00Z",
		"endTime":   "2026-07-01T11:00:00Z",
		"type":      "work",
	}
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rec := doJSON(t, router, http.MethodPost, "/api/v1/events/", body, access)
			codes[i] = rec.Code
		}(i)
	}
	wg.Wait()

	okCount := 0
	for _, code := range codes {
		if code == http.StatusOK {
			okCount++
		}
	}
	if okCount != 1 {
		t.Fatalf("expected exactly one overlapping create to succeed, got %v", codes)
	}
}
