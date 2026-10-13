package service

import (
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
