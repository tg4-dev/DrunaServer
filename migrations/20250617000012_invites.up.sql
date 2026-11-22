-- Event invitations: an organizer invites accepted friends, each invitee
-- answers going / not_going (or withdraws back to pending). The organizer
-- is not a participant row and is always considered going.
CREATE TABLE invites (
    id SERIAL PRIMARY KEY,
    organizer_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    all_day BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT invites_time_order CHECK (end_time > start_time),
    CONSTRAINT invites_description_length CHECK (char_length(description) <= 2000)
);

CREATE INDEX idx_invites_organizer_time ON invites (organizer_id, start_time);

CREATE TABLE invite_participants (
    invite_id INT NOT NULL REFERENCES invites(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'going', 'not_going')),
    responded_at TIMESTAMPTZ,
    PRIMARY KEY (invite_id, user_id)
);

CREATE INDEX idx_invite_participants_user ON invite_participants (user_id, status);
