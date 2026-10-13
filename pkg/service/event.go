package service

import (
	"druna_server/pkg/model"
	"druna_server/pkg/repository"
	"errors"
	"sort"
	"time"
)

type EventService struct {
	repo repository.Event
}

func NewEventService(repo repository.Event) *EventService {
	return &EventService{repo: repo}
}

func validateEventWindow(start, end time.Time) error {
	if !end.After(start) {
		return Validation("end time must be after start time")
	}
	return nil
}

func mapEventWriteErr(err error, overlapMsg string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrOverlap) {
		return Validation(overlapMsg)
	}
	if errors.Is(err, repository.ErrNotFound) {
		return NotFound("event not found")
	}
	return err
}

func (s *EventService) CreateEvent(event model.Event) (int, error) {
	if err := validateEventWindow(event.StartTime, event.EndTime); err != nil {
		return 0, err
	}
	id, err := s.repo.CreateEvent(event)
	if err != nil {
		return 0, mapEventWriteErr(err, "event overlaps with an existing event")
	}
	return id, nil
}

func (s *EventService) UpdateEvent(userID int, event model.Event) error {
	if err := validateEventWindow(event.StartTime, event.EndTime); err != nil {
		return err
	}
	return mapEventWriteErr(s.repo.UpdateEvent(userID, event), "event overlaps with an existing event")
}

func (s *EventService) DeleteEvent(userID, eventID int) error {
	err := s.repo.DeleteEvent(userID, eventID)
	if errors.Is(err, repository.ErrNotFound) {
		return NotFound("event not found")
	}
	return err
}

func (s *EventService) GetEventList(userID int, filter model.EventFilter) (model.EventListResponse, error) {
	filter = normalizeEventFilter(filter)
	events, err := s.repo.GetEventListFiltered(userID, filter)
	if err != nil {
		return model.EventListResponse{}, err
	}
	total, err := s.repo.CountEvents(userID, filter)
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

func (s *EventService) GetFreeTime(userID int, date time.Time) ([]model.TimeSlot, error) {
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.Add(24 * time.Hour)
	eventsByUser, err := s.repo.GetBusyEventsForUsers([]int{userID}, dayStart, dayEnd)
	if err != nil {
		return nil, err
	}
	return ComputeFreeSlots(eventsByUser[userID], date), nil
}

func normalizeEventFilter(filter model.EventFilter) model.EventFilter {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return filter
}

func (s *EventService) GetFreeTimeForUsers(userIDs []int, date time.Time) ([]model.TimeSlot, error) {
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.Add(24 * time.Hour)

	eventsByUser, err := s.repo.GetBusyEventsForUsers(userIDs, dayStart, dayEnd)
	if err != nil {
		return nil, err
	}

	var result []model.TimeSlot
	first := true
	for _, userID := range userIDs {
		slots := ComputeFreeSlots(eventsByUser[userID], date)
		if first {
			result = slots
			first = false
			continue
		}
		result = IntersectTimeSlots(result, slots)
	}
	if first {
		dayEndSlot := model.TimeSlot{Start: dayStart, End: dayEnd}
		return []model.TimeSlot{dayEndSlot}, nil
	}
	return result, nil
}

func ComputeFreeSlots(events []model.Event, date time.Time) []model.TimeSlot {
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.Add(24 * time.Hour)

	var busy []model.TimeSlot
	for _, event := range events {
		if event.EndTime.Before(dayStart) || !event.StartTime.Before(dayEnd) {
			continue
		}
		start := event.StartTime
		if start.Before(dayStart) {
			start = dayStart
		}
		end := event.EndTime
		if end.After(dayEnd) {
			end = dayEnd
		}
		if start.Before(end) {
			busy = append(busy, model.TimeSlot{Start: start, End: end})
		}
	}

	sort.Slice(busy, func(i, j int) bool { return busy[i].Start.Before(busy[j].Start) })

	merged := make([]model.TimeSlot, 0, len(busy))
	for _, slot := range busy {
		if len(merged) == 0 {
			merged = append(merged, slot)
			continue
		}
		last := &merged[len(merged)-1]
		if !slot.Start.After(last.End) {
			if slot.End.After(last.End) {
				last.End = slot.End
			}
			continue
		}
		merged = append(merged, slot)
	}

	free := make([]model.TimeSlot, 0)
	cursor := dayStart
	for _, slot := range merged {
		if cursor.Before(slot.Start) {
			free = append(free, model.TimeSlot{Start: cursor, End: slot.Start})
		}
		if slot.End.After(cursor) {
			cursor = slot.End
		}
	}
	if cursor.Before(dayEnd) {
		free = append(free, model.TimeSlot{Start: cursor, End: dayEnd})
	}
	return free
}

func IntersectTimeSlots(a, b []model.TimeSlot) []model.TimeSlot {
	result := make([]model.TimeSlot, 0)
	for _, slotA := range a {
		for _, slotB := range b {
			start := slotA.Start
			if slotB.Start.After(start) {
				start = slotB.Start
			}
			end := slotA.End
			if slotB.End.Before(end) {
				end = slotB.End
			}
			if start.Before(end) {
				result = append(result, model.TimeSlot{Start: start, End: end})
			}
		}
	}
	return result
}
