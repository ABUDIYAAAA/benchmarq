-- Sessions remember the refresh token they rotated away from, so a replayed
-- old token can be told apart from an unknown one.
ALTER TABLE user_sessions
    ADD COLUMN previous_refresh_token_hash VARCHAR(255),
    ADD COLUMN rotated_at TIMESTAMPTZ;

CREATE INDEX idx_user_sessions_previous_refresh_token
    ON user_sessions(previous_refresh_token_hash)
    WHERE previous_refresh_token_hash IS NOT NULL;
