package repository

import (
	"database/sql"
	"druna_server/pkg/model"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type EventPostgres struct {
	db *sqlx.DB
}

func NewEventPostgres(db *sqlx.DB) *EventPostgres {
	return &EventPostgres{db: db}
}

func (r *EventPostgres) CreateEvent(event model.Event) (int, error) {
	tx, err := r.db.Beginx()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockScope(tx, fmt.Sprintf("user:%d", event.UserID)); err != nil {
		return 0, err
	}
	overlap, err := overlapping(tx, true, event.UserID, event.StartTime, event.EndTime, 0)
	if err != nil {
		return 0, err
	}
	if overlap {
		return 0, ErrOverlap
	}

	var id int
	query := fmt.Sprintf("INSERT INTO %s (user_id, start_time, end_time, title, type) VALUES ($1, $2, $3, $4, $5) RETURNING id", eventsTable)
	if err := tx.QueryRow(query, event.UserID, event.StartTime, event.EndTime, event.Title, event.Type).Scan(&id); err != nil {
		return 0, MapPGError(err)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *EventPostgres) UpdateEvent(userID int, event model.Event) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockScope(tx, fmt.Sprintf("user:%d", userID)); err != nil {
		return err
	}
	overlap, err := overlapping(tx, true, userID, event.StartTime, event.EndTime, event.ID)
	if err != nil {
		return err
	}
	if overlap {
		return ErrOverlap
	}

	query := fmt.Sprintf(`
		UPDATE %s SET start_time = $1, end_time = $2, title = $3, type = $4
		WHERE id = $5 AND user_id = $6 AND group_id IS NULL`, eventsTable)
	result, err := tx.Exec(query, event.StartTime, event.EndTime, event.Title, event.Type, event.ID, userID)
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
	return tx.Commit()
}

func (r *EventPostgres) DeleteEvent(userID, eventID int) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1 AND user_id = $2 AND group_id IS NULL", eventsTable)
	result, err := r.db.Exec(query, eventID, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *EventPostgres) HasOverlappingEvent(userID int, start, end time.Time, excludeID int) (bool, error) {
	query := fmt.Sprintf(`
		SELECT 1 FROM %s
		WHERE user_id = $1
		  AND group_id IS NULL
		  AND id <> $2
		  AND start_time < $4
		  AND end_time > $3
		LIMIT 1`, eventsTable)
	var exists int
	err := r.db.Get(&exists, query, userID, excludeID, start, end)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *EventPostgres) GetEventList(userID int) ([]model.Event, error) {
	return r.GetEventListFiltered(userID, model.EventFilter{})
}

func (r *EventPostgres) GetEventListFiltered(userID int, filter model.EventFilter) ([]model.Event, error) {
	query := fmt.Sprintf("SELECT id, user_id, group_id, title, start_time, end_time, type FROM %s WHERE user_id = $1 AND group_id IS NULL", eventsTable)
	args := []interface{}{userID}
	argIdx := 2

	if filter.DateFrom != nil {
		query += fmt.Sprintf(" AND end_time >= $%d", argIdx)
		args = append(args, *filter.DateFrom)
		argIdx++
	}
	if filter.DateTo != nil {
		query += fmt.Sprintf(" AND start_time <= $%d", argIdx)
		args = append(args, *filter.DateTo)
		argIdx++
	}
	if filter.Type != "" {
		query += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, filter.Type)
		argIdx++
	}

	query += " ORDER BY start_time ASC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filter.Offset)
	}

	events := []model.Event{}
	if err := r.db.Select(&events, query, args...); err != nil {
		return nil, err
	}
	return events, nil
}

