package integration

import (
	"druna_server/pkg/model"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func errorField(t *testing.T, body []byte) string {
	t.Helper()
	var env struct {
		Error *model.ErrorBody `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil || env.Error == nil {
		t.Fatalf("expected error envelope: %s", body)
	}
	return env.Error.Field
}

func TestSignUpConflictNamesField(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	username := fmt.Sprintf("field_%d", time.Now().UnixNano())
	signUpAndSignIn(t, router, username, "secret12345")

	sameEmail := doJSON(t, router, http.MethodPost, "/auth/sign-up", map[string]string{
		"name": "X", "username": "other_" + username, "email": username + "@test.local", "password": "secret12345",
	}, "")
	if sameEmail.Code != http.StatusConflict || errorField(t, sameEmail.Body.Bytes()) != "email" {
		t.Fatalf("expected 409 email, got %d %s", sameEmail.Code, sameEmail.Body.String())
	}

	sameUsernameOtherCase := doJSON(t, router, http.MethodPost, "/auth/sign-up", map[string]string{
		"name": "X", "username": "FIELD_" + username[len("field_"):], "email": "x_" + username + "@test.local", "password": "secret12345",
	}, "")
	if sameUsernameOtherCase.Code != http.StatusConflict || errorField(t, sameUsernameOtherCase.Body.Bytes()) != "username" {
		t.Fatalf("expected 409 username, got %d %s", sameUsernameOtherCase.Code, sameUsernameOtherCase.Body.String())
	}
}

func TestUsernameAvailability(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	username := fmt.Sprintf("avail_%d", time.Now().UnixNano())
	signUpAndSignIn(t, router, username, "secret12345")

	taken := decodeData[model.UsernameAvailability](t, doJSON(t, router, http.MethodGet, "/auth/username-available?username=AVAIL_"+username[len("avail_"):], nil, ""))
	free := decodeData[model.UsernameAvailability](t, doJSON(t, router, http.MethodGet, "/auth/username-available?username=free."+username, nil, ""))
	if taken.Available || !free.Available {
		t.Fatalf("taken=%+v free=%+v", taken, free)
	}
}

func TestProfileAvatarStyleRoundTrip(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	access, _ := signUpAndSignIn(t, router, fmt.Sprintf("style_%d", time.Now().UnixNano()), "secret12345")

	rec := doJSON(t, router, http.MethodPatch, "/api/v1/users/me", map[string]interface{}{
		"avatarURL": "https://example.com/a.png", "avatarColor": 7, "avatarFont": "comfortaa",
	}, access)
	profile := decodeData[model.UserProfile](t, rec)
	if profile.AvatarURL != "https://example.com/a.png" || profile.AvatarColor == nil || *profile.AvatarColor != 7 ||
		profile.AvatarFont == nil || *profile.AvatarFont != "comfortaa" {
		t.Fatalf("unexpected profile: %s", rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPatch, "/api/v1/users/me", map[string]interface{}{
		"avatarURL": nil, "avatarColor": nil,
	}, access)
	profile = decodeData[model.UserProfile](t, rec)
	if profile.AvatarURL != "" || profile.AvatarColor != nil || profile.AvatarFont == nil || *profile.AvatarFont != "comfortaa" {
		t.Fatalf("null must clear only the given fields: %s", rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPatch, "/api/v1/users/me", map[string]interface{}{}, access)
	if rec.Code != http.StatusOK {
		t.Fatalf("empty patch: %d %s", rec.Code, rec.Body.String())
	}
}

func TestSearchRelationsAndAvatars(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	prefix := fmt.Sprintf("rel%d_", time.Now().UnixNano())
	me, _ := signUpAndSignIn(t, router, prefix+"me", "secret12345")
	friend, _ := signUpAndSignIn(t, router, prefix+"friend", "secret12345")
	incoming, _ := signUpAndSignIn(t, router, prefix+"incoming", "secret12345")
	signUpAndSignIn(t, router, prefix+"outgoing", "secret12345")
	signUpAndSignIn(t, router, prefix+"stranger", "secret12345")

	doJSON(t, router, http.MethodPatch, "/api/v1/users/me", map[string]interface{}{"avatarColor": 1, "avatarFont": "playfair"}, friend)
	doJSON(t, router, http.MethodPost, "/api/v1/friends/request", map[string]string{"username": prefix + "me"}, friend)
	doJSON(t, router, http.MethodPost, "/api/v1/friends/accept", map[string]string{"username": prefix + "friend"}, me)
	doJSON(t, router, http.MethodPost, "/api/v1/friends/request", map[string]string{"username": prefix + "me"}, incoming)
	doJSON(t, router, http.MethodPost, "/api/v1/friends/request", map[string]string{"username": prefix + "outgoing"}, me)

	rec := doJSON(t, router, http.MethodGet, "/api/v1/friends/search?username="+prefix, nil, me)
	users := decodeData[map[string][]model.UserSearchResult](t, rec)["users"]
	got := map[string]string{}
	for _, u := range users {
		got[u.Username] = u.Relation
	}
	want := map[string]string{
		prefix + "friend":   model.RelationFriend,
		prefix + "incoming": model.RelationIncoming,
		prefix + "outgoing": model.RelationOutgoing,
		prefix + "stranger": model.RelationNone,
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v (self must be excluded)", got, want)
	}
	for name, relation := range want {
		if got[name] != relation {
			t.Fatalf("%s: relation %q, want %q", name, got[name], relation)
		}
	}

	list := decodeData[map[string][]model.FriendInfo](t, doJSON(t, router, http.MethodGet, "/api/v1/friends/list", nil, me))["friends"]
	if len(list) != 1 || list[0].AvatarColor == nil || *list[0].AvatarColor != 1 || list[0].AvatarFont == nil || *list[0].AvatarFont != "playfair" {
		t.Fatalf("friend list must carry avatar style: %+v", list)
	}
}

func TestEventDescriptionAndAllDay(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	access, _ := signUpAndSignIn(t, router, fmt.Sprintf("allday_%d", time.Now().UnixNano()), "secret12345")

	create := doJSON(t, router, http.MethodPost, "/api/v1/events/", map[string]interface{}{
		"title": "Trip", "description": "bring snacks", "allDay": true,
		"startTime": "2026-12-01T00:00:00+03:00", "endTime": "2026-12-03T00:00:00+03:00", "type": "personal",
	}, access)
	if create.Code != http.StatusOK {
		t.Fatalf("create: %d %s", create.Code, create.Body.String())
	}
	id := int(decodeData[map[string]float64](t, create)["eventId"])

	list := decodeData[model.EventListResponse](t, doJSON(t, router, http.MethodGet, "/api/v1/events/", nil, access))
	if len(list.Events) != 1 || list.Events[0].Description != "bring snacks" || !list.Events[0].AllDay {
		t.Fatalf("unexpected list: %+v", list.Events)
	}

	update := doJSON(t, router, http.MethodPatch, fmt.Sprintf("/api/v1/events/%d", id), map[string]interface{}{
		"title": "Trip", "description": "", "allDay": false,
		"startTime": "2026-12-01T10:00:00+03:00", "endTime": "2026-12-01T12:00:00+03:00", "type": "personal",
	}, access)
	if update.Code != http.StatusOK {
		t.Fatalf("update: %d %s", update.Code, update.Body.String())
	}
	list = decodeData[model.EventListResponse](t, doJSON(t, router, http.MethodGet, "/api/v1/events/", nil, access))
	if list.Events[0].Description != "" || list.Events[0].AllDay {
		t.Fatalf("update not applied: %+v", list.Events[0])
	}
}

func TestGroupEventDescriptionAndMemberAvatars(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()

	fx := setupGroupWithMember(t, router)
	doJSON(t, router, http.MethodPatch, "/api/v1/users/me", map[string]interface{}{"avatarColor": 6}, fx.memberAccess)

	rec := doJSON(t, router, http.MethodPost, fmt.Sprintf("/api/v1/groups/%d/events", fx.groupID), map[string]interface{}{
		"title": "Dinner", "description": "at 8", "allDay": false,
		"startTime": "2026-12-05T18:00:00Z", "endTime": "2026-12-05T20:00:00Z", "type": "meeting",
	}, fx.ownerAccess)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	events := decodeData[model.EventListResponse](t, doJSON(t, router, http.MethodGet, fmt.Sprintf("/api/v1/groups/%d/events", fx.groupID), nil, fx.memberAccess))
	if len(events.Events) != 1 || events.Events[0].Description != "at 8" {
		t.Fatalf("unexpected group events: %+v", events.Events)
	}

	details := decodeData[model.GroupDetails](t, doJSON(t, router, http.MethodGet, fmt.Sprintf("/api/v1/groups/%d", fx.groupID), nil, fx.ownerAccess))
	found := false
	for _, m := range details.Members {
		if m.Username == fx.memberName {
			found = m.AvatarColor != nil && *m.AvatarColor == 6
		}
	}
	if !found {
		t.Fatalf("member avatar style missing: %+v", details.Members)
	}
}
