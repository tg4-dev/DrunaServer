ALTER TABLE events
    ALTER COLUMN start_time TYPE TIMESTAMP USING start_time AT TIME ZONE 'UTC',
    ALTER COLUMN end_time TYPE TIMESTAMP USING end_time AT TIME ZONE 'UTC';

ALTER TABLE groups
    ALTER COLUMN confirmed_time TYPE TIMESTAMP USING confirmed_time AT TIME ZONE 'UTC';

ALTER TABLE group_members
    ALTER COLUMN confirmed_time TYPE TIMESTAMP USING confirmed_time AT TIME ZONE 'UTC';
