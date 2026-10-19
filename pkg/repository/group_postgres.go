package repository

import (
	"database/sql"
	"druna_server/pkg/model"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type GroupPostgres struct {
	db *sqlx.DB
}

func NewGroupPostgres(db *sqlx.DB) *GroupPostgres {
	return &GroupPostgres{db: db}
}

func (r *GroupPostgres) CreateGroup(input model.Group) (int, error) {
	tx, err := r.db.Beginx()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var id int
	query := fmt.Sprintf(
		"INSERT INTO %s (owner_id, name, confirmed_time) VALUES ($1, $2, NULL) RETURNING id",
		groupTable,
	)
	if err := tx.QueryRow(query, input.OwnerID, input.Name).Scan(&id); err != nil {
		return 0, MapPGError(err)
	}

	memberQuery := "INSERT INTO group_members (group_id, user_id, confirmed_time) VALUES ($1, $2, NULL)"
	if _, err := tx.Exec(memberQuery, id, input.OwnerID); err != nil {
		return 0, MapPGError(err)
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return id, nil
}

func (r *GroupPostgres) ListGroups(userID int) ([]model.Group, error) {
	query := fmt.Sprintf(`
		SELECT DISTINCT g.id, g.owner_id, g.name, g.confirmed_time
		FROM %s g
		LEFT JOIN group_members gm ON g.id = gm.group_id
		WHERE g.owner_id = $1 OR gm.user_id = $1
		ORDER BY g.id`, groupTable)

	groups := []model.Group{}
	if err := r.db.Select(&groups, query, userID); err != nil {
		return nil, err
	}
	return groups, nil
}

func (r *GroupPostgres) GetGroupDetails(groupID, userID int) (model.GroupDetails, error) {
	var details model.GroupDetails

	accessQuery := `
		SELECT 1 FROM groups g
		LEFT JOIN group_members gm ON g.id = gm.group_id
		WHERE g.id = $1 AND (g.owner_id = $2 OR gm.user_id = $2)
		LIMIT 1`
	var access int
	if err := r.db.Get(&access, accessQuery, groupID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return details, ErrNotFound
		}
		return details, err
	}

	groupQuery := fmt.Sprintf(
		"SELECT id, owner_id, name, confirmed_time FROM %s WHERE id = $1",
		groupTable,
	)
	if err := r.db.Get(&details.Group, groupQuery, groupID); err != nil {
		return details, err
	}

	membersQuery := `
		SELECT u.id, u.name, u.username, gm.confirmed_time
		FROM group_members gm
		JOIN users u ON gm.user_id = u.id
		WHERE gm.group_id = $1
		ORDER BY u.username`
	if err := r.db.Select(&details.Members, membersQuery, groupID); err != nil {
		return details, err
	}

	return details, nil
}

func (r *GroupPostgres) AddGroupMember(groupID, ownerID, memberID int) error {
	var owner int
	ownerQuery := fmt.Sprintf("SELECT owner_id FROM %s WHERE id = $1", groupTable)
	if err := r.db.Get(&owner, ownerQuery, groupID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if owner != ownerID {
		return ErrNotOwner
	}

	query := `INSERT INTO group_members (group_id, user_id, confirmed_time) VALUES ($1, $2, NULL)`
	if _, err := r.db.Exec(query, groupID, memberID); err != nil {
		return MapPGError(err)
	}
	return r.syncGroupConfirmedTime(groupID)
}

func (r *GroupPostgres) IsGroupMember(groupID, userID int) (bool, error) {
	var exists int
	query := `SELECT 1 FROM group_members WHERE group_id = $1 AND user_id = $2 LIMIT 1`
	err := r.db.Get(&exists, query, groupID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (r *GroupPostgres) GetMemberUserIDs(groupID int) ([]int, error) {
	ids := []int{}
	query := `SELECT user_id FROM group_members WHERE group_id = $1 ORDER BY user_id`
	if err := r.db.Select(&ids, query, groupID); err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *GroupPostgres) DeleteGroup(groupID, ownerID int) error {
	var owner int
	if err := r.db.Get(&owner, fmt.Sprintf("SELECT owner_id FROM %s WHERE id = $1", groupTable), groupID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if owner != ownerID {
		return ErrNotOwner
	}
	_, err := r.db.Exec(fmt.Sprintf("DELETE FROM %s WHERE id = $1", groupTable), groupID)
	return err
}

func (r *GroupPostgres) LeaveGroup(groupID, userID int) error {
	var owner int
	if err := r.db.Get(&owner, fmt.Sprintf("SELECT owner_id FROM %s WHERE id = $1", groupTable), groupID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if owner == userID {
		return ErrOwnerCannotLeave
	}
	result, err := r.db.Exec(`DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`, groupID, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotMember
	}
	return r.syncGroupConfirmedTime(groupID)
}

func (r *GroupPostgres) ConfirmMemberTime(groupID, userID int, confirmedTime time.Time) error {
	isMember, err := r.IsGroupMember(groupID, userID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrNotMember
	}
	if _, err = r.db.Exec(`UPDATE group_members SET confirmed_time = $1 WHERE group_id = $2 AND user_id = $3`,
		confirmedTime, groupID, userID); err != nil {
		return err
	}
	return r.syncGroupConfirmedTime(groupID)
}

func (r *GroupPostgres) syncGroupConfirmedTime(groupID int) error {
	var row struct {
		Total     int        `db:"total"`
		Confirmed int        `db:"confirmed"`
		MinTime   *time.Time `db:"min_time"`
		MaxTime   *time.Time `db:"max_time"`
	}
	query := `
		SELECT COUNT(*) AS total,
		       COUNT(confirmed_time) AS confirmed,
		       MIN(confirmed_time) AS min_time,
		       MAX(confirmed_time) AS max_time
		FROM group_members
		WHERE group_id = $1`
	if err := r.db.Get(&row, query, groupID); err != nil {
		return err
	}
	if row.Total > 0 && row.Total == row.Confirmed && row.MinTime != nil && row.MaxTime != nil && row.MinTime.Equal(*row.MaxTime) {
		_, err := r.db.Exec(`UPDATE groups SET confirmed_time = $1 WHERE id = $2`, row.MinTime, groupID)
		return err
	}
	_, err := r.db.Exec(`UPDATE groups SET confirmed_time = NULL WHERE id = $1`, groupID)
	return err
}
