package model

import "time"

type SignUpDoc struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SignInDoc struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RenewTokenDoc struct {
	RefreshToken string `json:"refreshToken"`
}

type EventDoc struct {
	ID        int       `json:"eventID"`
	StartTime time.Time `json:"startTime" binding:"required"`
	EndTime   time.Time `json:"endTime" binding:"required"`
	Title     string    `json:"title" binding:"required"`
	Type      string    `json:"type"`
}

type DeleteEventDoc struct {
	ID int `json:"eventID"`
}

type AddEventDoc struct {
	ID int `json:"Id"`
}

type TimeSlot struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type FreeTimeInputDoc struct {
	Date     string `json:"date"`
	Timezone string `json:"timezone"`
}

type FreeTimeResponseDoc struct {
	FreeSlots []TimeSlot `json:"freeSlots"`
}

type TelegramAuthDoc struct {
	InitData string `json:"initData"`
}

type TelegramWidgetDoc struct {
	ID        int64  `json:"id" binding:"required"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	PhotoURL  string `json:"photo_url"`
	AuthDate  int64  `json:"auth_date" binding:"required"`
	Hash      string `json:"hash" binding:"required"`
}

type FriendRequestDoc struct {
	Username string `json:"username"`
}