func (r *EventPostgres) CountEvents(userID int, filter model.EventFilter) (int, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE user_id = $1 AND group_id IS NULL", eventsTable)
	args := []interface{}{userID}
	argIdx := 2

	if filter.DateFrom != nil {
		query += fmt.Sprintf(" AND end_time >= $%d", argIdx)
		args = append(args, *filter.DateFrom)
		argIdx++
	}
	if filter.DateTo != nil {
		query += fmt.Sprintf(" AND start_time <= $%d", argIdx)
		args = append(args, *filter.DateTo)
		argIdx++
	}
	if filter.Type != "" {
		query += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, filter.Type)
	}

	var count int
	if err := r.db.Get(&count, query, args...); err != nil {
		return 0, err
	}
	return count, nil
}

// GetBusyEventsForUsers returns, for each requested user, the union of their
// personal events (group_id IS NULL) and the events of every group they belong
// to, restricted to the [dateFrom, dateTo] window. The result is keyed by the
// member user id.
func (r *EventPostgres) GetBusyEventsForUsers(userIDs []int, dateFrom, dateTo time.Time) (map[int][]model.Event, error) {
	result := make(map[int][]model.Event)
	if len(userIDs) == 0 {
		return result, nil
	}

	type busyRow struct {
		MemberID int `db:"member_id"`
		model.Event
	}

	personalQuery, personalArgs, err := sqlx.In(fmt.Sprintf(`
		SELECT user_id AS member_id, id, user_id, group_id, title, start_time, end_time, type
		FROM %s
		WHERE group_id IS NULL
		  AND user_id IN (?)
		  AND end_time >= ?
		  AND start_time <= ?`, eventsTable), userIDs, dateFrom, dateTo)
	if err != nil {
		return nil, err
	}
	personalQuery = r.db.Rebind(personalQuery)

	var personalRows []busyRow
	if err := r.db.Select(&personalRows, personalQuery, personalArgs...); err != nil {
		return nil, err
	}
	for _, row := range personalRows {
		result[row.MemberID] = append(result[row.MemberID], row.Event)
	}

	groupQuery, groupArgs, err := sqlx.In(fmt.Sprintf(`
		SELECT gm.user_id AS member_id, e.id, e.user_id, e.group_id, e.title, e.start_time, e.end_time, e.type
		FROM %s e
		JOIN group_members gm ON e.group_id = gm.group_id
		WHERE gm.user_id IN (?)
		  AND e.end_time >= ?
		  AND e.start_time <= ?`, eventsTable), userIDs, dateFrom, dateTo)
	if err != nil {
		return nil, err
	}
	groupQuery = r.db.Rebind(groupQuery)

	var groupRows []busyRow
	if err := r.db.Select(&groupRows, groupQuery, groupArgs...); err != nil {
		return nil, err
	}
	for _, row := range groupRows {
		result[row.MemberID] = append(result[row.MemberID], row.Event)
	}

	return result, nil
}

func (r *EventPostgres) CreateGroupEvent(event model.Event) (int, error) {
	if event.GroupID == nil {
		return 0, errors.New("group id is required")
	}
	tx, err := r.db.Beginx()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockScope(tx, fmt.Sprintf("group:%d", *event.GroupID)); err != nil {
		return 0, err
	}
	overlap, err := overlapping(tx, false, *event.GroupID, event.StartTime, event.EndTime, 0)
	if err != nil {
		return 0, err
	}
	if overlap {
		return 0, ErrOverlap
	}

	var id int
	query := fmt.Sprintf("INSERT INTO %s (user_id, group_id, start_time, end_time, title, type) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id", eventsTable)
	if err := tx.QueryRow(query, event.UserID, event.GroupID, event.StartTime, event.EndTime, event.Title, event.Type).Scan(&id); err != nil {
		return 0, MapPGError(err)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *EventPostgres) UpdateGroupEvent(groupID, eventID int, event model.Event) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockScope(tx, fmt.Sprintf("group:%d", groupID)); err != nil {
		return err
	}
	overlap, err := overlapping(tx, false, groupID, event.StartTime, event.EndTime, eventID)
	if err != nil {
		return err
	}
	if overlap {
		return ErrOverlap
	}

	query := fmt.Sprintf(`
		UPDATE %s SET start_time = $1, end_time = $2, title = $3, type = $4
		WHERE id = $5 AND group_id = $6`, eventsTable)
	result, err := tx.Exec(query, event.StartTime, event.EndTime, event.Title, event.Type, eventID, groupID)
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
	return tx.Commit()
}

