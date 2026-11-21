package model

import "time"

// Participant answers to an invite. The organizer has no participant row
// and is always going.
const (
	InviteStatusPending  = "pending"
	InviteStatusGoing    = "going"
	InviteStatusNotGoing = "not_going"
)

// List filters for GET /invites.
const (
	InviteFilterPending  = "pending"  // I am invited and have not answered; not finished yet
	InviteFilterUpcoming = "upcoming" // not finished; I organize it or have answered
	InviteFilterPast     = "past"     // already finished
)

const MaxInvitees = 50

type Invite struct {
	ID           int                 `json:"id" db:"id"`
	Title        string              `json:"title" db:"title"`
	Description  string              `json:"description" db:"description"`
	StartTime    time.Time           `json:"startTime" db:"start_time"`
	EndTime      time.Time           `json:"endTime" db:"end_time"`
	AllDay       bool                `json:"allDay" db:"all_day"`
	OrganizerID  int                 `json:"-" db:"organizer_id"`
	Organizer    FriendInfo          `json:"organizer" db:"-"`
	Participants []InviteParticipant `json:"participants" db:"-"`
	CreatedAt    time.Time           `json:"createdAt" db:"created_at"`
	UpdatedAt    time.Time           `json:"updatedAt" db:"updated_at"`
}

type InviteParticipant struct {
	User        FriendInfo `json:"user"`
	Status      string     `json:"status"`
	RespondedAt *time.Time `json:"respondedAt"`
}

type CreateInviteInput struct {
	Title       string    `json:"title" binding:"required"`
	Description string    `json:"description"`
	StartTime   time.Time `json:"startTime" binding:"required"`
	EndTime     time.Time `json:"endTime" binding:"required"`
	AllDay      bool      `json:"allDay"`
	Invitees    []string  `json:"invitees" binding:"required"`
}

// UpdateInviteInput is the PATCH body; absent fields are unchanged.
type UpdateInviteInput struct {
	Title       *string    `json:"title"`
	Description *string    `json:"description"`
	StartTime   *time.Time `json:"startTime"`
	EndTime     *time.Time `json:"endTime"`
	AllDay      *bool      `json:"allDay"`
}

type RespondInviteInput struct {
	Status string `json:"status" binding:"required" enums:"going,not_going,pending"`
}

type InviteFilter struct {
	Status string
	Now    time.Time
	Limit  int
	Offset int
}

type InviteListResponse struct {
	Invites []Invite `json:"invites"`
	Total   int      `json:"total"`
	Limit   int      `json:"limit"`
	Offset  int      `json:"offset"`
}

// BusyInterval is a period when a user is busy, without any details of why.
type BusyInterval struct {
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	AllDay    bool      `json:"allDay"`
}

type FriendBusyResponse struct {
	Busy []BusyInterval `json:"busy"`
}
