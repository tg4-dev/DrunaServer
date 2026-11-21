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

// DuplicateError is a unique violation on a known request field.
// errors.Is(err, ErrDuplicate) holds for it.
type DuplicateError struct {
	Field string
}

func (e *DuplicateError) Error() string {
	if e.Field == "" {
		return ErrDuplicate.Error()
	}
	return e.Field + " " + ErrDuplicate.Error()
}

func (e *DuplicateError) Is(target error) bool {
	return target == ErrDuplicate
}
