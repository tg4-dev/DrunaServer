package service

import (
	"druna_server/pkg/model"
	"strings"
	"testing"
	"time"
)

func TestBusyIntervalsFiltersDedupesAndSorts(t *testing.T) {
	from := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	at := func(h int) time.Time { return from.Add(time.Duration(h) * time.Hour) }

	events := []model.Event{
		{Title: "b", StartTime: at(15), EndTime: at(16)},
		{Title: "a", StartTime: at(9), EndTime: at(10)},
		{Title: "dup", StartTime: at(9), EndTime: at(10)},
		{Title: "before", StartTime: at(-3), EndTime: at(0)},
		{Title: "after", StartTime: at(24), EndTime: at(26)},
		{Title: "spans", StartTime: at(-1), EndTime: at(1), AllDay: true},
	}
	got := busyIntervals(events, from, to)
	want := []time.Time{at(-1), at(9), at(15)}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i, w := range want {
		if !got[i].StartTime.Equal(w) {
			t.Fatalf("interval %d starts %v, want %v", i, got[i].StartTime, w)
		}
	}
	if !got[0].AllDay {
		t.Fatal("allDay must be kept")
	}
}

func TestFriendBusyValidatesRange(t *testing.T) {
	svc := NewBusyService(&mockEventRepo{}, nil)
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := svc.FriendBusy(1, "bob", from, from)
	assertKind(t, err, KindValidation, "dateTo")
	_, err = svc.FriendBusy(1, "bob", from, from.Add(MaxBusyRange+time.Hour))
	assertKind(t, err, KindValidation, "dateTo")
}

func TestInviteValidation(t *testing.T) {
	svc := NewInviteService(nil, nil, nil)
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	base := model.CreateInviteInput{Title: "Picnic", StartTime: start, EndTime: start.Add(time.Hour), Invitees: []string{"bob"}}

	cases := []struct {
		name  string
		edit  func(*model.CreateInviteInput)
		field string
	}{
		{"blank title", func(in *model.CreateInviteInput) { in.Title = "  " }, "title"},
		{"long title", func(in *model.CreateInviteInput) { in.Title = strings.Repeat("a", 256) }, "title"},
		{"long description", func(in *model.CreateInviteInput) { in.Description = strings.Repeat("a", 2001) }, "description"},
		{"end before start", func(in *model.CreateInviteInput) { in.EndTime = start }, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := base
			tc.edit(&in)
			_, err := svc.CreateInvite(1, in)
			assertKind(t, err, KindValidation, tc.field)
		})
	}

	_, err := svc.RespondInvite(1, 2, "maybe")
	assertKind(t, err, KindValidation, "status")
	_, err = svc.ListInvites(1, model.InviteFilter{Status: "soon"})
	assertKind(t, err, KindValidation, "status")
}
