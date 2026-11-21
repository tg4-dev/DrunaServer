package service

import (
	"druna_server/pkg/model"
	"druna_server/pkg/repository"
	"errors"
	"sort"
	"time"
)

// MaxBusyRange bounds GET /friends/:username/busy.
const MaxBusyRange = 366 * 24 * time.Hour

type BusyService struct {
	events  repository.Event
	friends repository.Friendship
}

func NewBusyService(events repository.Event, friends repository.Friendship) *BusyService {
	return &BusyService{events: events, friends: friends}
}

// FriendBusy returns when an accepted friend is busy in [from, to): personal
// and group events and the invites they organize or accepted. Titles and
// other details are not exposed. Non-friends get the same 404 as unknown users.
func (s *BusyService) FriendBusy(userID int, username string, from, to time.Time) ([]model.BusyInterval, error) {
	if !to.After(from) {
		return nil, Validation("dateTo must be after dateFrom").WithField("dateTo")
	}
	if to.Sub(from) > MaxBusyRange {
		return nil, Validation("the range must be at most 366 days").WithField("dateTo")
	}

	friendID, err := s.friends.ExistsByUsername(username)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, NotFound("friend not found")
	}
	if err != nil {
		return nil, err
	}
	status, _, err := s.friends.GetFriendshipStatus(userID, friendID)
	if errors.Is(err, repository.ErrNotFound) || (err == nil && status != "accepted") {
		return nil, NotFound("friend not found")
	}
	if err != nil {
		return nil, err
	}

	byUser, err := s.events.GetBusyEventsForUsers([]int{friendID}, from, to)
	if err != nil {
		return nil, err
	}
	return busyIntervals(byUser[friendID], from, to), nil
}

// busyIntervals strips event details, drops events outside [from, to) and
// duplicates, and sorts by start.
func busyIntervals(events []model.Event, from, to time.Time) []model.BusyInterval {
	type key struct {
		start, end int64
		allDay     bool
	}
	seen := map[key]bool{}
	result := make([]model.BusyInterval, 0, len(events))
	for _, e := range events {
		if !e.EndTime.After(from) || !e.StartTime.Before(to) {
			continue
		}
		k := key{e.StartTime.UnixNano(), e.EndTime.UnixNano(), e.AllDay}
		if seen[k] {
			continue
		}
		seen[k] = true
		result = append(result, model.BusyInterval{StartTime: e.StartTime, EndTime: e.EndTime, AllDay: e.AllDay})
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].StartTime.Equal(result[j].StartTime) {
			return result[i].StartTime.Before(result[j].StartTime)
		}
		return result[i].EndTime.Before(result[j].EndTime)
	})
	return result
}
