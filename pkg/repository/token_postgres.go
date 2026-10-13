package repository

import (
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

type TokenPostgres struct {
	db *sqlx.DB
}

func NewTokenPostgres(db *sqlx.DB) *TokenPostgres {
	return &TokenPostgres{db: db}
}

// ClaimRefreshToken inserts the refresh jti. claimed is false when another
// request already consumed the same token.
func (r *TokenPostgres) ClaimRefreshToken(jti, familyID string, expiresAt time.Time) (bool, error) {
	var got string
	query := `INSERT INTO revoked_tokens (jti, revoked_at, expires_at, family_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (jti) DO NOTHING
		RETURNING jti`
	err := r.db.QueryRow(query, jti, time.Now(), expiresAt, familyID).Scan(&got)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *TokenPostgres) RevokeFamily(familyID string, expiresAt time.Time) error {
	query := `INSERT INTO revoked_token_families (family_id, revoked_at, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (family_id) DO NOTHING`
	_, err := r.db.Exec(query, familyID, time.Now(), expiresAt)
	return err
}

func (r *TokenPostgres) IsFamilyRevoked(familyID string) (bool, error) {
	if familyID == "" {
		return false, nil
	}
	var exists int
	query := `SELECT 1 FROM revoked_token_families WHERE family_id = $1 LIMIT 1`
	err := r.db.Get(&exists, query, familyID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *TokenPostgres) Ping() error {
	return r.db.Ping()
}

func (r *TokenPostgres) PurgeExpiredTokens() (int64, error) {
	tokens, err := r.db.Exec(`DELETE FROM revoked_tokens WHERE expires_at < NOW()`)
	if err != nil {
		return 0, err
	}
	tokenCount, err := tokens.RowsAffected()
	if err != nil {
		return 0, err
	}
	families, err := r.db.Exec(`DELETE FROM revoked_token_families WHERE expires_at < NOW()`)
	if err != nil {
		return tokenCount, err
	}
	familyCount, err := families.RowsAffected()
	if err != nil {
		return tokenCount, err
	}
	return tokenCount + familyCount, nil
}
