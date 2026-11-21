package model

type UserProfile struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
	AvatarStyle
}

// UpdateProfileInput is the PATCH /users/me body. Absent fields are left
// unchanged. For avatarURL, avatarColor and avatarFont an explicit null
// clears the value. An empty name or avatarURL is treated as absent, which
// keeps older clients that always send both fields working.
type UpdateProfileInput struct {
	Name        OptionalString `json:"name" swaggertype:"string"`
	AvatarURL   OptionalString `json:"avatarURL" swaggertype:"string"`
	AvatarColor OptionalInt    `json:"avatarColor" swaggertype:"integer"`
	AvatarFont  OptionalString `json:"avatarFont" swaggertype:"string"`
}

// ProfileUpdate is a validated UpdateProfileInput for the repository.
// A nil pointer field means "leave unchanged".
type ProfileUpdate struct {
	Name        *string
	AvatarURL   *string // "" clears the avatar
	AvatarColor *OptionalInt
	AvatarFont  *OptionalString
}

type UsernameAvailability struct {
	Username  string `json:"username"`
	Available bool   `json:"available"`
}
