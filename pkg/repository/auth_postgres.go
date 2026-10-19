package repository

import (
	"druna_server/pkg/model"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

type AuthPostgres struct {
	db *sqlx.DB
}

func NewAuthPostgres(db *sqlx.DB) *AuthPostgres {
	return &AuthPostgres{db: db}
}

func (r *AuthPostgres) CreateUser(user model.User) (int, error) {
	var id int
	query := fmt.Sprintf(
		"INSERT INTO %s (name, username, email, password_hash, avatar_url) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		usersTable,
	)
	row := r.db.QueryRow(query, user.Name, user.Username, user.Email, user.PasswordHash, user.AvatarURL)

	if err := row.Scan(&id); err != nil {
		return 0, MapPGError(err)
	}
	return id, nil
}

func (r *AuthPostgres) GetUserByUsername(username string) (model.User, error) {
	var user model.User
	query := fmt.Sprintf(
		"SELECT id, name, username, email, password_hash, avatar_url FROM %s WHERE username=$1",
		usersTable,
	)
	err := r.db.Get(&user, query, username)
	return user, MapPGError(err)
}

func (r *AuthPostgres) GetUserByID(userID int) (model.User, error) {
	var user model.User
	query := fmt.Sprintf(
		"SELECT id, name, username, email, password_hash, avatar_url FROM %s WHERE id=$1",
		usersTable,
	)
	err := r.db.Get(&user, query, userID)
	return user, MapPGError(err)
}

func (r *AuthPostgres) UpdateUserProfile(userID int, name, avatarURL string) error {
	query := fmt.Sprintf(
		"UPDATE %s SET name = COALESCE(NULLIF($2, ''), name), avatar_url = COALESCE(NULLIF($3, ''), avatar_url) WHERE id = $1",
		usersTable,
	)
	_, err := r.db.Exec(query, userID, name, avatarURL)
	return err
}

func (r *AuthPostgres) SearchUsers(prefix string) ([]model.FriendInfo, error) {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		return []model.FriendInfo{}, nil
	}

	users := []model.FriendInfo{}
	query := fmt.Sprintf(`
		SELECT id, name, username FROM %s
		WHERE username ILIKE $1 ESCAPE '\'
		ORDER BY username
		LIMIT 20`, usersTable)
	if err := r.db.Select(&users, query, escapeLike(trimmed)+"%"); err != nil {
		return nil, err
	}
	return users, nil
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}
