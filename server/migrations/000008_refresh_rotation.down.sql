DROP INDEX IF EXISTS idx_user_sessions_previous_refresh_token;

ALTER TABLE user_sessions
    DROP COLUMN rotated_at,
    DROP COLUMN previous_refresh_token_hash;
