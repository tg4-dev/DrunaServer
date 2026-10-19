package service

import (
	"druna_server/pkg/model"
	"druna_server/pkg/repository"
	"time"
)

type Authorization interface {
	CreateUser(user model.User) (int, error)
	GenerateAccessRefreshToken(username, password string) (string, string, error)
	ParseAccessToken(token string) (int, string, error)
	ParseToken(token string) (int, string, error)
	RenewToken(refreshToken string) (string, string, error)
	Logout(refreshToken string) error
	GetCurrentUser(userID int) (model.UserProfile, error)
	UpdateProfile(userID int, name, avatarURL string) (model.UserProfile, error)
	SearchUsers(prefix string) ([]model.FriendInfo, error)
}

type Event interface {
	CreateEvent(event model.Event) (int, error)
	UpdateEvent(userID int, event model.Event) error
	DeleteEvent(userID, eventID int) error
	GetEventList(userID int, filter model.EventFilter) (model.EventListResponse, error)
	GetFreeTime(userID int, date time.Time) ([]model.TimeSlot, error)
}

type Friendship interface {
	SendFriendRequest(userID int, username string) error
	AcceptFriendRequest(userID int, username string) error
	RejectFriendRequest(userID int, username string) error
	FriendList(userID int) ([]model.FriendInfo, error)
	FriendRequestList(userID int) ([]model.FriendInfo, error)
	IncomingFriendRequests(userID int) ([]model.FriendInfo, error)
	OutgoingFriendRequests(userID int) ([]model.FriendInfo, error)
	DeleteFriend(userID int, username string) error
}

type Group interface {
	CreateGroup(input model.Group) (int, error)
	ListGroups(userID int) ([]model.Group, error)
	GetGroupDetails(groupID, userID int) (model.GroupDetails, error)
	AddGroupMember(groupID, ownerID int, username string) error
	DeleteGroup(groupID, ownerID int) error
	LeaveGroup(groupID, userID int) error
	ConfirmMemberTime(groupID, userID int, confirmedTime time.Time) error
	GetGroupFreeTime(groupID, userID int, date time.Time) ([]model.TimeSlot, error)
	CreateGroupEvent(groupID, userID int, event model.Event) (int, error)
	ListGroupEvents(groupID, userID int, filter model.EventFilter) (model.EventListResponse, error)
	UpdateGroupEvent(groupID, eventID, userID int, event model.Event) error
	DeleteGroupEvent(groupID, eventID, userID int) error
}

type Health interface {
	PingDB() error
}

type healthService struct {
	tokenRepo repository.Token
}

func NewHealthService(tokenRepo repository.Token) *healthService {
	return &healthService{tokenRepo: tokenRepo}
}

func (s *healthService) PingDB() error {
	return s.tokenRepo.Ping()
}

type Service struct {
	Authorization
	Event
	Friendship
	Group
	Health
}

func NewService(repos *repository.Repository) *Service {
	notifications := NewNotificationService(repos.Notification)
	return &Service{
		Authorization: NewAuthService(repos.Authorization, repos.Token),
		Event:         NewEventService(repos.Event),
		Friendship:    NewFriendshipService(repos.Friendship, repos.Authorization, notifications),
		Group:         NewGroupService(repos.Group, repos.Friendship, repos.Event, notifications),
		Health:        NewHealthService(repos.Token),
	}
}
