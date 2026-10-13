package service

import (
	"druna_server/pkg/model"
	"druna_server/pkg/repository"
	"errors"
	"fmt"
)

type FriendshipService struct {
	repo         repository.Friendship
	authRepo     repository.Authorization
	notification *NotificationService
}

func NewFriendshipService(repo repository.Friendship, authRepo repository.Authorization, notification *NotificationService) *FriendshipService {
	return &FriendshipService{repo: repo, authRepo: authRepo, notification: notification}
}

func (s *FriendshipService) SendFriendRequest(userID int, username string) error {
	friendID, err := s.repo.ExistsByUsername(username)
	if errors.Is(err, repository.ErrNotFound) {
		return NotFound("user not found")
	}
	if err != nil {
		return err
	}
	if friendID == userID {
		return Validation("cannot send friend request to yourself")
	}

	status, requesterID, err := s.repo.GetFriendshipStatus(userID, friendID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	if err == nil {
		switch status {
		case "accepted":
			return Conflict("you are already friends")
		case "pending":
			if requesterID == userID {
				return Conflict("friend request already pending")
			}
			return Conflict("user already sent you a request")
		case "rejected":
			if err := s.repo.DeleteFriend(userID, friendID); err != nil {
				return err
			}
		default:
			return Validation(fmt.Sprintf("unexpected friendship status: %s", status))
		}
	}

	if err := s.repo.CreateFriendRequest(userID, friendID); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return Conflict("friend request already pending")
		}
		return err
	}
	if s.notification != nil {
		if user, err := s.authRepo.GetUserByID(userID); err == nil {
			s.notification.EnqueueFriendRequest(friendID, user.Username)
		}
	}
	return nil
}

func (s *FriendshipService) AcceptFriendRequest(userID int, username string) error {
	friendID, err := s.lookupUser(username)
	if err != nil {
		return err
	}

	status, requesterID, err := s.repo.GetFriendshipStatus(userID, friendID)
	if errors.Is(err, repository.ErrNotFound) {
		return NotFound("no friend request found")
	}
	if err != nil {
		return err
	}
	if status != "pending" {
		return Validation(fmt.Sprintf("friend request is not pending (status: %s)", status))
	}
	if requesterID == userID {
		return Forbidden("only the recipient can accept a friend request")
	}

	if err := s.repo.AcceptFriendRequest(userID, friendID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return NotFound("no friend request found")
		}
		return err
	}
	return nil
}

func (s *FriendshipService) RejectFriendRequest(userID int, username string) error {
	friendID, err := s.lookupUser(username)
	if err != nil {
		return err
	}

	status, requesterID, err := s.repo.GetFriendshipStatus(userID, friendID)
	if errors.Is(err, repository.ErrNotFound) {
		return NotFound("no friend request found")
	}
	if err != nil {
		return err
	}
	if status != "pending" {
		return Validation(fmt.Sprintf("friend request is not pending (status: %s)", status))
	}
	if requesterID == userID {
		return Forbidden("only the recipient can reject a friend request")
	}

	if err := s.repo.RejectFriendRequest(userID, friendID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return NotFound("no friend request found")
		}
		return err
	}
	return nil
}

func (s *FriendshipService) lookupUser(username string) (int, error) {
	friendID, err := s.repo.ExistsByUsername(username)
	if errors.Is(err, repository.ErrNotFound) {
		return 0, NotFound("user not found")
	}
	return friendID, err
}

func (s *FriendshipService) FriendList(userID int) ([]model.FriendInfo, error) {
	return s.repo.GetFriendList(userID)
}

func (s *FriendshipService) FriendRequestList(userID int) ([]model.FriendInfo, error) {
	return s.repo.GetFriendRequestList(userID)
}

func (s *FriendshipService) IncomingFriendRequests(userID int) ([]model.FriendInfo, error) {
	return s.repo.GetIncomingFriendRequests(userID)
}

func (s *FriendshipService) OutgoingFriendRequests(userID int) ([]model.FriendInfo, error) {
	return s.repo.GetOutgoingFriendRequests(userID)
}

func (s *FriendshipService) DeleteFriend(userID int, username string) error {
	friendID, err := s.lookupUser(username)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteFriend(userID, friendID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return NotFound("no friendship found to delete")
		}
		return err
	}
	return nil
}
