package integration

import (
	"druna_server/pkg/model"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

type inviteUsers struct {
	prefix                    string
	org, alice, bob, stranger string // access tokens
}

func (u inviteUsers) name(who string) string { return u.prefix + who }

func makeFriends(t *testing.T, router http.Handler, aAccess, aName, bAccess, bName string) {
	t.Helper()
	if rec := doJSON(t, router, http.MethodPost, "/api/v1/friends/request", map[string]string{"username": bName}, aAccess); rec.Code != http.StatusOK {
		t.Fatalf("request: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, router, http.MethodPost, "/api/v1/friends/accept", map[string]string{"username": aName}, bAccess); rec.Code != http.StatusOK {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body.String())
	}
}

func setupInviteUsers(t *testing.T, router http.Handler) inviteUsers {
	t.Helper()
	u := inviteUsers{prefix: fmt.Sprintf("inv%d_", time.Now().UnixNano())}
	u.org, _ = signUpAndSignIn(t, router, u.name("org"), "secret12345")
	u.alice, _ = signUpAndSignIn(t, router, u.name("alice"), "secret12345")
	u.bob, _ = signUpAndSignIn(t, router, u.name("bob"), "secret12345")
	u.stranger, _ = signUpAndSignIn(t, router, u.name("stranger"), "secret12345")
	makeFriends(t, router, u.org, u.name("org"), u.alice, u.name("alice"))
	makeFriends(t, router, u.org, u.name("org"), u.bob, u.name("bob"))
	return u
}

func participantStatus(inv model.Invite, username string) string {
	for _, p := range inv.Participants {
		if p.User.Username == username {
			return p.Status
		}
	}
	return "<missing>"
}

func listInviteIDs(t *testing.T, router http.Handler, access, status string) []int {
	t.Helper()
	rec := doJSON(t, router, http.MethodGet, "/api/v1/invites?status="+status, nil, access)
	if rec.Code != http.StatusOK {
		t.Fatalf("list %s: %d %s", status, rec.Code, rec.Body.String())
	}
	list := decodeData[model.InviteListResponse](t, rec)
	ids := []int{}
	for _, inv := range list.Invites {
		ids = append(ids, inv.ID)
	}
	if list.Total != len(ids) {
		t.Fatalf("total %d != %d", list.Total, len(ids))
	}
	return ids
}

func outboxTypes(t *testing.T, userAccessName string) []string {
	t.Helper()
	db, err := sqlx.Open("postgres", os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	var types []string
	err = db.Select(&types, `SELECT o.type FROM notification_outbox o JOIN users u ON u.id = o.user_id WHERE u.username = $1 ORDER BY o.id`, userAccessName)
	if err != nil {
		t.Fatalf("outbox: %v", err)
	}
	return types
}

func TestInviteLifecycle(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()
	u := setupInviteUsers(t, router)

	start := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	body := map[string]interface{}{
		"title": "Picnic", "description": "bring a blanket",
		"startTime": start, "endTime": start.Add(3 * time.Hour),
		"invitees": []string{u.name("alice"), u.name("bob"), u.name("stranger")},
	}
	rec := doJSON(t, router, http.MethodPost, "/api/v1/invites", body, u.org)
	if rec.Code != http.StatusBadRequest || errorField(t, rec.Body.Bytes()) != "invitees" || !strings.Contains(rec.Body.String(), u.name("stranger")) {
		t.Fatalf("non-friend must be rejected: %d %s", rec.Code, rec.Body.String())
	}

	body["invitees"] = []string{u.name("alice"), u.name("bob")}
	rec = doJSON(t, router, http.MethodPost, "/api/v1/invites", body, u.org)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	inv := decodeData[model.Invite](t, rec)
	if inv.Organizer.Username != u.name("org") || len(inv.Participants) != 2 ||
		participantStatus(inv, u.name("alice")) != model.InviteStatusPending || inv.Description != "bring a blanket" {
		t.Fatalf("unexpected invite: %s", rec.Body.String())
	}
	path := fmt.Sprintf("/api/v1/invites/%d", inv.ID)

	if got := outboxTypes(t, u.name("alice")); len(got) == 0 || got[len(got)-1] != "invite_created" {
		t.Fatalf("alice outbox: %v", got)
	}

	if rec := doJSON(t, router, http.MethodGet, path, nil, u.stranger); rec.Code != http.StatusNotFound {
		t.Fatalf("stranger must get 404, got %d", rec.Code)
	}
	if rec := doJSON(t, router, http.MethodGet, path, nil, u.bob); rec.Code != http.StatusOK {
		t.Fatalf("bob get: %d %s", rec.Code, rec.Body.String())
	}

	if ids := listInviteIDs(t, router, u.alice, "pending"); len(ids) != 1 || ids[0] != inv.ID {
		t.Fatalf("alice pending: %v", ids)
	}
	if ids := listInviteIDs(t, router, u.alice, "upcoming"); len(ids) != 0 {
		t.Fatalf("alice upcoming before answering: %v", ids)
	}
	if ids := listInviteIDs(t, router, u.org, "upcoming"); len(ids) != 1 {
		t.Fatalf("organizer upcoming: %v", ids)
	}

	rec = doJSON(t, router, http.MethodPost, path+"/respond", map[string]string{"status": "going"}, u.alice)
	if rec.Code != http.StatusOK || participantStatus(decodeData[model.Invite](t, rec), u.name("alice")) != model.InviteStatusGoing {
		t.Fatalf("alice respond: %d %s", rec.Code, rec.Body.String())
	}
	if ids := listInviteIDs(t, router, u.alice, "upcoming"); len(ids) != 1 {
		t.Fatalf("alice upcoming after answering: %v", ids)
	}
	if got := outboxTypes(t, u.name("org")); len(got) == 0 || got[len(got)-1] != "invite_answered" {
		t.Fatalf("organizer outbox: %v", got)
	}

	if rec := doJSON(t, router, http.MethodPost, path+"/respond", map[string]string{"status": "going"}, u.org); rec.Code != http.StatusBadRequest {
		t.Fatalf("organizer respond must be 400, got %d", rec.Code)
	}
	if rec := doJSON(t, router, http.MethodPost, path+"/respond", map[string]string{"status": "maybe"}, u.bob); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown status must be 400, got %d", rec.Code)
	}
	if rec := doJSON(t, router, http.MethodPatch, path, map[string]string{"title": "Mine"}, u.alice); rec.Code != http.StatusForbidden {
		t.Fatalf("invitee patch must be 403, got %d", rec.Code)
	}
	if rec := doJSON(t, router, http.MethodDelete, path, nil, u.alice); rec.Code != http.StatusForbidden {
		t.Fatalf("invitee delete must be 403, got %d", rec.Code)
	}

	rec = doJSON(t, router, http.MethodPatch, path, map[string]string{"title": "Big picnic"}, u.org)
	inv = decodeData[model.Invite](t, rec)
	if inv.Title != "Big picnic" || participantStatus(inv, u.name("alice")) != model.InviteStatusGoing {
		t.Fatalf("title change must keep answers: %s", rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodPatch, path, map[string]interface{}{"startTime": start.Add(time.Hour)}, u.org)
	inv = decodeData[model.Invite](t, rec)
	if participantStatus(inv, u.name("alice")) != model.InviteStatusPending {
		t.Fatalf("time change must reset answers: %s", rec.Body.String())
	}
	if rec := doJSON(t, router, http.MethodPatch, path, map[string]interface{}{"endTime": start}, u.org); rec.Code != http.StatusBadRequest {
		t.Fatalf("end before start must be 400, got %d", rec.Code)
	}

	doJSON(t, router, http.MethodPost, path+"/respond", map[string]string{"status": "not_going"}, u.bob)
	rec = doJSON(t, router, http.MethodPost, path+"/respond", map[string]string{"status": "pending"}, u.bob)
	inv = decodeData[model.Invite](t, rec)
	for _, p := range inv.Participants {
		if p.User.Username == u.name("bob") && (p.Status != model.InviteStatusPending || p.RespondedAt != nil) {
			t.Fatalf("withdrawn answer: %+v", p)
		}
	}

	if rec := doJSON(t, router, http.MethodDelete, path, nil, u.org); rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, router, http.MethodGet, path, nil, u.alice); rec.Code != http.StatusNotFound {
		t.Fatalf("deleted invite must be 404, got %d", rec.Code)
	}
	if got := outboxTypes(t, u.name("alice")); got[len(got)-1] != "invite_cancelled" {
		t.Fatalf("alice outbox after delete: %v", got)
	}
}

func TestInvitePastFilter(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()
	u := setupInviteUsers(t, router)

	start := time.Now().Add(-72 * time.Hour).UTC().Truncate(time.Second)
	rec := doJSON(t, router, http.MethodPost, "/api/v1/invites", map[string]interface{}{
		"title": "Old", "startTime": start, "endTime": start.Add(time.Hour), "invitees": []string{u.name("alice")},
	}, u.org)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	if ids := listInviteIDs(t, router, u.alice, "past"); len(ids) != 1 {
		t.Fatalf("past: %v", ids)
	}
	if ids := listInviteIDs(t, router, u.alice, "pending"); len(ids) != 0 {
		t.Fatalf("finished invite must not be pending: %v", ids)
	}
	if rec := doJSON(t, router, http.MethodGet, "/api/v1/invites?status=soon", nil, u.alice); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown filter must be 400, got %d", rec.Code)
	}
}

func TestFriendBusyIncludesEventsAndAcceptedInvites(t *testing.T) {
	router, cleanup := setupIntegration(t)
	defer cleanup()
	u := setupInviteUsers(t, router)

	day := time.Now().Add(96 * time.Hour).UTC().Truncate(24 * time.Hour)
	dayStr := day.Format("2006-01-02")

	rec := doJSON(t, router, http.MethodPost, "/api/v1/events/", map[string]interface{}{
		"title": "Secret dentist", "startTime": day.Add(9 * time.Hour), "endTime": day.Add(10 * time.Hour), "type": "personal",
	}, u.alice)
	if rec.Code != http.StatusOK {
		t.Fatalf("event: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodPost, "/api/v1/invites", map[string]interface{}{
		"title": "Secret party", "startTime": day.Add(18 * time.Hour), "endTime": day.Add(20 * time.Hour),
		"invitees": []string{u.name("alice")},
	}, u.org)
	inv := decodeData[model.Invite](t, rec)

	busyPath := fmt.Sprintf("/api/v1/friends/%s/busy?dateFrom=%s&dateTo=%s", u.name("alice"), dayStr, dayStr)

	// bob and alice are both friends of org, but not of each other.
	if rec := doJSON(t, router, http.MethodGet, busyPath, nil, u.bob); rec.Code != http.StatusNotFound {
		t.Fatalf("friend of a friend must get 404, got %d", rec.Code)
	}
	if rec := doJSON(t, router, http.MethodGet, busyPath, nil, u.stranger); rec.Code != http.StatusNotFound {
		t.Fatalf("non-friend must get 404, got %d", rec.Code)
	}

	rec = doJSON(t, router, http.MethodGet, busyPath, nil, u.org)
	busy := decodeData[model.FriendBusyResponse](t, rec)
	if len(busy.Busy) != 1 || !busy.Busy[0].StartTime.Equal(day.Add(9*time.Hour)) {
		t.Fatalf("pending invite must not count: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Secret") {
		t.Fatalf("titles leaked: %s", rec.Body.String())
	}

	doJSON(t, router, http.MethodPost, fmt.Sprintf("/api/v1/invites/%d/respond", inv.ID), map[string]string{"status": "going"}, u.alice)
	busy = decodeData[model.FriendBusyResponse](t, doJSON(t, router, http.MethodGet, busyPath, nil, u.org))
	if len(busy.Busy) != 2 || !busy.Busy[1].StartTime.Equal(day.Add(18*time.Hour)) {
		t.Fatalf("accepted invite must count: %+v", busy.Busy)
	}

	free := decodeData[map[string][]model.TimeSlot](t, doJSON(t, router, http.MethodPost, "/api/v1/events/free-time", map[string]string{"date": dayStr}, u.alice))["freeSlots"]
	for _, slot := range free {
		if slot.Start.Before(day.Add(20*time.Hour)) && slot.End.After(day.Add(18*time.Hour)) {
			t.Fatalf("accepted invite must block free time: %+v", free)
		}
	}
	orgFree := decodeData[map[string][]model.TimeSlot](t, doJSON(t, router, http.MethodPost, "/api/v1/events/free-time", map[string]string{"date": dayStr}, u.org))["freeSlots"]
	for _, slot := range orgFree {
		if slot.Start.Before(day.Add(20*time.Hour)) && slot.End.After(day.Add(18*time.Hour)) {
			t.Fatalf("organizer's invite must block free time: %+v", orgFree)
		}
	}

	if rec := doJSON(t, router, http.MethodGet, fmt.Sprintf("/api/v1/friends/%s/busy?dateFrom=%s&dateTo=2027-12-31", u.name("alice"), dayStr), nil, u.org); rec.Code != http.StatusBadRequest {
		t.Fatalf("range over 366 days must be 400, got %d", rec.Code)
	}
	if rec := doJSON(t, router, http.MethodGet, fmt.Sprintf("/api/v1/friends/%s/busy?dateFrom=nope&dateTo=%s", u.name("alice"), dayStr), nil, u.org); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad date must be 400, got %d", rec.Code)
	}
}
