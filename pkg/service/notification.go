package service

import (
	"druna_server/pkg/model"
	"druna_server/pkg/repository"
	"encoding/json"
	"time"

	"github.com/sirupsen/logrus"
)

type NotificationService struct {
	repo repository.Notification
}

func NewNotificationService(repo repository.Notification) *NotificationService {
	return &NotificationService{repo: repo}
}

func (s *NotificationService) EnqueueFriendRequest(targetUserID int, fromUsername string) {
	s.enqueue(targetUserID, "friend_request", map[string]string{
		"fromUsername": fromUsername,
	})
}

func (s *NotificationService) EnqueueGroupConfirm(targetUserID int, groupID int) {
	s.enqueue(targetUserID, "group_confirm", map[string]int{
		"groupId": groupID,
	})
}

func (s *NotificationService) EnqueueGroupEventCreated(targetUserID, groupID, eventID int, title string, startTime time.Time) {
	s.enqueue(targetUserID, "group_event_created", map[string]interface{}{
		"groupId":   groupID,
		"eventId":   eventID,
		"title":     title,
		"startTime": startTime,
	})
}

// EnqueueInvite queues an invite event (invite_created, invite_changed,
// invite_cancelled, invite_answered) with the invite summary and extra fields.
func (s *NotificationService) EnqueueInvite(targetUserID int, notificationType string, invite model.Invite, extra map[string]interface{}) {
	payload := map[string]interface{}{
		"inviteId":          invite.ID,
		"title":             invite.Title,
		"startTime":         invite.StartTime,
		"endTime":           invite.EndTime,
		"allDay":            invite.AllDay,
		"organizerUsername": invite.Organizer.Username,
	}
	for k, v := range extra {
		payload[k] = v
	}
	s.enqueue(targetUserID, notificationType, payload)
}

func (s *NotificationService) enqueue(userID int, notificationType string, payload any) {
	if s == nil || s.repo == nil {
		return
	}
	body, err := json.Marshal(payload)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"user_id": userID,
			"type":    notificationType,
		}).Error("failed to marshal notification payload")
		return
	}
	if err := s.repo.Enqueue(userID, notificationType, string(body)); err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"user_id": userID,
			"type":    notificationType,
		}).Error("failed to enqueue notification")
	}
}
