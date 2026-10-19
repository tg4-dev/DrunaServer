ALTER TABLE friends
    ALTER COLUMN request_at TYPE TIMESTAMP USING request_at AT TIME ZONE 'UTC',
    ALTER COLUMN confirmed_at TYPE TIMESTAMP USING confirmed_at AT TIME ZONE 'UTC';

ALTER TABLE revoked_tokens
    ALTER COLUMN revoked_at TYPE TIMESTAMP USING revoked_at AT TIME ZONE 'UTC',
    ALTER COLUMN expires_at TYPE TIMESTAMP USING expires_at AT TIME ZONE 'UTC';

ALTER TABLE revoked_token_families
    ALTER COLUMN revoked_at TYPE TIMESTAMP USING revoked_at AT TIME ZONE 'UTC',
    ALTER COLUMN expires_at TYPE TIMESTAMP USING expires_at AT TIME ZONE 'UTC';

ALTER TABLE notification_outbox
    ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC',
    ALTER COLUMN delivered_at TYPE TIMESTAMP USING delivered_at AT TIME ZONE 'UTC';
