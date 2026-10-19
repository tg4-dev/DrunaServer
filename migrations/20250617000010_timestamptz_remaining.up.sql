-- 000008 converted event/group times; convert the remaining timestamp
-- columns too. Existing values are interpreted as UTC.
ALTER TABLE friends
    ALTER COLUMN request_at TYPE TIMESTAMPTZ USING request_at AT TIME ZONE 'UTC',
    ALTER COLUMN confirmed_at TYPE TIMESTAMPTZ USING confirmed_at AT TIME ZONE 'UTC';

ALTER TABLE revoked_tokens
    ALTER COLUMN revoked_at TYPE TIMESTAMPTZ USING revoked_at AT TIME ZONE 'UTC',
    ALTER COLUMN expires_at TYPE TIMESTAMPTZ USING expires_at AT TIME ZONE 'UTC';

ALTER TABLE revoked_token_families
    ALTER COLUMN revoked_at TYPE TIMESTAMPTZ USING revoked_at AT TIME ZONE 'UTC',
    ALTER COLUMN expires_at TYPE TIMESTAMPTZ USING expires_at AT TIME ZONE 'UTC';

ALTER TABLE notification_outbox
    ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
    ALTER COLUMN delivered_at TYPE TIMESTAMPTZ USING delivered_at AT TIME ZONE 'UTC';
