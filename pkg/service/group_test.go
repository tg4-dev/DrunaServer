package service

import (
	"druna_server/pkg/model"
	"errors"
	"testing"
	"time"
)

type mockGroupRepo struct {
	member bool
}

func (m *mockGroupRepo) CreateGroup(input model.Group) (int, error) { return 1, nil }
func (m *mockGroupRepo) ListGroups(userID int) ([]model.Group, error) {
	return nil, nil
}
func (m *mockGroupRepo) GetGroupDetails(groupID, userID int) (model.GroupDetails, error) {
	return model.GroupDetails{}, nil
}
func (m *mockGroupRepo) AddGroupMember(groupID, ownerID, memberID int) error { return nil }
func (m *mockGroupRepo) DeleteGroup(groupID, ownerID int) error              { return nil }
func (m *mockGroupRepo) LeaveGroup(groupID, userID int) error                { return nil }
func (m *mockGroupRepo) ConfirmMemberTime(groupID, userID int, confirmedTime time.Time) error {
	return nil
}
func (m *mockGroupRepo) GetMemberUserIDs(groupID int) ([]int, error) { return []int{1}, nil }
func (m *mockGroupRepo) IsGroupMember(groupID, userID int) (bool, error) {
	return m.member, nil
}

type errFriendRepo struct {
	mockFriendRepo
}

func (m errFriendRepo) GetFriendshipStatus(userID, friendID int) (string, int, error) {
	return "", 0, errors.New("db down")
}

func TestAddGroupMemberDoesNotMaskDBError(t *testing.T) {
	friends := &errFriendRepo{mockFriendRepo: mockFriendRepo{users: map[string]int{"bob": 2}}}
	svc := NewGroupService(&mockGroupRepo{}, friends, &mockEventRepo{}, nil)
	err := svc.AddGroupMember(1, 1, "bob")
	if err == nil || err.Error() != "db down" {
		t.Fatalf("database error was masked: %v", err)
	}
}

func TestAddGroupMemberRequiresAcceptedFriend(t *testing.T) {
	friends := &mockFriendRepo{
		users:  map[string]int{"bob": 2},
		status: "pending",
		from:   1,
	}
	svc := NewGroupService(&mockGroupRepo{}, friends, &mockEventRepo{}, nil)
	err := svc.AddGroupMember(1, 1, "bob")
	var domain *DomainError
	if !errors.As(err, &domain) || domain.Kind != KindValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}