func (r *EventPostgres) DeleteGroupEvent(groupID, eventID int) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1 AND group_id = $2", eventsTable)
	result, err := r.db.Exec(query, eventID, groupID)
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

func (r *EventPostgres) GetGroupEventByID(groupID, eventID int) (model.Event, error) {
	var event model.Event
	query := fmt.Sprintf("SELECT id, user_id, group_id, title, start_time, end_time, type FROM %s WHERE id = $1 AND group_id = $2", eventsTable)
	if err := r.db.Get(&event, query, eventID, groupID); err != nil {
		return event, MapPGError(err)
	}
	return event, nil
}

func (r *EventPostgres) GetGroupEvents(groupID int, filter model.EventFilter) ([]model.Event, error) {
	query := fmt.Sprintf("SELECT id, user_id, group_id, title, start_time, end_time, type FROM %s WHERE group_id = $1", eventsTable)
	args := []interface{}{groupID}
	argIdx := 2

	if filter.DateFrom != nil {
		query += fmt.Sprintf(" AND end_time >= $%d", argIdx)
		args = append(args, *filter.DateFrom)
		argIdx++
	}
	if filter.DateTo != nil {
		query += fmt.Sprintf(" AND start_time <= $%d", argIdx)
		args = append(args, *filter.DateTo)
		argIdx++
	}
	if filter.Type != "" {
		query += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, filter.Type)
		argIdx++
	}

	query += " ORDER BY start_time ASC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filter.Offset)
	}

	events := []model.Event{}
	if err := r.db.Select(&events, query, args...); err != nil {
		return nil, err
	}
	return events, nil
}

func (r *EventPostgres) CountGroupEvents(groupID int, filter model.EventFilter) (int, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE group_id = $1", eventsTable)
	args := []interface{}{groupID}
	argIdx := 2

	if filter.DateFrom != nil {
		query += fmt.Sprintf(" AND end_time >= $%d", argIdx)
		args = append(args, *filter.DateFrom)
		argIdx++
	}
	if filter.DateTo != nil {
		query += fmt.Sprintf(" AND start_time <= $%d", argIdx)
		args = append(args, *filter.DateTo)
		argIdx++
	}
	if filter.Type != "" {
		query += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, filter.Type)
	}

	var count int
	if err := r.db.Get(&count, query, args...); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *EventPostgres) HasOverlappingGroupEvent(groupID int, start, end time.Time, excludeID int) (bool, error) {
	query := fmt.Sprintf(`
		SELECT 1 FROM %s
		WHERE group_id = $1
		  AND id <> $2
		  AND start_time < $4
		  AND end_time > $3
		LIMIT 1`, eventsTable)
	var exists int
	err := r.db.Get(&exists, query, groupID, excludeID, start, end)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func lockScope(tx *sqlx.Tx, scope string) error {
	_, err := tx.Exec(`SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, scope)
	return err
}

func overlapping(tx *sqlx.Tx, personal bool, ownerID int, start, end time.Time, excludeID int) (bool, error) {
	var query string
	if personal {
		query = fmt.Sprintf(`
			SELECT 1 FROM %s
			WHERE user_id = $1
			  AND group_id IS NULL
			  AND id <> $2
			  AND start_time < $4
			  AND end_time > $3
			LIMIT 1`, eventsTable)
	} else {
		query = fmt.Sprintf(`
			SELECT 1 FROM %s
			WHERE group_id = $1
			  AND id <> $2
			  AND start_time < $4
			  AND end_time > $3
			LIMIT 1`, eventsTable)
	}
	var exists int
	err := tx.Get(&exists, query, ownerID, excludeID, start, end)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
