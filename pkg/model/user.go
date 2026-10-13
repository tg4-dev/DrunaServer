package model

type User struct {
	ID           int    `json:"id" db:"id"`
	Name         string `json:"name" binding:"required" db:"name"`
	Username     string `json:"username" binding:"required" db:"username"`
	Email        string `json:"email" binding:"required" db:"email"`
	Password     string `json:"-"`
	PasswordHash string `json:"-" db:"password_hash"`
	AvatarURL    string `json:"avatarURL" db:"avatar_url"`
	TelegramID   *int64 `json:"telegramID,omitempty" db:"telegram_id"`
}
