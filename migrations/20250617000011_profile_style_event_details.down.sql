ALTER TABLE events
    DROP CONSTRAINT IF EXISTS events_description_length,
    DROP COLUMN IF EXISTS all_day,
    DROP COLUMN IF EXISTS description;

DROP INDEX IF EXISTS idx_users_username_lower;

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_avatar_color_range,
    DROP COLUMN IF EXISTS avatar_font,
    DROP COLUMN IF EXISTS avatar_color;
