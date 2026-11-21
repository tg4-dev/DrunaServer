package model

import (
	"time"
)

type Friend struct {
	UserID      int       `json:"userID"`
	FriendID    int       `json:"friendID"`
	Status      string    `json:"status"`
	RequestAt   time.Time `json:"requestAt"`
	ConfirmedAt time.Time `json:"confirmedAt"`
	// User        User      `json:"user"`
	// Friend      User      `json:"friend"`
	//PRIMARY KEY(user_id, friend_id)
}

type FriendInfo struct {
	ID       int    `json:"id" db:"id"`
	Name     string `json:"name" db:"name"`
	Username string `json:"username" db:"username"`
	AvatarStyle
}

// Relation of a search result to the current user.
const (
	RelationFriend   = "friend"
	RelationIncoming = "incoming"
	RelationOutgoing = "outgoing"
	RelationNone     = "none"
)

type UserSearchResult struct {
	FriendInfo
	Relation string `json:"relation" db:"relation"`
}
