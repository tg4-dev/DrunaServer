package service

import (
	"druna_server/pkg/model"
	"druna_server/pkg/repository"
	"errors"
	"strings"
	"testing"
	"time"
)

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }
func set(v string) model.OptionalString {
	return model.OptionalString{Set: true, Value: &v}
}

func newProfileService(t *testing.T) (*AuthService, *mockAuthRepo, int) {
	t.Helper()
	repo := &mockAuthRepo{users: map[string]model.User{}}
	svc := NewAuthService(repo, &mockTokenRepo{})
	id, err := svc.CreateUser(model.User{Name: "Ann", Username: "ann", Email: "ann@test.local", Password: "secret12345"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return svc, repo, id
}

func assertKind(t *testing.T, err error, kind ErrorKind, field string) {
	t.Helper()
	var domain *DomainError
	if !errors.As(err, &domain) {
		t.Fatalf("expected DomainError, got %v", err)
	}
	if domain.Kind != kind || domain.Field != field {
		t.Fatalf("got kind=%d field=%q, want kind=%d field=%q", domain.Kind, domain.Field, kind, field)
	}
}

func TestUpdateProfileSetsAndClearsAvatarStyle(t *testing.T) {
	svc, _, id := newProfileService(t)

	profile, err := svc.UpdateProfile(id, model.UpdateProfileInput{
		AvatarURL:   set("https://example.com/a.png"),
		AvatarColor: model.OptionalInt{Set: true, Value: intPtr(5)},
		AvatarFont:  set("lobster"),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if profile.AvatarURL != "https://example.com/a.png" || profile.AvatarColor == nil || *profile.AvatarColor != 5 ||
		profile.AvatarFont == nil || *profile.AvatarFont != "lobster" {
		t.Fatalf("unexpected profile: %+v", profile)
	}

	profile, err = svc.UpdateProfile(id, model.UpdateProfileInput{
		AvatarURL:   model.OptionalString{Set: true},
		AvatarColor: model.OptionalInt{Set: true},
		AvatarFont:  model.OptionalString{Set: true},
	})
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if profile.AvatarURL != "" || profile.AvatarColor != nil || profile.AvatarFont != nil {
		t.Fatalf("avatar style not cleared: %+v", profile)
	}
	if profile.Name != "Ann" {
		t.Fatalf("name changed: %q", profile.Name)
	}
}

func TestUpdateProfileEmptyStringsAreIgnored(t *testing.T) {
	svc, _, id := newProfileService(t)
	if _, err := svc.UpdateProfile(id, model.UpdateProfileInput{AvatarURL: set("https://example.com/a.png")}); err != nil {
		t.Fatalf("update: %v", err)
	}
	profile, err := svc.UpdateProfile(id, model.UpdateProfileInput{Name: set("Anna"), AvatarURL: set("")})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if profile.Name != "Anna" || profile.AvatarURL != "https://example.com/a.png" {
		t.Fatalf("unexpected profile: %+v", profile)
	}
}

func TestUpdateProfileValidation(t *testing.T) {
	svc, _, id := newProfileService(t)
	cases := []struct {
		name  string
		input model.UpdateProfileInput
		field string
	}{
		{"color out of range", model.UpdateProfileInput{AvatarColor: model.OptionalInt{Set: true, Value: intPtr(8)}}, "avatarColor"},
		{"negative color", model.UpdateProfileInput{AvatarColor: model.OptionalInt{Set: true, Value: intPtr(-1)}}, "avatarColor"},
		{"unknown font", model.UpdateProfileInput{AvatarFont: set("comic-sans")}, "avatarFont"},
		{"http avatar", model.UpdateProfileInput{AvatarURL: set("http://example.com/a.png")}, "avatarURL"},
		{"null name", model.UpdateProfileInput{Name: model.OptionalString{Set: true}}, "name"},
		{"blank name", model.UpdateProfileInput{Name: set("   ")}, "name"},
		{"long name", model.UpdateProfileInput{Name: set(strings.Repeat("a", 256))}, "name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UpdateProfile(id, tc.input)
			assertKind(t, err, KindValidation, tc.field)
		})
	}
}

func TestCreateUserStoresAvatarStyle(t *testing.T) {
	repo := &mockAuthRepo{users: map[string]model.User{}}
	svc := NewAuthService(repo, &mockTokenRepo{})
	user := model.User{Name: "Bo", Username: "bo", Email: "bo@test.local", Password: "secret12345"}
	user.AvatarColor = intPtr(2)
	user.AvatarFont = strPtr("onest")
	if _, err := svc.CreateUser(user); err != nil {
		t.Fatalf("create: %v", err)
	}
	stored := repo.users["bo"]
	if stored.AvatarColor == nil || *stored.AvatarColor != 2 || stored.AvatarFont == nil || *stored.AvatarFont != "onest" {
		t.Fatalf("avatar style not stored: %+v", stored.AvatarStyle)
	}

	bad := model.User{Name: "Cy", Username: "cy", Email: "cy@test.local", Password: "secret12345"}
	bad.AvatarFont = strPtr("nope")
	_, err := svc.CreateUser(bad)
	assertKind(t, err, KindValidation, "avatarFont")
}

func TestCreateUserUsernameTakenIgnoresCase(t *testing.T) {
	svc, _, _ := newProfileService(t)
	_, err := svc.CreateUser(model.User{Name: "Ann 2", Username: "ANN", Email: "other@test.local", Password: "secret12345"})
	assertKind(t, err, KindConflict, "username")
}

type duplicateAuthRepo struct {
	mockAuthRepo
	field string
}

func (m *duplicateAuthRepo) CreateUser(model.User) (int, error) {
	return 0, &repository.DuplicateError{Field: m.field}
}

func TestCreateUserDuplicateReportsField(t *testing.T) {
	for _, field := range []string{"username", "email"} {
		repo := &duplicateAuthRepo{mockAuthRepo: mockAuthRepo{users: map[string]model.User{}}, field: field}
		svc := NewAuthService(repo, &mockTokenRepo{})
		_, err := svc.CreateUser(model.User{Name: "Dee", Username: "dee", Email: "dee@test.local", Password: "secret12345"})
		assertKind(t, err, KindConflict, field)
	}
}

func TestUsernameAvailable(t *testing.T) {
	svc, _, _ := newProfileService(t)

	available, err := svc.UsernameAvailable("Ann")
	if err != nil || available {
		t.Fatalf("Ann: available=%v err=%v, want taken", available, err)
	}
	available, err = svc.UsernameAvailable("new.user_1")
	if err != nil || !available {
		t.Fatalf("new.user_1: available=%v err=%v, want free", available, err)
	}
	for _, bad := range []string{"", "ab", strings.Repeat("a", 33), "with space", "dash-name", "юзер"} {
		_, err := svc.UsernameAvailable(bad)
		assertKind(t, err, KindValidation, "username")
	}
}

func TestEventDescriptionLimit(t *testing.T) {
	svc := NewEventService(&mockEventRepo{})
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	event := model.Event{UserID: 1, Title: "t", StartTime: start, EndTime: start.Add(time.Hour)}

	event.Description = strings.Repeat("я", model.MaxEventDescriptionLen)
	if _, err := svc.CreateEvent(event); err != nil {
		t.Fatalf("2000 characters must be accepted: %v", err)
	}
	event.Description += "я"
	_, err := svc.CreateEvent(event)
	assertKind(t, err, KindValidation, "description")
	assertKind(t, svc.UpdateEvent(1, event), KindValidation, "description")
}
