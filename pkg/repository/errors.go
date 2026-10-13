package repository

import (
	"database/sql"
	"errors"

	"github.com/lib/pq"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrDuplicate        = errors.New("duplicate")
	ErrNotOwner         = errors.New("not owner")
	ErrOwnerCannotLeave = errors.New("owner cannot leave")
	ErrNotMember        = errors.New("not member")
	ErrOverlap          = errors.New("overlap")
)

// MapPGError turns driver errors into repository sentinels.
// sql.ErrNoRows becomes ErrNotFound; unique violations (23505) become ErrDuplicate.
func MapPGError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return ErrDuplicate
	}
	return err
}
