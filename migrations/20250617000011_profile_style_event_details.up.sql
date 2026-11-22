-- Avatar style chosen in the client (palette index and letter font),
-- visible to friends. NULL means "not chosen": the client falls back to
-- its defaults.
ALTER TABLE users
    ADD COLUMN avatar_color SMALLINT,
    ADD COLUMN avatar_font VARCHAR(32),
    ADD CONSTRAINT users_avatar_color_range CHECK (avatar_color BETWEEN 0 AND 7);

-- Case-insensitive lookups for username availability checks.
CREATE INDEX IF NOT EXISTS idx_users_username_lower ON users (LOWER(username));

ALTER TABLE events
    ADD COLUMN description TEXT NOT NULL DEFAULT '',
    ADD COLUMN all_day BOOLEAN NOT NULL DEFAULT FALSE,
    ADD CONSTRAINT events_description_length CHECK (char_length(description) <= 2000);
