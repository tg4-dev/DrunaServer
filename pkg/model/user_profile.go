package model

type UserProfile struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatarURL"`
}

type UpdateProfileInput struct {
	Name      string `json:"name"`
	AvatarURL string `json:"avatarURL"`
}
