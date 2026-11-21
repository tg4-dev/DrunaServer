package repository

import (
	"druna_server/pkg/model"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

const (
	invitesTable            = "invites"
	inviteParticipantsTable = "invite_participants"
)

type InvitePostgres struct {
	db *sqlx.DB
}

func NewInvitePostgres(db *sqlx.DB) *InvitePostgres {
	return &InvitePostgres{db: db}
}

const inviteColumns = "i.id, i.organizer_id, i.title, i.description, i.start_time, i.end_time, i.all_day, i.created_at, i.updated_at"

func (r *InvitePostgres) CreateInvite(invite model.Invite, inviteeIDs []int) (int, error) {
	tx, err := r.db.Beginx()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var id int
	query := fmt.Sprintf(`
		INSERT INTO %s (organizer_id, title, description, start_time, end_time, all_day)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`, invitesTable)
	if err := tx.QueryRow(query, invite.OrganizerID, invite.Title, invite.Description,
		invite.StartTime, invite.EndTime, invite.AllDay).Scan(&id); err != nil {
		return 0, MapPGError(err)
	}

	participantQuery := fmt.Sprintf("INSERT INTO %s (invite_id, user_id) VALUES ($1, $2)", inviteParticipantsTable)
	for _, userID := range inviteeIDs {
		if _, err := tx.Exec(participantQuery, id, userID); err != nil {
			return 0, MapPGError(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// GetInvite returns the invite if userID organizes it or is invited to it,
// otherwise ErrNotFound.
func (r *InvitePostgres) GetInvite(inviteID, userID int) (model.Invite, error) {
	var invite model.Invite
	query := fmt.Sprintf(`
		SELECT %s FROM %s i
		WHERE i.id = $1
		  AND (i.organizer_id = $2 OR EXISTS (
			SELECT 1 FROM %s p WHERE p.invite_id = i.id AND p.user_id = $2))`,
		inviteColumns, invitesTable, inviteParticipantsTable)
	if err := r.db.Get(&invite, query, inviteID, userID); err != nil {
		return invite, MapPGError(err)
	}
	invites := []model.Invite{invite}
	if err := r.loadPeople(invites); err != nil {
		return invite, err
	}
	return invites[0], nil
}

func inviteListWhere(filter model.InviteFilter) (string, []interface{}, error) {
	where := "(i.organizer_id = $1 OR p.user_id IS NOT NULL)"
	var args []interface{}
	switch filter.Status {
	case "":
	case model.InviteFilterPending:
		where += " AND p.status = 'pending' AND i.end_time > $2"
		args = append(args, filter.Now)
	case model.InviteFilterUpcoming:
		where += " AND i.end_time > $2 AND (i.organizer_id = $1 OR p.status <> 'pending')"
		args = append(args, filter.Now)
	case model.InviteFilterPast:
		where += " AND i.end_time <= $2"
		args = append(args, filter.Now)
	default:
		return "", nil, fmt.Errorf("unknown invite filter %q", filter.Status)
	}
	return where, args, nil
}

func (r *InvitePostgres) ListInvites(userID int, filter model.InviteFilter) ([]model.Invite, int, error) {
	where, extra, err := inviteListWhere(filter)
	if err != nil {
		return nil, 0, err
	}
	from := fmt.Sprintf(`FROM %s i
		LEFT JOIN %s p ON p.invite_id = i.id AND p.user_id = $1
		WHERE %s`, invitesTable, inviteParticipantsTable, where)
	args := append([]interface{}{userID}, extra...)

	var total int
	if err := r.db.Get(&total, "SELECT COUNT(*) "+from, args...); err != nil {
		return nil, 0, err
	}

	order := "i.start_time ASC, i.id ASC"
	if filter.Status == model.InviteFilterPast {
		order = "i.start_time DESC, i.id DESC"
	}
	listArgs := append(args, filter.Limit, filter.Offset)
	query := fmt.Sprintf("SELECT %s %s ORDER BY %s LIMIT $%d OFFSET $%d",
		inviteColumns, from, order, len(args)+1, len(args)+2)

	invites := []model.Invite{}
	if err := r.db.Select(&invites, query, listArgs...); err != nil {
		return nil, 0, err
	}
	if err := r.loadPeople(invites); err != nil {
		return nil, 0, err
	}
	return invites, total, nil
}

// loadPeople fills Organizer and Participants for the given invites.
func (r *InvitePostgres) loadPeople(invites []model.Invite) error {
	if len(invites) == 0 {
		return nil
	}
	ids := make([]int, 0, len(invites))
	organizerIDs := make([]int, 0, len(invites))
	for _, inv := range invites {
		ids = append(ids, inv.ID)
		organizerIDs = append(organizerIDs, inv.OrganizerID)
	}

	query, args, err := sqlx.In(fmt.Sprintf(`
		SELECT id, name, username, COALESCE(avatar_url, '') AS avatar_url, avatar_color, avatar_font
		FROM %s WHERE id IN (?)`, usersTable), organizerIDs)
	if err != nil {
		return err
	}
	var organizers []model.FriendInfo
	if err := r.db.Select(&organizers, r.db.Rebind(query), args...); err != nil {
		return err
	}
	byID := make(map[int]model.FriendInfo, len(organizers))
	for _, o := range organizers {
		byID[o.ID] = o
	}

	type participantRow struct {
		InviteID int `db:"invite_id"`
		model.FriendInfo
		Status      string     `db:"status"`
		RespondedAt *time.Time `db:"responded_at"`
	}
	query, args, err = sqlx.In(fmt.Sprintf(`
		SELECT p.invite_id, u.id, u.name, u.username, COALESCE(u.avatar_url, '') AS avatar_url,
			u.avatar_color, u.avatar_font, p.status, p.responded_at
		FROM %s p
		JOIN %s u ON u.id = p.user_id
		WHERE p.invite_id IN (?)
		ORDER BY u.username`, inviteParticipantsTable, usersTable), ids)
	if err != nil {
		return err
	}
	var rows []participantRow
	if err := r.db.Select(&rows, r.db.Rebind(query), args...); err != nil {
		return err
	}
	participants := make(map[int][]model.InviteParticipant)
	for _, row := range rows {
		participants[row.InviteID] = append(participants[row.InviteID], model.InviteParticipant{
			User:        row.FriendInfo,
			Status:      row.Status,
			RespondedAt: row.RespondedAt,
		})
	}

	for i := range invites {
		invites[i].Organizer = byID[invites[i].OrganizerID]
		invites[i].Participants = participants[invites[i].ID]
		if invites[i].Participants == nil {
			invites[i].Participants = []model.InviteParticipant{}
		}
	}
	return nil
}

// UpdateInvite stores the editable fields. With resetResponses, every
// answer goes back to pending (used when the time changes).
func (r *InvitePostgres) UpdateInvite(invite model.Invite, resetResponses bool) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	query := fmt.Sprintf(`
		UPDATE %s SET title = $1, description = $2, start_time = $3, end_time = $4, all_day = $5, updated_at = NOW()
		WHERE id = $6`, invitesTable)
	result, err := tx.Exec(query, invite.Title, invite.Description, invite.StartTime, invite.EndTime, invite.AllDay, invite.ID)
	if err != nil {
		return MapPGError(err)
	}
	if rows, err := result.RowsAffected(); err != nil {
		return err
	} else if rows == 0 {
		return ErrNotFound
	}

	if resetResponses {
		reset := fmt.Sprintf("UPDATE %s SET status = 'pending', responded_at = NULL WHERE invite_id = $1", inviteParticipantsTable)
		if _, err := tx.Exec(reset, invite.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *InvitePostgres) DeleteInvite(inviteID int) error {
	result, err := r.db.Exec(fmt.Sprintf("DELETE FROM %s WHERE id = $1", invitesTable), inviteID)
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

// RespondInvite sets the participant's answer; ErrNotFound if userID is not invited.
func (r *InvitePostgres) RespondInvite(inviteID, userID int, status string) error {
	query := fmt.Sprintf(`
		UPDATE %s SET status = $3::varchar, responded_at = CASE WHEN $3::varchar = 'pending' THEN NULL ELSE NOW() END
		WHERE invite_id = $1 AND user_id = $2`, inviteParticipantsTable)
	result, err := r.db.Exec(query, inviteID, userID, status)
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
