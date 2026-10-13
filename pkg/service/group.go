package service

import (
	"druna_server/pkg/model"
	"druna_server/pkg/repository"
	"errors"
	"time"
)

type GroupService struct {
	repo         repository.Group
	friendRepo   repository.Friendship
	eventRepo    repository.Event
	notification *NotificationService
}

func NewGroupService(repo repository.Group, friendRepo repository.Friendship, eventRepo repository.Event, notification *NotificationService) *GroupService {
	return &GroupService{repo: repo, friendRepo: friendRepo, eventRepo: eventRepo, notification: notification}
}

func (s *GroupService) CreateGroup(input model.Group) (int, error) {
	if input.Name == "" {
		return 0, Validation("group name is required")
	}
	return s.repo.CreateGroup(input)
}

func (s *GroupService) ListGroups(userID int) ([]model.Group, error) {
	return s.repo.ListGroups(userID)
}

func (s *GroupService) GetGroupDetails(groupID, userID int) (model.GroupDetails, error) {
	details, err := s.repo.GetGroupDetails(groupID, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return details, NotFound("group not found or access denied")
	}
	return details, err
}

func (s *GroupService) AddGroupMember(groupID, ownerID int, username string) error {
	memberID, err := s.friendRepo.ExistsByUsername(username)
	if errors.Is(err, repository.ErrNotFound) {
		return NotFound("user not found")
	}
	if err != nil {
		return err
	}
	status, _, err := s.friendRepo.GetFriendshipStatus(ownerID, memberID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return Validation("can only add accepted friends to a group")
		}
		return err
	}
	if status != "accepted" {
		return Validation("can only add accepted friends to a group")
	}
	isMember, err := s.repo.IsGroupMember(groupID, memberID)
	if err != nil {
		return err
	}
	if isMember {
		return Conflict("user is already a group member")
	}
	if err := s.repo.AddGroupMember(groupID, ownerID, memberID); err != nil {
		if errors.Is(err, repository.ErrNotOwner) {
			return Forbidden("only group owner can add members")
		}
		return mapGroupRepoErr(err)
	}
	return nil
}

func (s *GroupService) DeleteGroup(groupID, ownerID int) error {
	err := s.repo.DeleteGroup(groupID, ownerID)
	if errors.Is(err, repository.ErrNotOwner) {
		return Forbidden("only group owner can delete the group")
	}
	if errors.Is(err, repository.ErrNotFound) {
		return NotFound("group not found")
	}
	return err
}

func (s *GroupService) LeaveGroup(groupID, userID int) error {
	return mapGroupRepoErr(s.repo.LeaveGroup(groupID, userID))
}

func (s *GroupService) ConfirmMemberTime(groupID, userID int, confirmedTime time.Time) error {
	if err := mapGroupRepoErr(s.repo.ConfirmMemberTime(groupID, userID, confirmedTime)); err != nil {
		return err
	}
	if s.notification != nil {
		details, err := s.repo.GetGroupDetails(groupID, userID)
		if err == nil && details.OwnerID != userID {
			s.notification.EnqueueGroupConfirm(details.OwnerID, groupID)
		}
	}
	return nil
}

func (s *GroupService) GetGroupFreeTime(groupID, userID int, date time.Time) ([]model.TimeSlot, error) {
	if _, err := s.GetGroupDetails(groupID, userID); err != nil {
		return nil, err
	}
	memberIDs, err := s.repo.GetMemberUserIDs(groupID)
	if err != nil {
		return nil, err
	}

	eventSvc := &EventService{repo: s.eventRepo}
	return eventSvc.GetFreeTimeForUsers(memberIDs, date)
}

func (s *GroupService) CreateGroupEvent(groupID, userID int, event model.Event) (int, error) {
	isMember, err := s.repo.IsGroupMember(groupID, userID)
	if err != nil {
		return 0, err
	}
	if !isMember {
		return 0, ErrGroupAccessDenied
	}

	if err := validateEventWindow(event.StartTime, event.EndTime); err != nil {
		return 0, err
	}

	event.UserID = userID
	event.GroupID = &groupID
	eventID, err := s.eventRepo.CreateGroupEvent(event)
	if err != nil {
		return 0, mapEventWriteErr(err, "event overlaps with an existing group event")
	}

	if s.notification != nil {
		memberIDs, err := s.repo.GetMemberUserIDs(groupID)
		if err == nil {
			for _, memberID := range memberIDs {
				if memberID == userID {
					continue
				}
				s.notification.EnqueueGroupEventCreated(memberID, groupID, eventID, event.Title, event.StartTime)
			}
		}
	}

	return eventID, nil
}

func (s *GroupService) ListGroupEvents(groupID, userID int, filter model.EventFilter) (model.EventListResponse, error) {
	isMember, err := s.repo.IsGroupMember(groupID, userID)
	if err != nil {
		return model.EventListResponse{}, err
	}
	if !isMember {
		return model.EventListResponse{}, ErrGroupAccessDenied
	}

	filter = normalizeEventFilter(filter)
	events, err := s.eventRepo.GetGroupEvents(groupID, filter)
	if err != nil {
		return model.EventListResponse{}, err
	}
	total, err := s.eventRepo.CountGroupEvents(groupID, filter)
	if err != nil {
		return model.EventListResponse{}, err
	}
	return model.EventListResponse{
		Events: events,
		Total:  total,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}, nil
}

func (s *GroupService) canModifyGroupEvent(groupID, eventID, userID int) (model.Event, error) {
	isMember, err := s.repo.IsGroupMember(groupID, userID)
	if err != nil {
		return model.Event{}, err
	}
	if !isMember {
		return model.Event{}, ErrGroupAccessDenied
	}
	existing, err := s.eventRepo.GetGroupEventByID(groupID, eventID)
	if errors.Is(err, repository.ErrNotFound) {
		return existing, ErrGroupEventNotFound
	}
	if err != nil {
		return existing, err
	}
	details, err := s.repo.GetGroupDetails(groupID, userID)
	if err != nil {
		return existing, err
	}
	if existing.UserID != userID && details.OwnerID != userID {
		return existing, ErrGroupEventForbidden
	}
	return existing, nil
}

func (s *GroupService) UpdateGroupEvent(groupID, eventID, userID int, event model.Event) error {
	if _, err := s.canModifyGroupEvent(groupID, eventID, userID); err != nil {
		return err
	}

	if err := validateEventWindow(event.StartTime, event.EndTime); err != nil {
		return err
	}
	return mapEventWriteErr(s.eventRepo.UpdateGroupEvent(groupID, eventID, event), "event overlaps with an existing group event")
}

func (s *GroupService) DeleteGroupEvent(groupID, eventID, userID int) error {
	if _, err := s.canModifyGroupEvent(groupID, eventID, userID); err != nil {
		return err
	}
	err := s.eventRepo.DeleteGroupEvent(groupID, eventID)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrGroupEventNotFound
	}
	return err
}

func mapGroupRepoErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrNotFound):
		return NotFound("group not found")
	case errors.Is(err, repository.ErrNotOwner):
		return Forbidden("only the group owner can do that")
	case errors.Is(err, repository.ErrOwnerCannotLeave):
		return Validation("group owner cannot leave; delete the group instead")
	case errors.Is(err, repository.ErrNotMember):
		return Forbidden("you are not a member of this group")
	case errors.Is(err, repository.ErrDuplicate):
		return Conflict("user is already a group member")
	default:
		return err
	}
}
