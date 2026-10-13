ALTER TABLE revoked_tokens ADD COLUMN family_id VARCHAR(255);

CREATE TABLE revoked_token_families (
    family_id VARCHAR(255) PRIMARY KEY,
    revoked_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_revoked_token_families_expires_at ON revoked_token_families(expires_at);
