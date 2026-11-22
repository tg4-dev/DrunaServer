package service

import (
	"druna_server/pkg/model"
	"druna_server/pkg/repository"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const maxInviteTitleLen = 255

type InviteService struct {
	repo         repository.Invite
	friendRepo   repository.Friendship
	notification *NotificationService
	now          func() time.Time
}

func NewInviteService(repo repository.Invite, friendRepo repository.Friendship, notification *NotificationService) *InviteService {
	return &InviteService{repo: repo, friendRepo: friendRepo, notification: notification, now: time.Now}
}

var (
	ErrInviteNotFound      = NotFound("invite not found")
	ErrInviteOrganizerOnly = Forbidden("only the organizer can change this invite")
)

func validateInviteFields(invite model.Invite) error {
	title := strings.TrimSpace(invite.Title)
	if title == "" {
		return Validation("title is required").WithField("title")
	}
	if utf8.RuneCountInString(title) > maxInviteTitleLen {
		return Validation(fmt.Sprintf("title must be at most %d characters", maxInviteTitleLen)).WithField("title")
	}
	if utf8.RuneCountInString(invite.Description) > model.MaxEventDescriptionLen {
		return Validation(fmt.Sprintf("description must be at most %d characters", model.MaxEventDescriptionLen)).WithField("description")
	}
	return validateEventWindow(invite.StartTime, invite.EndTime)
}

// resolveInvitees maps usernames to user ids. Every invitee must be an
// accepted friend of the organizer; otherwise a 400 lists the offenders.
func (s *InviteService) resolveInvitees(organizerID int, usernames []string) ([]int, error) {
	seen := map[string]bool{}
	var ids []int
	var notFriends []string
	for _, raw := range usernames {
		username := strings.TrimSpace(raw)
		if username == "" || seen[username] {
			continue
		}
		seen[username] = true

		id, err := s.friendRepo.ExistsByUsername(username)
		if errors.Is(err, repository.ErrNotFound) {
			notFriends = append(notFriends, username)
			continue
		}
		if err != nil {
			return nil, err
		}
		if id == organizerID {
			return nil, Validation("you cannot invite yourself").WithField("invitees")
		}
		status, _, err := s.friendRepo.GetFriendshipStatus(organizerID, id)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}
		if status != "accepted" {
			notFriends = append(notFriends, username)
			continue
		}
		ids = append(ids, id)
	}
	if len(notFriends) > 0 {
		return nil, Validation("only friends can be invited: " + strings.Join(notFriends, ", ")).WithField("invitees")
	}
	if len(ids) == 0 {
		return nil, Validation("invitees must list at least one friend").WithField("invitees")
	}
	if len(ids) > model.MaxInvitees {
		return nil, Validation(fmt.Sprintf("at most %d invitees", model.MaxInvitees)).WithField("invitees")
	}
	return ids, nil
}

func (s *InviteService) CreateInvite(organizerID int, input model.CreateInviteInput) (model.Invite, error) {
	invite := model.Invite{
		OrganizerID: organizerID,
		Title:       strings.TrimSpace(input.Title),
		Description: input.Description,
		StartTime:   input.StartTime,
		EndTime:     input.EndTime,
		AllDay:      input.AllDay,
	}
	if err := validateInviteFields(invite); err != nil {
		return model.Invite{}, err
	}
	inviteeIDs, err := s.resolveInvitees(organizerID, input.Invitees)
	if err != nil {
		return model.Invite{}, err
	}

	id, err := s.repo.CreateInvite(invite, inviteeIDs)
	if err != nil {
		return model.Invite{}, err
	}
	created, err := s.repo.GetInvite(id, organizerID)
	if err != nil {
		return model.Invite{}, err
	}
	for _, userID := range inviteeIDs {
		s.notification.EnqueueInvite(userID, "invite_created", created, nil)
	}
	return created, nil
}

func (s *InviteService) GetInvite(inviteID, userID int) (model.Invite, error) {
	invite, err := s.repo.GetInvite(inviteID, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return model.Invite{}, ErrInviteNotFound
	}
	return invite, err
}

