-- Attempt runtime: device-bound sessions, frozen papers and server-side clocks.

-- Attempts freeze their timing inputs at start so every deadline can be
-- recomputed exactly from stored values.
ALTER TABLE exam_attempts
    ADD COLUMN assignment_id UUID
        REFERENCES exam_assignments(id)
        ON DELETE SET NULL,
    ADD COLUMN device_id VARCHAR(128),
    ADD COLUMN seed BIGINT NOT NULL DEFAULT 0,
    -- The exam clock runs from timer_origin_at (the window start for fixed
    -- windows, started_at otherwise) for limit_seconds of unpaused time.
    ADD COLUMN timer_origin_at TIMESTAMPTZ,
    ADD COLUMN limit_seconds INTEGER,
    ADD COLUMN window_end_at TIMESTAMPTZ,
    ADD COLUMN time_multiplier NUMERIC(3, 2) NOT NULL DEFAULT 1.00,
    ADD COLUMN current_section_id UUID
        REFERENCES exam_sections(id)
        ON DELETE SET NULL,
    ADD COLUMN paused_at TIMESTAMPTZ,
    ADD COLUMN pause_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN paused_seconds INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN last_heartbeat_at TIMESTAMPTZ,
    -- deadline_at is the latest moment the attempt can still be running. It
    -- is maintained for the expiry sweeper; clocks never read it.
    ADD COLUMN deadline_at TIMESTAMPTZ,
    ADD COLUMN ended_reason VARCHAR(32),
    ADD COLUMN max_score NUMERIC(8, 2),

    ADD CONSTRAINT exam_attempts_pause_count_non_negative
        CHECK (pause_count >= 0),
    ADD CONSTRAINT exam_attempts_paused_seconds_non_negative
        CHECK (paused_seconds >= 0);

-- One running attempt per candidate and exam.
CREATE UNIQUE INDEX exam_attempts_one_in_progress
    ON exam_attempts(exam_id, user_id)
    WHERE status = 'in_progress';

CREATE INDEX idx_exam_attempts_running_deadline
    ON exam_attempts(deadline_at)
    WHERE status = 'in_progress';

ALTER TABLE attempt_section_states
    ADD COLUMN position INTEGER NOT NULL DEFAULT 0,
    -- Frozen section time limit; NULL when the section shares the exam clock.
    ADD COLUMN time_limit_seconds INTEGER,
    -- Start of the current visit while the section is open, NULL otherwise.
    ADD COLUMN entered_at TIMESTAMPTZ,
    ADD COLUMN cursor INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN score_awarded NUMERIC(8, 2),
    ADD COLUMN max_score NUMERIC(8, 2),

    ADD CONSTRAINT attempt_section_states_unique_position
        UNIQUE (attempt_id, position);

-- The questions served to one attempt, in the order the candidate sees them.
CREATE TABLE attempt_questions (
    attempt_id UUID NOT NULL
        REFERENCES exam_attempts(id)
        ON DELETE CASCADE,

    question_id UUID NOT NULL
        REFERENCES questions(id)
        ON DELETE CASCADE,

    section_id UUID NOT NULL
        REFERENCES exam_sections(id)
        ON DELETE CASCADE,

    position INTEGER NOT NULL,
    option_order UUID[] NOT NULL DEFAULT '{}',
    points NUMERIC(6, 2) NOT NULL,
    first_viewed_at TIMESTAMPTZ,

    PRIMARY KEY (attempt_id, question_id),

    CONSTRAINT attempt_questions_unique_position
        UNIQUE (attempt_id, section_id, position),

    CONSTRAINT attempt_questions_position_non_negative
        CHECK (position >= 0)
);

-- Single-use codes the web portal hands to the desktop app.
CREATE TABLE attempt_launch_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    code_hash VARCHAR(64) NOT NULL UNIQUE,

    user_id UUID NOT NULL
        REFERENCES users(id)
        ON DELETE CASCADE,

    exam_id UUID NOT NULL
        REFERENCES exams(id)
        ON DELETE CASCADE,

    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_attempt_launch_codes_user_exam ON attempt_launch_codes(user_id, exam_id);

-- Desktop app sessions. A session belongs to one device and one exam, and to
-- the attempt once it has started.
CREATE TABLE attempt_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    token_hash VARCHAR(64) NOT NULL UNIQUE,

    user_id UUID NOT NULL
        REFERENCES users(id)
        ON DELETE CASCADE,

    exam_id UUID NOT NULL
        REFERENCES exams(id)
        ON DELETE CASCADE,

    attempt_id UUID
        REFERENCES exam_attempts(id)
        ON DELETE CASCADE,

    device_id VARCHAR(128) NOT NULL,
    client_ip INET,
    user_agent TEXT,

    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_attempt_sessions_user_exam ON attempt_sessions(user_id, exam_id);
CREATE INDEX idx_attempt_sessions_attempt ON attempt_sessions(attempt_id);

-- Violations below a grace threshold are logged but do not count towards limits.
ALTER TABLE proctoring_violation_logs
    ADD COLUMN counted BOOLEAN NOT NULL DEFAULT TRUE;

CREATE INDEX idx_proctoring_violation_attempt_type ON proctoring_violation_logs(attempt_id, violation_type) WHERE counted;
