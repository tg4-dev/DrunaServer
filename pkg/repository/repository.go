package repository

import (
	"druna_server/pkg/model"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type Authorization interface {
	CreateUser(user model.User) (int, error)
	GetUserByUsername(username string) (model.User, error)
	GetUserByID(userID int) (model.User, error)
	UpdateUserProfile(userID int, name, avatarURL string) error
	SearchUsers(prefix string) ([]model.FriendInfo, error)
}

type Token interface {
	ClaimRefreshToken(jti, familyID string, expiresAt time.Time) (bool, error)
	RevokeFamily(familyID string, expiresAt time.Time) error
	IsFamilyRevoked(familyID string) (bool, error)
	PurgeExpiredTokens() (int64, error)
	Ping() error
}

type Notification interface {
	Enqueue(userID int, notificationType string, payload string) error
}

type Event interface {
	CreateEvent(user model.Event) (int, error)
	UpdateEvent(userID int, event model.Event) error
	DeleteEvent(userID, eventID int) error
	HasOverlappingEvent(userID int, start, end time.Time, excludeID int) (bool, error)
	GetEventList(userID int) ([]model.Event, error)
	GetEventListFiltered(userID int, filter model.EventFilter) ([]model.Event, error)
	CountEvents(userID int, filter model.EventFilter) (int, error)
	GetBusyEventsForUsers(userIDs []int, dateFrom, dateTo time.Time) (map[int][]model.Event, error)

	CreateGroupEvent(event model.Event) (int, error)
	UpdateGroupEvent(groupID, eventID int, event model.Event) error
	DeleteGroupEvent(groupID, eventID int) error
	GetGroupEventByID(groupID, eventID int) (model.Event, error)
	GetGroupEvents(groupID int, filter model.EventFilter) ([]model.Event, error)
	CountGroupEvents(groupID int, filter model.EventFilter) (int, error)
	HasOverlappingGroupEvent(groupID int, start, end time.Time, excludeID int) (bool, error)
}

type Friendship interface {
	CreateFriendRequest(userID, friendID int) error
	AcceptFriendRequest(userID int, friendID int) error
	RejectFriendRequest(userID int, friendID int) error
	ExistsByUsername(username string) (int, error)
	GetFriendList(userID int) ([]model.FriendInfo, error)
	GetIncomingFriendRequests(userID int) ([]model.FriendInfo, error)
	GetOutgoingFriendRequests(userID int) ([]model.FriendInfo, error)
	GetFriendRequestList(userID int) ([]model.FriendInfo, error)
	GetFriendshipStatus(userID, friendID int) (status string, requesterID int, err error)
	DeleteFriend(userID int, friendID int) error
}

type Group interface {
	CreateGroup(input model.Group) (int, error)
	ListGroups(userID int) ([]model.Group, error)
	GetGroupDetails(groupID, userID int) (model.GroupDetails, error)
	AddGroupMember(groupID, ownerID, memberID int) error
	DeleteGroup(groupID, ownerID int) error
	LeaveGroup(groupID, userID int) error
	ConfirmMemberTime(groupID, userID int, confirmedTime time.Time) error
	GetMemberUserIDs(groupID int) ([]int, error)
	IsGroupMember(groupID, userID int) (bool, error)
}

type Repository struct {
	Authorization
	Token
	Notification
	Event
	Friendship
	Group
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{
		Authorization: NewAuthPostgres(db),
		Token:         NewTokenPostgres(db),
		Notification:  NewNotificationPostgres(db),
		Event:         NewEventPostgres(db),
		Friendship:    NewFriendshipPostgres(db),
		Group:         NewGroupPostgres(db),
	}
}