func (s *InviteService) ListInvites(userID int, filter model.InviteFilter) (model.InviteListResponse, error) {
	switch filter.Status {
	case "", model.InviteFilterPending, model.InviteFilterUpcoming, model.InviteFilterPast:
	default:
		return model.InviteListResponse{}, Validation("status must be pending, upcoming or past").WithField("status")
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	filter.Now = s.now()

	invites, total, err := s.repo.ListInvites(userID, filter)
	if err != nil {
		return model.InviteListResponse{}, err
	}
	return model.InviteListResponse{Invites: invites, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

// organizerInvite loads an invite the user can see and checks they organize it.
func (s *InviteService) organizerInvite(inviteID, userID int) (model.Invite, error) {
	invite, err := s.GetInvite(inviteID, userID)
	if err != nil {
		return model.Invite{}, err
	}
	if invite.OrganizerID != userID {
		return model.Invite{}, ErrInviteOrganizerOnly
	}
	return invite, nil
}

// UpdateInvite changes the organizer's fields. When the time changes, every
// answer is reset to pending: people agreed to the old time, not the new one.
func (s *InviteService) UpdateInvite(inviteID, userID int, input model.UpdateInviteInput) (model.Invite, error) {
	invite, err := s.organizerInvite(inviteID, userID)
	if err != nil {
		return model.Invite{}, err
	}

	updated := invite
	if input.Title != nil {
		updated.Title = strings.TrimSpace(*input.Title)
	}
	if input.Description != nil {
		updated.Description = *input.Description
	}
	if input.StartTime != nil {
		updated.StartTime = *input.StartTime
	}
	if input.EndTime != nil {
		updated.EndTime = *input.EndTime
	}
	if input.AllDay != nil {
		updated.AllDay = *input.AllDay
	}
	if err := validateInviteFields(updated); err != nil {
		return model.Invite{}, err
	}

	timeChanged := !updated.StartTime.Equal(invite.StartTime) || !updated.EndTime.Equal(invite.EndTime)
	if err := s.repo.UpdateInvite(updated, timeChanged); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Invite{}, ErrInviteNotFound
		}
		return model.Invite{}, err
	}

	result, err := s.repo.GetInvite(inviteID, userID)
	if err != nil {
		return model.Invite{}, err
	}
	for _, p := range result.Participants {
		s.notification.EnqueueInvite(p.User.ID, "invite_changed", result, map[string]interface{}{"responsesReset": timeChanged})
	}
	return result, nil
}

func (s *InviteService) DeleteInvite(inviteID, userID int) error {
	invite, err := s.organizerInvite(inviteID, userID)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteInvite(inviteID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrInviteNotFound
		}
		return err
	}
	for _, p := range invite.Participants {
		s.notification.EnqueueInvite(p.User.ID, "invite_cancelled", invite, nil)
	}
	return nil
}

func (s *InviteService) RespondInvite(inviteID, userID int, status string) (model.Invite, error) {
	switch status {
	case model.InviteStatusGoing, model.InviteStatusNotGoing, model.InviteStatusPending:
	default:
		return model.Invite{}, Validation("status must be going, not_going or pending").WithField("status")
	}
	invite, err := s.GetInvite(inviteID, userID)
	if err != nil {
		return model.Invite{}, err
	}
	if invite.OrganizerID == userID {
		return model.Invite{}, Validation("the organizer is always going and cannot respond").WithField("status")
	}
	if err := s.repo.RespondInvite(inviteID, userID, status); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Invite{}, ErrInviteNotFound
		}
		return model.Invite{}, err
	}

	result, err := s.repo.GetInvite(inviteID, userID)
	if err != nil {
		return model.Invite{}, err
	}
	var username string
	for _, p := range result.Participants {
		if p.User.ID == userID {
			username = p.User.Username
		}
	}
	s.notification.EnqueueInvite(invite.OrganizerID, "invite_answered", result, map[string]interface{}{
		"username": username,
		"status":   status,
	})
	return result, nil
}
