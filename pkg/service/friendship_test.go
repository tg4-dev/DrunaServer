package service

import (
	"druna_server/pkg/model"
	"druna_server/pkg/repository"
	"errors"
	"testing"
)

type mockFriendRepo struct {
	users   map[string]int
	status  string
	from    int
	missing bool
	deleted bool
	created bool
}

func (m *mockFriendRepo) CreateFriendRequest(userID, friendID int) error {
	m.created = true
	m.status = "pending"
	m.from = userID
	return nil
}
func (m *mockFriendRepo) AcceptFriendRequest(userID int, friendID int) error { return nil }
func (m *mockFriendRepo) RejectFriendRequest(userID int, friendID int) error { return nil }
func (m *mockFriendRepo) ExistsByUsername(username string) (int, error) {
	id, ok := m.users[username]
	if !ok {
		return 0, repository.ErrNotFound
	}
	return id, nil
}
func (m *mockFriendRepo) GetFriendList(userID int) ([]model.FriendInfo, error) {
	return nil, nil
}
func (m *mockFriendRepo) GetIncomingFriendRequests(userID int) ([]model.FriendInfo, error) {
	return nil, nil
}
func (m *mockFriendRepo) GetOutgoingFriendRequests(userID int) ([]model.FriendInfo, error) {
	return nil, nil
}
func (m *mockFriendRepo) GetFriendRequestList(userID int) ([]model.FriendInfo, error) {
	return nil, nil
}
func (m *mockFriendRepo) GetFriendshipStatus(userID, friendID int) (string, int, error) {
	if m.missing {
		return "", 0, repository.ErrNotFound
	}
	return m.status, m.from, nil
}
func (m *mockFriendRepo) DeleteFriend(userID int, friendID int) error {
	m.deleted = true
	m.missing = true
	return nil
}

type mockFriendAuth struct{}

func (m mockFriendAuth) CreateUser(user model.User) (int, error) { return 0, nil }
func (m mockFriendAuth) GetUserByUsername(username string) (model.User, error) {
	return model.User{}, repository.ErrNotFound
}
func (m mockFriendAuth) GetUserByTelegramID(telegramID int64) (model.User, error) {
	return model.User{}, repository.ErrNotFound
}
func (m mockFriendAuth) GetUserByID(userID int) (model.User, error) {
	return model.User{ID: userID, Username: "alice"}, nil
}
func (m mockFriendAuth) UpdateUserProfile(userID int, name, avatarURL string) error { return nil }
func (m mockFriendAuth) SearchUsers(prefix string) ([]model.FriendInfo, error) {
	return nil, nil
}

func TestSendFriendRequestCounterRequest(t *testing.T) {
	repo := &mockFriendRepo{
		users:  map[string]int{"bob": 2},
		status: "pending",
		from:   2,
	}
	svc := NewFriendshipService(repo, mockFriendAuth{}, nil)
	err := svc.SendFriendRequest(1, "bob")
	if !errors.As(err, new(*DomainError)) {
		t.Fatalf("expected domain error, got %v", err)
	}
	var domain *DomainError
	if !errors.As(err, &domain) || domain.Kind != KindConflict || domain.Msg != "user already sent you a request" {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.created {
		t.Fatal("must not create a second request")
	}
}

func TestSendFriendRequestAfterReject(t *testing.T) {
	repo := &mockFriendRepo{
		users:  map[string]int{"bob": 2},
		status: "rejected",
		from:   1,
	}
	svc := NewFriendshipService(repo, mockFriendAuth{}, nil)
	if err := svc.SendFriendRequest(1, "bob"); err != nil {
		t.Fatalf("resend after reject: %v", err)
	}
	if !repo.deleted || !repo.created {
		t.Fatalf("expected delete then create, deleted=%v created=%v", repo.deleted, repo.created)
	}
}

func TestAcceptOnlyRecipient(t *testing.T) {
	repo := &mockFriendRepo{
		users:  map[string]int{"bob": 2},
		status: "pending",
		from:   1,
	}
	svc := NewFriendshipService(repo, mockFriendAuth{}, nil)
	err := svc.AcceptFriendRequest(1, "bob")
	var domain *DomainError
	if !errors.As(err, &domain) || domain.Kind != KindForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}
