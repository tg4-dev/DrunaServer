package model

import (
	"time"
)

type Group struct {
	ID            int       `json:"groupID" db:"id"`
	OwnerID       int       `json:"ownerID" db:"owner_id"`
	Name          string    `json:"name" db:"name" binding:"required"`
	ConfirmedTime *time.Time `json:"confirmedTime" db:"confirmed_time"`
}

type GroupMemberInfo struct {
	ID            int        `json:"id" db:"id"`
	Name          string     `json:"name" db:"name"`
	Username      string     `json:"username" db:"username"`
	ConfirmedTime *time.Time `json:"confirmedTime,omitempty" db:"confirmed_time"`
}

type GroupDetails struct {
	Group
	Members []GroupMemberInfo `json:"members"`
}

type AddGroupMemberDoc struct {
	Username string `json:"username"`
}

type ConfirmGroupTimeDoc struct {
	ConfirmedTime time.Time `json:"confirmedTime"`
}

type GroupFreeTimeInputDoc struct {
	Date     string `json:"date"`
	Timezone string `json:"timezone"`
}
