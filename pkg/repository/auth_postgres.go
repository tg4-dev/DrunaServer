package repository

import (
	"druna_server/pkg/model"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type AuthPostgres struct {
	db *sqlx.DB
}

func NewAuthPostgres(db *sqlx.DB) *AuthPostgres {
	return &AuthPostgres{db: db}
}

// userColumns selects a full users row; avatar_url is nullable in the schema.
const userColumns = "id, name, username, email, password_hash, COALESCE(avatar_url, '') AS avatar_url, avatar_color, avatar_font"

// userConstraintFields maps unique constraints on users to request fields.
var userConstraintFields = map[string]string{
	"unique_username": "username",
	"unique_email":    "email",
}

func (r *AuthPostgres) CreateUser(user model.User) (int, error) {
	var id int
	query := fmt.Sprintf(
		"INSERT INTO %s (name, username, email, password_hash, avatar_url, avatar_color, avatar_font) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id",
		usersTable,
	)
	row := r.db.QueryRow(query, user.Name, user.Username, user.Email, user.PasswordHash, user.AvatarURL, user.AvatarColor, user.AvatarFont)

	if err := row.Scan(&id); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return 0, &DuplicateError{Field: userConstraintFields[pqErr.Constraint]}
		}
		return 0, MapPGError(err)
	}
	return id, nil
}

func (r *AuthPostgres) GetUserByUsername(username string) (model.User, error) {
	var user model.User
	query := fmt.Sprintf("SELECT %s FROM %s WHERE username=$1", userColumns, usersTable)
	err := r.db.Get(&user, query, username)
	return user, MapPGError(err)
}

func (r *AuthPostgres) GetUserByID(userID int) (model.User, error) {
	var user model.User
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id=$1", userColumns, usersTable)
	err := r.db.Get(&user, query, userID)
	return user, MapPGError(err)
}

func (r *AuthPostgres) UsernameExists(username string) (bool, error) {
	var exists bool
	query := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM %s WHERE LOWER(username) = LOWER($1))", usersTable)
	err := r.db.Get(&exists, query, username)
	return exists, err
}

func (r *AuthPostgres) UpdateUserProfile(userID int, update model.ProfileUpdate) error {
	var sets []string
	args := []interface{}{userID}
	add := func(column string, value interface{}) {
		args = append(args, value)
		sets = append(sets, fmt.Sprintf("%s = $%d", column, len(args)))
	}

	if update.Name != nil {
		add("name", *update.Name)
	}
	if update.AvatarURL != nil {
		add("avatar_url", *update.AvatarURL)
	}
	if update.AvatarColor != nil {
		add("avatar_color", update.AvatarColor.Value)
	}
	if update.AvatarFont != nil {
		add("avatar_font", update.AvatarFont.Value)
	}
	if len(sets) == 0 {
		return nil
	}

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = $1", usersTable, strings.Join(sets, ", "))
	result, err := r.db.Exec(query, args...)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// SearchUsers finds users by username prefix, excluding the caller, and
// reports how each one relates to the caller.
func (r *AuthPostgres) SearchUsers(userID int, prefix string) ([]model.UserSearchResult, error) {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		return []model.UserSearchResult{}, nil
	}

	users := []model.UserSearchResult{}
	query := fmt.Sprintf(`
		SELECT u.id, u.name, u.username, COALESCE(u.avatar_url, '') AS avatar_url, u.avatar_color, u.avatar_font,
			CASE
				WHEN f.status = 'accepted' THEN '%s'
				WHEN f.status = 'pending' AND f.user_id = $2 THEN '%s'
				WHEN f.status = 'pending' THEN '%s'
				ELSE '%s'
			END AS relation
		FROM %s u
		LEFT JOIN %s f ON (f.user_id = $2 AND f.friend_id = u.id) OR (f.friend_id = $2 AND f.user_id = u.id)
		WHERE u.username ILIKE $1 ESCAPE '\' AND u.id <> $2
		ORDER BY u.username
		LIMIT 20`,
		model.RelationFriend, model.RelationOutgoing, model.RelationIncoming, model.RelationNone,
		usersTable, friendsTable)
	if err := r.db.Select(&users, query, escapeLike(trimmed)+"%", userID); err != nil {
		return nil, err
	}
	return users, nil
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}
