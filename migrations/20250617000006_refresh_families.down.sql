DROP TABLE IF EXISTS revoked_token_families;

ALTER TABLE revoked_tokens DROP COLUMN IF EXISTS family_id;
